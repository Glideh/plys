package api

import (
	"bytes"
	"context"
	"encoding/json"
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
	Genres        []int32  `json:"genres,omitempty"`
	ExcludeGenres []int32  `json:"excludeGenres,omitempty"`
	GenreMode     string   `json:"genreMode,omitempty"`
	Actors        []int32  `json:"actors,omitempty"`
	Creators      []int32  `json:"creators,omitempty"`
	Keywords      []int32  `json:"keywords,omitempty"`
	Languages     []string `json:"languages,omitempty"`
	Countries     []string `json:"countries,omitempty"`
	YearMin       *int     `json:"yearMin,omitempty"`
	YearMax       *int     `json:"yearMax,omitempty"`
	RatingMin     *float64 `json:"ratingMin,omitempty"`
	VotesMin      *int     `json:"votesMin,omitempty"`
	RuntimeMin    *int     `json:"runtimeMin,omitempty"`
	RuntimeMax    *int     `json:"runtimeMax,omitempty"`
}

type catalogOptionsResponse struct {
	Options            map[string][]metadata.FilterOption `json:"options"`
	UnavailableServers int                                `json:"unavailableServers"`
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
	response := catalogOptionsResponse{Options: make(map[string][]metadata.FilterOption)}
	for _, row := range rows {
		label := row.Label
		if row.Category == "languages" {
			label = display.Languages(language.Make(locale)).Name(language.Make(row.ID))
		}
		if row.Category == "countries" {
			if region, err := language.ParseRegion(row.ID); err == nil {
				label = display.Regions(language.Make(locale)).Name(region)
			}
		}
		if (row.Category == "languages" || row.Category == "countries") && !strings.Contains(foldSearch(label), foldSearch(query)) && !slices.Contains(selected, row.ID) {
			continue
		}
		response.Options[row.Category] = append(response.Options[row.Category], metadata.FilterOption{ID: row.ID, Label: label})
	}
	if !federated && libraryID == nil {
		peers, err := s.q.ListPeers(r.Context())
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		for _, peer := range peers {
			remote, err := s.peerFilterOptions(r.Context(), peer, userID, kind, category, query, selected, locale)
			if err != nil {
				response.UnavailableServers++
				continue
			}
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
		response.Options[key] = unique[:min(len(unique), 50)]
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (s *Server) peerFilterOptions(ctx context.Context, peer store.Peer, userID uuid.UUID, kind, category, query string, selected []string, locale string) (catalogOptionsResponse, error) {
	if _, err := s.checkPeerVersion(ctx, peer); err != nil {
		return catalogOptionsResponse{}, err
	}
	token, err := s.federatedTokenFor(peer.ID, userID)
	if err != nil {
		return catalogOptionsResponse{}, err
	}
	fetch := func(locale string) (catalogOptionsResponse, error) {
		target := peer.Url + "/api/v1/federation/catalog/kinds/" + kind + "/filters?" + url.Values{"category": {category}, "query": {query}, "selected": {strings.Join(selected, ",")}, "locale": {locale}}.Encode()
		var response catalogOptionsResponse
		err := s.getPeerCatalog(ctx, target, token, &response)
		return response, err
	}
	response, err := fetch(locale)
	if err != nil && !slices.Contains(legacyFilterLocales, locale) {
		// Peers released before pt-BR reject it; English labels beat no suggestions, and clients relabel ISO names anyway.
		return fetch("en")
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
	if filters.GenreMode != "" && filters.GenreMode != "any" && filters.GenreMode != "all" {
		return nil, fmt.Errorf("genreMode must be any or all")
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
