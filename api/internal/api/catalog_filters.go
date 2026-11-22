package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/metadata"
	"toto/api/internal/store"
)

type catalogFilters struct {
	Genres        []int32 `json:"genres,omitempty"`
	ExcludeGenres []int32 `json:"excludeGenres,omitempty"`
	// LegacyGenreMode is accepted and dropped: web fronts and peers up to 0.3, or links they shared, may still send it.
	LegacyGenreMode json.RawMessage `json:"genreMode,omitempty"`
	Actors          []int32         `json:"actors,omitempty"`
	Creators        []int32         `json:"creators,omitempty"`
	Keywords        []int32         `json:"keywords,omitempty"`
	Languages       []string        `json:"languages,omitempty"`
	Countries       []string        `json:"countries,omitempty"`
	YearMin         *int            `json:"yearMin,omitempty"`
	YearMax         *int            `json:"yearMax,omitempty"`
	RatingMin       *float64        `json:"ratingMin,omitempty"`
	VotesMin        *int            `json:"votesMin,omitempty"`
	RuntimeMin      *int            `json:"runtimeMin,omitempty"`
	RuntimeMax      *int            `json:"runtimeMax,omitempty"`
	Resolution      string          `json:"resolution,omitempty"`
	Audio           string          `json:"audio,omitempty"`
}

// Values of the technical filters, as computed by media_resolution and media_audio_formats (migration 00017).
var (
	filterResolutions  = []string{"2160p", "1080p", "720p", "SD"}
	filterAudioFormats = []string{"atmos", "truehd", "eac3", "ac3", "dtsHdMa", "dts", "flac", "pcm", "aac", "opus", "mp3"}
)

type catalogOptionsResponse struct {
	Options            map[string][]metadata.FilterOption `json:"options"`
	UnavailableServers int                                `json:"unavailableServers"`
	// UnavailableServerNames names the peers left out, so that clients can say which servers are missing.
	UnavailableServerNames []string `json:"unavailableServerNames"`
}

var filterCategories = []string{"genres", "actors", "creators", "keywords", "languages", "countries"}

func (s *Server) listCatalogFilterOptions(w http.ResponseWriter, r *http.Request) {
	s.catalogFilterOptions(w, r, false)
}

func (s *Server) peerCatalogFilterOptions(w http.ResponseWriter, r *http.Request) {
	s.catalogFilterOptions(w, r, true)
}

func (s *Server) catalogFilterOptions(w http.ResponseWriter, r *http.Request, federated bool) {
	kind := chi.URLParam(r, "kind")
	category, query := r.URL.Query().Get("category"), strings.TrimSpace(r.URL.Query().Get("query"))
	locale, localeOK := "fr", true
	if raw := r.URL.Query().Get("locale"); raw != "" {
		locale, localeOK = canonicalLocale(raw)
	}
	selected := []string{}
	if raw := r.URL.Query().Get("selected"); raw != "" {
		selected = strings.Split(raw, ",")
	}
	if !slices.Contains([]string{"movies", "series"}, kind) || category != "" && !slices.Contains(filterCategories, category) || len(query) > 200 || len(selected) > 20 || len(r.URL.Query().Get("selected")) > 500 || !localeOK {
		httpx.Error(w, http.StatusBadRequest, "validation", "invalid filter suggestions parameters")
		return
	}
	principal, _ := auth.PrincipalFrom(r.Context())
	userID, isAdmin := principal.UserID, principal.Role == auth.RoleAdmin
	if federated {
		var ok bool
		userID, ok = s.federatedUser(w, r)
		if !ok {
			return
		}
		isAdmin = false
	}
	var libraryID *uuid.UUID
	if raw := r.URL.Query().Get("library"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil || federated {
			httpx.Error(w, http.StatusBadRequest, "validation", "invalid library")
			return
		}
		allowed, err := s.canAccessLibrary(r, parsed)
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		if !allowed {
			httpx.Error(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		libraryID = &parsed
	}
	pattern := ""
	if query != "" {
		pattern = "%" + likeEscaper.Replace(query) + "%"
	}
	rows, err := s.q.ListCatalogFilterOptions(r.Context(), store.ListCatalogFilterOptionsParams{
		Kind: kind, LibraryID: libraryID, UserID: userID, IsAdmin: isAdmin, Category: category, Query: pattern, Selected: selected,
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	response := catalogOptionsResponse{Options: make(map[string][]metadata.FilterOption), UnavailableServerNames: []string{}}
	for _, row := range rows {
		label := localizedFilterLabel(row.Category, row.ID, row.Label, locale)
		if (row.Category == "languages" || row.Category == "countries") && !strings.Contains(foldSearch(label), foldSearch(query)) && !slices.Contains(selected, row.ID) {
			continue
		}
		response.Options[row.Category] = append(response.Options[row.Category], metadata.FilterOption{ID: row.ID, Label: label})
	}
	if !federated && libraryID == nil {
		results, unavailable, err := fanOutPeers(r.Context(), s, "filter options", func(ctx context.Context, peer store.Peer) (catalogOptionsResponse, error) {
			key := strings.Join([]string{userID.String(), kind, category, query, strings.Join(selected, ","), locale}, "\x00")
			return cachedPeerCall(ctx, s, s.peerFilters, peer, key, func(ctx context.Context) (catalogOptionsResponse, error) {
				return s.peerFilterOptions(ctx, peer, userID, kind, category, query, selected, locale)
			})
		})
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		response.UnavailableServers, response.UnavailableServerNames = len(unavailable), unavailable
		for _, remote := range results {
			for key, options := range remote.Options {
				response.Options[key] = append(response.Options[key], options...)
			}
		}
	}
	for key, options := range response.Options {
		seen := make(map[string]bool)
		unique := make([]metadata.FilterOption, 0, len(options))
		for _, option := range options {
			if !seen[option.ID] {
				seen[option.ID] = true
				unique = append(unique, option)
			}
		}
		slices.SortFunc(unique, func(left, right metadata.FilterOption) int {
			leftSelected, rightSelected := slices.Contains(selected, left.ID), slices.Contains(selected, right.ID)
			if leftSelected != rightSelected {
				if leftSelected {
					return -1
				}
				return 1
			}
			return strings.Compare(strings.ToLower(left.Label), strings.ToLower(right.Label))
		})
		limit := 50
		if key == "languages" || key == "countries" {
			limit = 250
		}
		response.Options[key] = unique[:min(len(unique), limit)]
	}
	httpx.JSON(w, http.StatusOK, response)
}

// historicalRegions names the withdrawn country codes TMDB still uses, which x/text leaves unnamed.
var historicalRegions = map[string]map[string]string{
	"AN": {"en": "Netherlands Antilles", "fr": "Antilles néerlandaises", "pt-BR": "Antilhas Holandesas"},
	"CS": {"en": "Serbia and Montenegro", "fr": "Serbie-et-Monténégro", "pt-BR": "Sérvia e Montenegro"},
	"SU": {"en": "Soviet Union", "fr": "Union soviétique", "pt-BR": "União Soviética"},
	"XC": {"en": "Czechoslovakia", "fr": "Tchécoslovaquie", "pt-BR": "Tchecoslováquia"},
	"XG": {"en": "East Germany", "fr": "Allemagne de l'Est", "pt-BR": "Alemanha Oriental"},
	"YU": {"en": "Yugoslavia", "fr": "Yougoslavie", "pt-BR": "Iugoslávia"},
}

// localizedFilterLabel names languages and countries in the locale. A code without a name keeps its stored label,
// or the code itself: an empty label would be listed first, as a blank entry.
func localizedFilterLabel(category, id, stored, locale string) string {
	label := stored
	switch category {
	case "languages":
		if tag, err := language.Parse(id); err == nil {
			label = display.Languages(language.Make(locale)).Name(tag)
		}
	case "countries":
		if names, ok := historicalRegions[strings.ToUpper(id)]; ok {
			label = names[locale]
		} else if region, err := language.ParseRegion(id); err == nil {
			label = display.Regions(language.Make(locale)).Name(region)
		}
	}
	if strings.TrimSpace(label) == "" {
		label = stored
	}
	if strings.TrimSpace(label) == "" {
		label = id
	}
	return label
}

// peerFilterOptions reads the filter suggestions of one peer for the user; callers go through callPeer.
func (s *Server) peerFilterOptions(ctx context.Context, peer store.Peer, userID uuid.UUID, kind, category, query string, selected []string, locale string) (catalogOptionsResponse, error) {
	var response catalogOptionsResponse
	token, err := s.federatedTokenFor(peer.ID, userID)
	if err != nil {
		return response, err
	}
	fetch := func(locale string) error {
		target := peer.Url + "/api/v1/federation/catalog/kinds/" + kind + "/filters?" + url.Values{"category": {category}, "query": {query}, "selected": {strings.Join(selected, ",")}, "locale": {locale}}.Encode()
		response = catalogOptionsResponse{}
		return s.getPeerCatalog(ctx, target, token, &response)
	}
	err = fetch(locale)
	if err != nil && !errors.Is(err, errPeerDown) && !slices.Contains(legacyFilterLocales, locale) {
		// Peers released before pt-BR reject it; English labels beat no suggestions, and clients relabel ISO names anyway.
		err = fetch("en")
	}
	return response, err
}

// legacyFilterLocales are the filter locales every compatible peer accepts.
var legacyFilterLocales = []string{"fr", "en"}

func parseCatalogFilters(raw string) (json.RawMessage, error) {
	if raw == "" {
		return json.RawMessage(`{}`), nil
	}
	if len(raw) > 8192 || !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return nil, fmt.Errorf("filters must be a JSON object of at most 8192 bytes")
	}
	var filters catalogFilters
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&filters); err != nil {
		return nil, fmt.Errorf("invalid filters: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("filters must contain one object")
	}
	filters.LegacyGenreMode = nil
	if filters.Resolution != "" && !slices.Contains(filterResolutions, filters.Resolution) {
		return nil, fmt.Errorf("resolution must be one of %s", strings.Join(filterResolutions, ", "))
	}
	if filters.Audio != "" && !slices.Contains(filterAudioFormats, filters.Audio) {
		return nil, fmt.Errorf("audio must be one of %s", strings.Join(filterAudioFormats, ", "))
	}
	for _, ids := range [][]int32{filters.Genres, filters.ExcludeGenres, filters.Actors, filters.Creators, filters.Keywords} {
		if len(ids) > 20 {
			return nil, fmt.Errorf("at most 20 selections per filter")
		}
		for _, id := range ids {
			if id <= 0 {
				return nil, fmt.Errorf("TMDB IDs must be positive integers")
			}
		}
	}
	for _, id := range filters.Genres {
		if slices.Contains(filters.ExcludeGenres, id) {
			return nil, fmt.Errorf("a genre cannot be included and excluded")
		}
	}
	for _, codes := range [][]string{filters.Languages, filters.Countries} {
		if len(codes) > 20 {
			return nil, fmt.Errorf("at most 20 selections per filter")
		}
		for _, code := range codes {
			if len(code) != 2 || !((code[0] >= 'a' && code[0] <= 'z' && code[1] >= 'a' && code[1] <= 'z') || (code[0] >= 'A' && code[0] <= 'Z' && code[1] >= 'A' && code[1] <= 'Z')) {
				return nil, fmt.Errorf("language and country codes must be ISO two-letter codes")
			}
		}
	}
	for index := range filters.Languages {
		filters.Languages[index] = strings.ToLower(filters.Languages[index])
	}
	for index := range filters.Countries {
		filters.Countries[index] = strings.ToUpper(filters.Countries[index])
	}
	for _, bound := range []struct {
		value    *int
		min, max int
	}{
		{filters.YearMin, 1800, 9999}, {filters.YearMax, 1800, 9999},
		{filters.VotesMin, 0, 2147483647}, {filters.RuntimeMin, 0, 10000}, {filters.RuntimeMax, 0, 10000},
	} {
		if bound.value != nil && (*bound.value < bound.min || *bound.value > bound.max) {
			return nil, fmt.Errorf("filter value must be between %d and %d", bound.min, bound.max)
		}
	}
	if filters.YearMin != nil && filters.YearMax != nil && *filters.YearMin > *filters.YearMax {
		return nil, fmt.Errorf("yearMin must not exceed yearMax")
	}
	if filters.RuntimeMin != nil && filters.RuntimeMax != nil && *filters.RuntimeMin > *filters.RuntimeMax {
		return nil, fmt.Errorf("runtimeMin must not exceed runtimeMax")
	}
	if filters.RatingMin != nil && (*filters.RatingMin < 0 || *filters.RatingMin > 10) {
		return nil, fmt.Errorf("ratingMin must be between 0 and 10")
	}
	return json.Marshal(filters)
}
