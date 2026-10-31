package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/httpx"
	"toto/api/internal/metadata"
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

func (s *Server) serverInfo(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id":        s.Identity.ID,
		"name":      "Plys",
		"version":   Version,
		"publicKey": s.Identity.PublicKey,
	})
}

type settingsDTO struct {
	TMDBConfigured   bool   `json:"tmdbConfigured"`
	MetadataLanguage string `json:"metadataLanguage"`
}

func (s *Server) currentSettings(r *http.Request) (settingsDTO, error) {
	key, err := s.Settings.TMDBKey(r.Context())
	if err != nil {
		return settingsDTO{}, err
	}
	lang, err := s.Settings.MetadataLanguage(r.Context())
	return settingsDTO{TMDBConfigured: key != "", MetadataLanguage: lang}, err
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
	if req.MetadataLanguage != nil {
		if err := s.Settings.SetMetadataLanguage(ctx, *req.MetadataLanguage); err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	// Rescans are cheap when files are unchanged and run the metadata pass with the new settings.
	s.Scanner.EnqueueAll(ctx)
	s.getSettings(w, r)
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
	results, err := s.Metadata.Search(r.Context(), it.Kind, query, year)
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
	updated, err := s.q.GetItem(r.Context(), it.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.JSON(w, http.StatusOK, map[string]any{"merged": true})
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toItemSummary(updated))
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
	if strings.HasPrefix(*path, "local/") {
		u = "/api/v1/images/" + *path
	} else {
		u = "/api/v1/images/tmdb/" + size + *path
	}
	return &u
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
