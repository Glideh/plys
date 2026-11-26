package api

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/httpx"
	"toto/api/internal/metadata"
	"toto/api/internal/opensubtitles"
	"toto/api/internal/settings"
	"toto/api/internal/store"
)

// Version is set at build time with -ldflags "-X toto/api/internal/api.Version=...".
var Version = "dev"

const (
	// Image URLs embed the TMDB path, which changes when the image changes.
	imageCacheControl = "public, max-age=2592000, immutable"
	// Only the manual matching screen shows posters that are not in the catalogue yet.
	searchPosterBaseURL = metadata.DefaultImageBaseURL + "/w185"
)

var localThumbRe = regexp.MustCompile(`^[0-9a-f-]{36}\.jpg$`)
var fanKaiImageRe = regexp.MustCompile(`^[0-9a-f-]{36}-(?:[0-9]+-)?(?:poster|backdrop|thumb)\.(?:jpg|png|webp)$`)

func (s *Server) serverInfo(w http.ResponseWriter, r *http.Request) {
	name, err := s.Settings.ServerName(r.Context())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	info := map[string]any{
		"id":        s.Identity.ID,
		"name":      name,
		"version":   Version,
		"publicKey": s.Identity.PublicKey,
	}
	if update := s.Releases.Available(); update != nil {
		info["update"] = update
	}
	httpx.JSON(w, http.StatusOK, info)
}

type settingsDTO struct {
	TMDBConfigured   bool   `json:"tmdbConfigured"`
	MetadataLanguage string `json:"metadataLanguage"`
	ServerName       string `json:"serverName"`
	PublicURL        string `json:"publicUrl"`
	// The OpenSubtitles API key and password are write-only, like the TMDB key.
	OpenSubtitlesConfigured bool   `json:"openSubtitlesConfigured"`
	OpenSubtitlesUsername   string `json:"openSubtitlesUsername"`
}

func (s *Server) currentSettings(r *http.Request) (settingsDTO, error) {
	ctx := r.Context()
	key, err := s.Settings.TMDBKey(ctx)
	if err != nil {
		return settingsDTO{}, err
	}
	lang, err := s.Settings.MetadataLanguage(ctx)
	if err != nil {
		return settingsDTO{}, err
	}
	name, err := s.Settings.ServerName(ctx)
	if err != nil {
		return settingsDTO{}, err
	}
	publicURL, err := s.Settings.ServerPublicURL(ctx)
	if err != nil {
		return settingsDTO{}, err
	}
	osub, err := s.Settings.OpenSubtitles(ctx)
	return settingsDTO{
		TMDBConfigured: key != "", MetadataLanguage: lang, ServerName: name, PublicURL: publicURL,
		OpenSubtitlesConfigured: osub.APIKey != "", OpenSubtitlesUsername: osub.Username,
	}, err
}

// The TMDB key is write-only: it is never sent back to clients.
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	dto, err := s.currentSettings(r)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dto)
}

type updateSettingsRequest struct {
	TMDBAPIKey       *string `json:"tmdbApiKey"`
	MetadataLanguage *string `json:"metadataLanguage"`
	ServerName       *string `json:"serverName"`
	PublicURL        *string `json:"publicUrl"`
	// An empty key removes the OpenSubtitles configuration; an empty username removes the login.
	OpenSubtitlesAPIKey   *string `json:"openSubtitlesApiKey"`
	OpenSubtitlesUsername *string `json:"openSubtitlesUsername"`
	OpenSubtitlesPassword *string `json:"openSubtitlesPassword"`
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req updateSettingsRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	ctx := r.Context()
	if req.MetadataLanguage != nil && !settings.ValidLanguage(*req.MetadataLanguage) {
		httpx.Error(w, http.StatusBadRequest, "validation", "metadataLanguage must look like fr-FR")
		return
	}
	if req.ServerName != nil && !settings.ValidServerName(strings.TrimSpace(*req.ServerName)) {
		httpx.Error(w, http.StatusBadRequest, "validation", "serverName must be 1 to 64 characters")
		return
	}
	var publicURL string
	if req.PublicURL != nil && *req.PublicURL != "" {
		normalized, ok := settings.NormalizePublicURL(*req.PublicURL)
		if !ok {
			httpx.Error(w, http.StatusBadRequest, "validation", "publicUrl must be an absolute http(s) URL without path")
			return
		}
		publicURL = normalized
	}
	if req.TMDBAPIKey != nil {
		key := strings.TrimSpace(*req.TMDBAPIKey)
		if key != "" {
			err := s.Metadata.ValidateKey(ctx, key)
			if errors.Is(err, metadata.ErrUnauthorized) {
				httpx.Error(w, http.StatusBadRequest, "invalid_tmdb_key", "TMDB rejected this API key")
				return
			}
			if err != nil {
				httpx.Error(w, http.StatusBadGateway, "tmdb_unreachable", "TMDB could not be reached to check the key")
				return
			}
		}
		if err := s.Settings.SetTMDBKey(ctx, key); err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	if req.OpenSubtitlesAPIKey != nil || req.OpenSubtitlesUsername != nil || req.OpenSubtitlesPassword != nil {
		if !s.updateOpenSubtitles(w, r, req) {
			return
		}
	}
	if req.MetadataLanguage != nil {
		if err := s.Settings.SetMetadataLanguage(ctx, *req.MetadataLanguage); err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	if req.ServerName != nil {
		if err := s.Settings.SetServerName(ctx, strings.TrimSpace(*req.ServerName)); err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	if req.PublicURL != nil {
		if err := s.Settings.SetServerPublicURL(ctx, publicURL); err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	if req.TMDBAPIKey != nil || req.MetadataLanguage != nil {
		s.Scanner.EnqueueAll(ctx)
	}
	s.getSettings(w, r)
}

// updateOpenSubtitles merges the fields sent with the stored ones, and checks them with OpenSubtitles before saving.
func (s *Server) updateOpenSubtitles(w http.ResponseWriter, r *http.Request, req updateSettingsRequest) bool {
	ctx := r.Context()
	cur, err := s.Settings.OpenSubtitles(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return false
	}
	if req.OpenSubtitlesAPIKey != nil {
		cur.APIKey = strings.TrimSpace(*req.OpenSubtitlesAPIKey)
	}
	if req.OpenSubtitlesUsername != nil {
		cur.Username = strings.TrimSpace(*req.OpenSubtitlesUsername)
	}
	if req.OpenSubtitlesPassword != nil {
		cur.Password = *req.OpenSubtitlesPassword
	}
	if cur.Username == "" {
		cur.Password = ""
	}
	if cur.APIKey == "" {
		cur = settings.OpenSubtitles{}
	}
	if cur.Username != "" && cur.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "validation", "openSubtitlesPassword is required with a username")
		return false
	}
	if cur.APIKey != "" && s.OpenSubtitles != nil {
		err := s.OpenSubtitles.Validate(ctx, opensubtitles.Credentials{APIKey: cur.APIKey, Username: cur.Username, Password: cur.Password})
		if err != nil {
			slog.WarnContext(ctx, "OpenSubtitles credentials check failed", "err", err)
		}
		var apiErr *opensubtitles.APIError
		switch {
		case err == nil:
		case errors.Is(err, opensubtitles.ErrInvalidLogin):
			// The message is the OpenSubtitles one, shown as is to the administrator.
			httpx.Error(w, http.StatusBadRequest, "invalid_opensubtitles_login", openSubtitlesDetail(err))
			return false
		case errors.Is(err, opensubtitles.ErrUnauthorized):
			httpx.Error(w, http.StatusBadRequest, "invalid_opensubtitles_credentials", openSubtitlesDetail(err))
			return false
		case errors.Is(err, opensubtitles.ErrRateLimited):
			httpx.Error(w, http.StatusTooManyRequests, "subtitles_rate_limited", "OpenSubtitles received too many requests, try again in a moment")
			return false
		case errors.As(err, &apiErr):
			httpx.Error(w, http.StatusBadGateway, "subtitles_rejected", apiErr.Detail())
			return false
		default:
			httpx.Error(w, http.StatusBadGateway, "subtitles_unreachable", "OpenSubtitles could not be reached to check the credentials")
			return false
		}
	}
	if err := s.Settings.SetOpenSubtitles(ctx, cur); err != nil {
		httpx.InternalError(w, r, err)
		return false
	}
	return true
}

// openSubtitlesDetail returns what OpenSubtitles answered to a rejected check, such as "Invalid username/password (HTTP 401)".
func openSubtitlesDetail(err error) string {
	var apiErr *opensubtitles.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Detail()
	}
	return "no detail given"
}

type matchCandidateDTO struct {
	TMDBID        int     `json:"tmdbId"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"originalTitle,omitempty"`
	Year          int     `json:"year,omitempty"`
	Overview      string  `json:"overview,omitempty"`
	PosterURL     *string `json:"posterUrl"`
}

func (s *Server) searchMatch(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadMatchableItem(w, r)
	if !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = it.Title
	}
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	lib, err := s.q.GetLibrary(r.Context(), it.LibraryID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	results, err := s.Metadata.Search(r.Context(), lib, it.Kind, query, year)
	if s.writeMetadataError(w, r, err) {
		return
	}
	out := make([]matchCandidateDTO, 0, len(results))
	for _, res := range results {
		title, original := res.Titles()
		dto := matchCandidateDTO{TMDBID: res.ID, Title: title, OriginalTitle: original, Year: res.Year(), Overview: res.Overview}
		if res.PosterPath != "" {
			u := searchPosterBaseURL + res.PosterPath
			dto.PosterURL = &u
		}
		out = append(out, dto)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type fanKaiMatchCandidateDTO struct {
	FanKaiID      int    `json:"fankaiId"`
	Title         string `json:"title"`
	OriginalTitle string `json:"originalTitle,omitempty"`
	Year          int    `json:"year,omitempty"`
	Overview      string `json:"overview,omitempty"`
}

func (s *Server) searchFanKaiMatch(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadMatchableItem(w, r)
	if !ok {
		return
	}
	if it.Kind != "series" {
		httpx.Error(w, http.StatusBadRequest, "not_matchable", "FanKai metadata is available for series only")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = it.Title
	}
	if utf8.RuneCountInString(query) > 200 {
		httpx.Error(w, http.StatusBadRequest, "validation", "query must be at most 200 characters")
		return
	}
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	results, err := s.Metadata.SearchFanKai(r.Context(), query, year)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "fankai_unreachable", "FanKai could not be reached to search series")
		return
	}
	out := make([]fanKaiMatchCandidateDTO, 0, len(results))
	for _, result := range results {
		out = append(out, fanKaiMatchCandidateDTO{
			FanKaiID: result.ID, Title: result.Title, OriginalTitle: result.OriginalTitle,
			Year: result.Year, Overview: result.Overview,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type setFanKaiMatchRequest struct {
	FanKaiID int `json:"fankaiId"`
}

func (s *Server) setFanKaiMatch(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadMatchableItem(w, r)
	if !ok {
		return
	}
	if it.Kind != "series" {
		httpx.Error(w, http.StatusBadRequest, "not_matchable", "FanKai metadata is available for series only")
		return
	}
	var req setFanKaiMatchRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if req.FanKaiID <= 0 || req.FanKaiID > 2_147_483_647 {
		httpx.Error(w, http.StatusBadRequest, "validation", "fankaiId must be a positive 32-bit integer")
		return
	}
	if err := s.Metadata.SetFanKaiMatch(r.Context(), it.ID, req.FanKaiID); err != nil {
		switch {
		case errors.Is(err, metadata.ErrFanKaiNotFound):
			httpx.Error(w, http.StatusNotFound, "fankai_not_found", "no such series on FanKai")
		case errors.Is(err, metadata.ErrFanKaiUnavailable):
			httpx.Error(w, http.StatusBadGateway, "fankai_unreachable", "FanKai could not be reached to retrieve the series")
		default:
			httpx.InternalError(w, r, err)
		}
		return
	}
	s.respondItemSummary(w, r, it.ID)
}

type setMatchRequest struct {
	TMDBID int `json:"tmdbId"`
}

func (s *Server) setMatch(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadMatchableItem(w, r)
	if !ok {
		return
	}
	var req setMatchRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if req.TMDBID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "validation", "tmdbId must be positive")
		return
	}
	s.refreshAndRespond(w, r, it, &req.TMDBID)
}

func (s *Server) refreshItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadMatchableItem(w, r)
	if !ok {
		return
	}
	s.refreshAndRespond(w, r, it, nil)
}

func (s *Server) refreshAndRespond(w http.ResponseWriter, r *http.Request, it store.Item, tmdbID *int) {
	if s.writeMetadataError(w, r, s.Metadata.RefreshItem(r.Context(), it.ID, tmdbID)) {
		return
	}
	// A merge may have removed the item in favour of an older duplicate.
	s.respondItemSummary(w, r, it.ID)
}

func (s *Server) loadMatchableItem(w http.ResponseWriter, r *http.Request) (store.Item, bool) {
	id, ok := idParam(w, r, "item")
	if !ok {
		return store.Item{}, false
	}
	it, err := s.q.GetItem(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "item not found")
		return it, false
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return it, false
	}
	if it.Kind != "movie" && it.Kind != "series" && it.Kind != "video" {
		httpx.Error(w, http.StatusBadRequest, "not_matchable", "only movies, series and videos have their own metadata")
		return it, false
	}
	return it, true
}

func (s *Server) writeMetadataError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, metadata.ErrNoAPIKey):
		httpx.Error(w, http.StatusConflict, "tmdb_not_configured", "configure a TMDB API key first")
	case errors.Is(err, metadata.ErrUnauthorized):
		httpx.Error(w, http.StatusConflict, "invalid_tmdb_key", "TMDB rejected the configured API key")
	case errors.Is(err, metadata.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "tmdb_not_found", "no such title on TMDB")
	case errors.Is(err, metadata.ErrFanKaiNotFound):
		httpx.Error(w, http.StatusNotFound, "fankai_not_found", "no such series on FanKai")
	case errors.Is(err, metadata.ErrFanKaiUnavailable):
		httpx.Error(w, http.StatusBadGateway, "fankai_unreachable", "FanKai could not be reached")
	default:
		httpx.InternalError(w, r, err)
	}
	return true
}

type itemSummaryDTO struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`
	Title       string    `json:"title"`
	Year        *int32    `json:"year"`
	TMDBID      *int32    `json:"tmdbId"`
	MatchStatus string    `json:"matchStatus"`
	PosterURL   *string   `json:"posterUrl"`
}

func toItemSummary(it store.Item) itemSummaryDTO {
	return itemSummaryDTO{
		ID: it.ID, Kind: it.Kind, Title: it.Title, Year: it.Year, TMDBID: it.TmdbID,
		MatchStatus: it.MatchStatus, PosterURL: imageURL(it.PosterPath, "w342"),
	}
}

// imageURL maps a stored image path to the API endpoint serving it.
func imageURL(path *string, size string) *string {
	if path == nil || *path == "" {
		return nil
	}
	var u string
	if strings.HasPrefix(*path, "fankai/") {
		u = "/api/v1/images/" + *path
	} else if strings.HasPrefix(*path, "local/") {
		u = "/api/v1/images/" + *path
	} else {
		u = "/api/v1/images/tmdb/" + size + *path
	}
	return &u
}

func (s *Server) fanKaiImage(w http.ResponseWriter, r *http.Request) {
	file := chi.URLParam(r, "file")
	if !fanKaiImageRe.MatchString(file) {
		httpx.Error(w, http.StatusNotFound, "not_found", "image not found")
		return
	}
	if s.Images == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "image_cache_unavailable", "image cache is unavailable")
		return
	}
	path := s.Images.FanKaiLocal(file)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "image not found")
			return
		}
		httpx.InternalError(w, r, err)
		return
	}
	serveImage(w, r, path)
}

func (s *Server) tmdbImage(w http.ResponseWriter, r *http.Request) {
	size, file := chi.URLParam(r, "size"), chi.URLParam(r, "file")
	if !metadata.ValidImageRequest(size, file) {
		httpx.Error(w, http.StatusNotFound, "not_found", "image not found")
		return
	}
	// A cached file was referenced when downloaded: skip the database lookup on the hot path.
	if path, ok := s.Images.Cached(size, file); ok {
		serveImage(w, r, path)
		return
	}
	// Refusing unknown paths keeps the endpoint from acting as an open proxy to TMDB.
	known, err := s.q.IsImagePathReferenced(r.Context(), "/"+file)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if !known {
		httpx.Error(w, http.StatusNotFound, "not_found", "image not found")
		return
	}
	path, err := s.Images.TMDB(r.Context(), size, file)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "image_unavailable", "image could not be fetched")
		return
	}
	serveImage(w, r, path)
}

func (s *Server) localImage(w http.ResponseWriter, r *http.Request) {
	file := chi.URLParam(r, "file")
	if !localThumbRe.MatchString(file) {
		httpx.Error(w, http.StatusNotFound, "not_found", "image not found")
		return
	}
	serveImage(w, r, s.Images.Local(file))
}

func serveImage(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "image not found")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", imageCacheControl)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	modTime := info.ModTime()
	if modTime.IsZero() {
		modTime = time.Now()
	}
	http.ServeContent(w, r, filepath.Base(path), modTime, f)
}
