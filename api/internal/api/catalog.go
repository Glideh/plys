package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/scanner"
	"toto/api/internal/store"
)

const (
	defaultPageSize = 60
	// maxPageSize lets a client reload the pages it already shows in one request.
	maxPageSize = 1000
)

var itemSorts = []string{"title", "year", "added", "rating", "watched"}

func (s *Server) canAccessLibrary(r *http.Request, libraryID uuid.UUID) (bool, error) {
	p, _ := auth.PrincipalFrom(r.Context())
	return s.q.CanAccessLibrary(r.Context(), store.CanAccessLibraryParams{
		LibraryID: libraryID, IsAdmin: p.Role == auth.RoleAdmin, UserID: p.UserID,
	})
}

type itemCardDTO struct {
	ID           uuid.UUID `json:"id"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Year         *int32    `json:"year"`
	PosterURL    *string   `json:"posterUrl"`
	Rating       *float32  `json:"rating"`
	EpisodeCount int32     `json:"episodeCount,omitempty"`
	MatchStatus  string    `json:"matchStatus"`
	// Watched is set for movies and videos, and for series whose episodes are all watched.
	Watched bool `json:"watched"`
	// UnwatchedCount counts the episodes of a series the caller has not watched yet.
	UnwatchedCount int32 `json:"unwatchedCount,omitempty"`
}

func (s *Server) listLibraryItems(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "library")
	if !ok {
		return
	}
	allowed, err := s.canAccessLibrary(r, id)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	// Restricted libraries are reported as missing so their existence is not revealed.
	if !allowed {
		httpx.Error(w, http.StatusNotFound, "not_found", "library not found")
		return
	}

	params, ok := parseItemListParams(w, r, maxPageSize)
	if !ok {
		return
	}
	sort, limit, offset, query := params.sort, params.limit, params.offset, params.query

	p, _ := auth.PrincipalFrom(r.Context())
	listParams := store.ListLibraryItemsParams{
		UserID: p.UserID, LibraryID: id, Query: query, Sort: sort, Off: int32(offset), Lim: int32(limit),
		Filters: params.filters,
	}
	rows, err := s.q.ListLibraryItems(r.Context(), listParams)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	total := int32(0)
	if len(rows) == 0 && offset > 0 {
		listParams.Off, listParams.Lim = 0, 1
		first, err := s.q.ListLibraryItems(r.Context(), listParams)
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		if len(first) > 0 {
			total = first[0].Total
		}
	}
	items := make([]itemCardDTO, 0, len(rows))
	for _, row := range rows {
		total = row.Total
		poster := imageURL(row.PosterPath, "w342")
		if poster == nil {
			poster = imageURL(row.ThumbPath, "w342")
		}
		items = append(items, itemCardDTO{
			ID: row.ID, Kind: row.Kind, Title: row.Title, Year: row.Year, PosterURL: poster,
			Rating: row.Rating, EpisodeCount: row.EpisodeCount, MatchStatus: row.MatchStatus,
			Watched: row.Watched || (row.Kind == "series" && row.UnwatchedCount == 0), UnwatchedCount: row.UnwatchedCount,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "offset": offset, "limit": limit})
}

func (s *Server) listKindItems(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	if !slices.Contains(libraryKinds, kind) {
		httpx.Error(w, http.StatusBadRequest, "validation", "kind must be one of: movies, series, other")
		return
	}
	var libraryID *uuid.UUID
	if raw := r.URL.Query().Get("library"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "validation", "library must be a uuid")
			return
		}
		libraryID = &id
	}
	params, ok := parseItemListParams(w, r, maxPageSize)
	if !ok {
		return
	}

	ctx := r.Context()
	p, _ := auth.PrincipalFrom(ctx)
	isAdmin := p.Role == auth.RoleAdmin
	serverName, err := s.Settings.ServerName(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	publicURL, err := s.Settings.ServerPublicURL(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	local, _, err := s.kindCards(ctx, p.UserID, isAdmin, kind, libraryID, params.query, params.sort, 0, allRows, params.filters)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	cards := make([]sourcedCard, 0, len(local))
	for _, c := range local {
		cards = append(cards, sourcedCard{federatedCard: c, source: cardSourceDTO{
			ServerID: s.Identity.ID, ServerName: serverName, ServerURL: publicURL, ItemID: c.ID,
		}})
	}
	unavailable := []string{}
	// A library filter names a local library, so peers are not asked.
	if libraryID == nil {
		raw := strings.TrimSpace(r.URL.Query().Get("query"))
		peerCards, failed, err := s.peerKindCards(ctx, p.UserID, kind, raw, params.sort, params.filters)
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		cards = append(cards, peerCards...)
		unavailable = failed
	}
	slices.SortStableFunc(cards, func(a, b sourcedCard) int { return compareCards(a, b, params.sort) })
	groups := groupCards(cards)
	total := len(groups)
	start, end := min(params.offset, total), min(params.offset+params.limit, total)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"items": groups[start:end], "total": total, "offset": params.offset, "limit": params.limit,
		"unavailableServers": len(unavailable), "unavailableServerNames": unavailable,
	})
}

type itemListParams struct {
	filters       json.RawMessage
	sort, query   string
	limit, offset int
}

// parseItemListParams reads the sort, pagination and search query parameters shared by the item listings;
// maxLimit is the largest page the listing serves.
func parseItemListParams(w http.ResponseWriter, r *http.Request, maxLimit int) (itemListParams, bool) {
	qs := r.URL.Query()
	var p itemListParams
	p.sort = qs.Get("sort")
	if p.sort == "" {
		p.sort = "title"
	}
	if !slices.Contains(itemSorts, p.sort) {
		httpx.Error(w, http.StatusBadRequest, "validation", "sort must be one of: title, year, added, rating, watched")
		return p, false
	}
	limit, err := intParam(qs.Get("limit"), defaultPageSize, 1, maxLimit)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "limit "+err.Error())
		return p, false
	}
	offset, err := intParam(qs.Get("offset"), 0, 0, 1<<30)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "offset "+err.Error())
		return p, false
	}
	p.limit, p.offset = limit, offset
	p.filters, err = parseCatalogFilters(qs.Get("filters"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return p, false
	}
	if q := strings.TrimSpace(qs.Get("query")); q != "" {
		p.query = "%" + likeEscaper.Replace(q) + "%"
	}
	return p, true
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func intParam(raw string, fallback, minV, maxV int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < minV || n > maxV {
		return 0, fmt.Errorf("must be an integer between %d and %d", minV, maxV)
	}
	return n, nil
}

type castDTO struct {
	// ID is the TMDB person, absent when unknown; clients filter the catalog on it (actors or creators).
	ID        int     `json:"id,omitempty"`
	Name      string  `json:"name"`
	Character string  `json:"character,omitempty"`
	PhotoURL  *string `json:"photoUrl"`
}

type trackDTO struct {
	Index      int    `json:"index"`
	Language   string `json:"language,omitempty"`
	Codec      string `json:"codec"`
	Title      string `json:"title,omitempty"`
	Channels   int    `json:"channels,omitempty"`
	Default    bool   `json:"default"`
	Forced     bool   `json:"forced,omitempty"`
	ImageBased bool   `json:"imageBased,omitempty"`
}

type versionDTO struct {
	ID              uuid.UUID  `json:"id"`
	Label           string     `json:"label"`
	Resolution      string     `json:"resolution,omitempty"`
	HDR             bool       `json:"hdr"`
	VideoCodec      string     `json:"videoCodec,omitempty"`
	Edition         *string    `json:"edition"`
	Size            int64      `json:"size"`
	DurationSeconds *float64   `json:"durationSeconds"`
	Audio           []trackDTO `json:"audio"`
	Subtitles       []trackDTO `json:"subtitles"`
	// ExternalSubtitles were downloaded online for this file.
	ExternalSubtitles []externalSubtitleDTO `json:"externalSubtitles"`
	// Set when the file could not be analysed; such a version may not play.
	Error *string `json:"error,omitempty"`
	// Path is the file location on the server, only sent to local admins.
	Path string `json:"path,omitempty"`
}

type childDTO struct {
	ID               uuid.UUID  `json:"id"`
	Kind             string     `json:"kind"`
	Title            string     `json:"title"`
	Overview         *string    `json:"overview"`
	Year             *int32     `json:"year"`
	ReleaseDate      *time.Time `json:"releaseDate"`
	RuntimeMinutes   *int32     `json:"runtimeMinutes"`
	Rating           *float32   `json:"rating"`
	PosterURL        *string    `json:"posterUrl"`
	ThumbURL         *string    `json:"thumbUrl"`
	SeasonNumber     *int32     `json:"seasonNumber,omitempty"`
	EpisodeNumber    *int32     `json:"episodeNumber,omitempty"`
	EpisodeNumberEnd *int32     `json:"episodeNumberEnd,omitempty"`
	ChildCount       int32      `json:"childCount,omitempty"`
	Watched          bool       `json:"watched"`
	// PositionSeconds is the resume point, 0 when none.
	PositionSeconds float64 `json:"positionSeconds"`
	// UnwatchedCount counts the episodes of a season the caller has not watched yet.
	UnwatchedCount int32 `json:"unwatchedCount,omitempty"`
}

type ancestorDTO struct {
	ID           uuid.UUID `json:"id"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	SeasonNumber *int32    `json:"seasonNumber,omitempty"`
}

type itemDetailDTO struct {
	ID               uuid.UUID     `json:"id"`
	Kind             string        `json:"kind"`
	LibraryID        uuid.UUID     `json:"libraryId"`
	Title            string        `json:"title"`
	OriginalTitle    *string       `json:"originalTitle"`
	Year             *int32        `json:"year"`
	Overview         *string       `json:"overview"`
	Tagline          *string       `json:"tagline"`
	ReleaseDate      *time.Time    `json:"releaseDate"`
	RuntimeMinutes   *int32        `json:"runtimeMinutes"`
	Rating           *float32      `json:"rating"`
	Genres           []string      `json:"genres"`
	Directors        []string      `json:"directors"`
	DirectorCredits  []castDTO     `json:"directorCredits"`
	Cast             []castDTO     `json:"cast"`
	PosterURL        *string       `json:"posterUrl"`
	BackdropURL      *string       `json:"backdropUrl"`
	LogoURL          *string       `json:"logoUrl"`
	ThumbURL         *string       `json:"thumbUrl"`
	TMDBID           *int32        `json:"tmdbId"`
	MatchStatus      string        `json:"matchStatus"`
	SeasonNumber     *int32        `json:"seasonNumber,omitempty"`
	EpisodeNumber    *int32        `json:"episodeNumber,omitempty"`
	EpisodeNumberEnd *int32        `json:"episodeNumberEnd,omitempty"`
	Ancestors        []ancestorDTO `json:"ancestors"`
	Versions         []versionDTO  `json:"versions"`
	Children         []childDTO    `json:"children"`
	// Watched covers the item itself, or everything under a series or season.
	Watched bool `json:"watched"`
	// PositionSeconds is the resume point of a playable item, 0 when none.
	PositionSeconds float64 `json:"positionSeconds"`
	// PlayTarget is what "play" starts on a series or season.
	PlayTarget *playTargetDTO `json:"playTarget"`
	// Next follows an episode in viewing order.
	Next *playTargetDTO `json:"next"`
	// InWatchlist is set when the caller added this top-level title, or another copy of it, to their watchlist.
	InWatchlist bool `json:"inWatchlist"`
	// Sources lists the servers holding this title; set on top-level items for local users only.
	Sources []cardSourceDTO `json:"sources"`
	// SubtitleSearch tells whether subtitles can be searched online for the versions of this item.
	SubtitleSearch bool `json:"subtitleSearch"`
	// Folder is the deepest directory holding every file of the item and its children, only sent to local admins.
	Folder string `json:"folder,omitempty"`
	// EditedFields lists the fields an administrator edited, which metadata refreshes keep; only sent to local admins.
	EditedFields []string `json:"editedFields,omitempty"`
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "item")
	if !ok {
		return
	}
	ctx := r.Context()
	it, err := s.q.GetItem(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "item not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	allowed, err := s.canAccessLibrary(r, it.LibraryID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if !allowed {
		httpx.Error(w, http.StatusNotFound, "not_found", "item not found")
		return
	}

	dto := itemDetailDTO{
		ID: it.ID, Kind: it.Kind, LibraryID: it.LibraryID, Title: it.Title, OriginalTitle: it.OriginalTitle,
		Year: it.Year, Overview: it.Overview, Tagline: it.Tagline, ReleaseDate: it.ReleaseDate,
		RuntimeMinutes: it.RuntimeMinutes, Rating: it.Rating, Genres: it.Genres, Directors: it.Directors,
		PosterURL: imageURL(it.PosterPath, "w500"), BackdropURL: imageURL(it.BackdropPath, "w1280"),
		ThumbURL: imageURL(it.ThumbPath, "w500"), TMDBID: it.TmdbID, MatchStatus: it.MatchStatus,
		SeasonNumber: it.SeasonNumber, EpisodeNumber: it.EpisodeNumber, EpisodeNumberEnd: it.EpisodeNumberEnd,
		Ancestors: []ancestorDTO{}, Cast: []castDTO{}, Sources: []cardSourceDTO{},
	}
	labels := personLabels(it.FilterLabels)
	if dto.Cast, err = castFromJSON(it.CastMembers, labels["actors"]); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if dto.DirectorCredits, err = castFromJSON(it.DirectorMembers, labels["creators"]); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	// Seasons and episodes show the logo of their series.
	var logoPath *string
	if dto.Ancestors, logoPath, err = s.ancestors(r, it); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	dto.LogoURL = imageURL(logoPath, "w500")
	if dto.Versions, err = s.versions(r, it.ID); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if dto.Children, err = s.children(r, it.ID); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if dto.SubtitleSearch, err = s.subtitleSearchAvailable(ctx); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if canSeePaths(r) {
		paths, err := s.q.ListItemTreeFilePaths(ctx, it.ID)
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		dto.Folder = commonFolder(paths)
		dto.EditedFields = it.LockedFields
	}
	if err := s.withProgress(r, it, &dto); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if it.ParentID == nil {
		p, _ := auth.PrincipalFrom(ctx)
		if dto.InWatchlist, err = s.q.IsInWatchlist(ctx, store.IsInWatchlistParams{UserID: p.UserID, ItemID: it.ID}); err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	if it.ParentID == nil && it.TmdbID != nil && !isFederatedRequest(ctx) {
		p, _ := auth.PrincipalFrom(ctx)
		title, err := s.titleSources(ctx, p.UserID, p.Role == auth.RoleAdmin, it.Kind, *it.TmdbID)
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		dto.Sources = title.Sources
		// The watchlist holds the title, whichever copy was added and whichever server holds it.
		dto.InWatchlist = dto.InWatchlist || title.InWatchlist
	}
	httpx.JSON(w, http.StatusOK, dto)
}

// ancestors returns the chain from the top-level item down to the parent (series, season), and the logo of the
// top-level item.
func (s *Server) ancestors(r *http.Request, it store.Item) ([]ancestorDTO, *string, error) {
	out := []ancestorDTO{}
	logo := it.LogoPath
	for parent := it.ParentID; parent != nil; {
		p, err := s.q.GetItem(r.Context(), *parent)
		if err != nil {
			return nil, nil, err
		}
		out = append([]ancestorDTO{{ID: p.ID, Kind: p.Kind, Title: p.Title, SeasonNumber: p.SeasonNumber}}, out...)
		logo = p.LogoPath
		parent = p.ParentID
	}
	return out, logo, nil
}

func (s *Server) children(r *http.Request, id uuid.UUID) ([]childDTO, error) {
	p, _ := auth.PrincipalFrom(r.Context())
	rows, err := s.q.ListChildItems(r.Context(), store.ListChildItemsParams{UserID: p.UserID, ParentID: &id})
	if err != nil {
		return nil, err
	}
	out := make([]childDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, childDTO{
			ID: c.ID, Kind: c.Kind, Title: c.Title, Overview: c.Overview, Year: c.Year, ReleaseDate: c.ReleaseDate,
			RuntimeMinutes: c.RuntimeMinutes, Rating: c.Rating, PosterURL: imageURL(c.PosterPath, "w342"),
			ThumbURL: imageURL(c.ThumbPath, "w300"), SeasonNumber: c.SeasonNumber, EpisodeNumber: c.EpisodeNumber,
			EpisodeNumberEnd: c.EpisodeNumberEnd, ChildCount: c.ChildCount,
			Watched:         c.Watched || (c.Kind == "season" && c.UnwatchedCount == 0),
			PositionSeconds: resumePoint(c.Watched, c.PositionSeconds), UnwatchedCount: c.UnwatchedCount,
		})
	}
	return out, nil
}

func (s *Server) versions(r *http.Request, id uuid.UUID) ([]versionDTO, error) {
	rows, err := s.q.ListItemFiles(r.Context(), []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	showPath := canSeePaths(r)
	ids := make([]uuid.UUID, len(rows))
	for i, f := range rows {
		ids[i] = f.ID
	}
	subs, err := s.q.ListExternalSubtitles(r.Context(), ids)
	if err != nil {
		return nil, err
	}
	out := make([]versionDTO, 0, len(rows))
	for _, f := range rows {
		var streams []scanner.Stream
		if err := json.Unmarshal(f.Streams, &streams); err != nil {
			return nil, fmt.Errorf("decode streams of file %s: %w", f.ID, err)
		}
		v := describeVersion(f, streams)
		for _, sub := range subs {
			if sub.FileID == f.ID {
				v.ExternalSubtitles = append(v.ExternalSubtitles, externalSubtitle(sub.ID, sub.Provider, sub.Language, sub.ReleaseName, sub.HearingImpaired))
			}
		}
		if showPath {
			v.Path = f.Path
		}
		out = append(out, v)
	}
	return out, nil
}

// canSeePaths tells whether file paths may be sent: they reveal the server's directory layout,
// so peers and regular users never see them.
func canSeePaths(r *http.Request) bool {
	p, _ := auth.PrincipalFrom(r.Context())
	return p.Role == auth.RoleAdmin && !isFederatedRequest(r.Context())
}

// commonFolder returns the deepest directory holding every path, "" when there is none.
func commonFolder(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	folder := path.Dir(paths[0])
	for _, p := range paths[1:] {
		for folder != "/" && folder != "." && !strings.HasPrefix(p, folder+"/") {
			folder = path.Dir(folder)
		}
	}
	return folder
}

func describeVersion(f store.ListItemFilesRow, streams []scanner.Stream) versionDTO {
	v := versionDTO{
		ID: f.ID, Edition: f.Edition, Size: f.Size, DurationSeconds: f.DurationSeconds,
		Audio: []trackDTO{}, Subtitles: []trackDTO{}, ExternalSubtitles: []externalSubtitleDTO{}, Error: f.ProbeError,
	}
	for _, st := range streams {
		t := trackDTO{Index: st.Index, Language: st.Language, Codec: st.Codec, Title: st.Title, Channels: st.Channels,
			Default: st.Default, Forced: st.Forced, ImageBased: st.ImageBased}
		switch st.Type {
		case "video":
			if v.VideoCodec == "" {
				v.Resolution = resolutionLabel(st.Width, st.Height)
				v.VideoCodec = codecLabel(st.Codec)
				v.HDR = st.ColorTransfer == "smpte2084" || st.ColorTransfer == "arib-std-b67"
			}
		case "audio":
			v.Audio = append(v.Audio, t)
		case "subtitle":
			v.Subtitles = append(v.Subtitles, t)
		}
	}
	var parts []string
	for _, p := range []string{v.Resolution, map[bool]string{true: "HDR"}[v.HDR], v.VideoCodec} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if v.Edition != nil {
		parts = append(parts, *v.Edition)
	}
	v.Label = strings.Join(parts, " · ")
	return v
}

// resolutionLabel uses the width too, so that cropped widescreen films (1920x800) still count as 1080p.
func resolutionLabel(w, h int) string {
	switch {
	case w == 0 && h == 0:
		return ""
	case w >= 3200 || h >= 1800:
		return "2160p"
	case w >= 1700 || h >= 1000:
		return "1080p"
	case w >= 1200 || h >= 700:
		return "720p"
	default:
		return "SD"
	}
}

func codecLabel(codec string) string {
	switch codec {
	case "hevc":
		return "HEVC"
	case "h264":
		return "H.264"
	case "av1":
		return "AV1"
	case "vp9":
		return "VP9"
	case "mpeg4", "msmpeg4v3":
		return "MPEG-4"
	case "mpeg2video":
		return "MPEG-2"
	default:
		return strings.ToUpper(codec)
	}
}

type storedCast struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profilePath"`
}

// castFromJSON reads stored credits. People stored before their TMDB ID was kept get it from the item's filter
// labels (name to ID), which the same metadata refresh wrote.
func castFromJSON(raw []byte, ids map[string]int) ([]castDTO, error) {
	var cast []storedCast
	if err := json.Unmarshal(raw, &cast); err != nil {
		return nil, err
	}
	out := make([]castDTO, 0, len(cast))
	for _, c := range cast {
		path := c.ProfilePath
		id := c.ID
		if id <= 0 {
			id = ids[c.Name]
		}
		out = append(out, castDTO{ID: id, Name: c.Name, Character: c.Character, PhotoURL: imageURL(&path, "w185")})
	}
	return out, nil
}

// personLabels maps, per filter category, a person's name to their TMDB ID. A name shared by two people of the
// same category maps to nothing, rather than to the wrong one.
func personLabels(raw []byte) map[string]map[string]int {
	var labels map[string][]struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &labels) != nil {
		return nil
	}
	out := make(map[string]map[string]int, 2)
	for _, category := range []string{"actors", "creators"} {
		ids := make(map[string]int, len(labels[category]))
		for _, option := range labels[category] {
			id, err := strconv.Atoi(option.ID)
			if err != nil || id <= 0 {
				continue
			}
			if _, dup := ids[option.Label]; dup {
				ids[option.Label] = 0
				continue
			}
			ids[option.Label] = id
		}
		out[category] = ids
	}
	return out
}
