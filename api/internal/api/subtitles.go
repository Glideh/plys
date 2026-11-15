package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/opensubtitles"
	"toto/api/internal/store"
)

const (
	providerOpenSubtitles = "opensubtitles"
	maxReleaseLength      = 255
)

// OpenSubtitles language codes: ISO 639-1, with a region for a few languages (pt-BR, zh-CN...).
var subtitleLanguageRe = regexp.MustCompile(`^[a-z]{2}(-[A-Za-z]{2})?$`)

type externalSubtitleDTO struct {
	ID              uuid.UUID `json:"id"`
	Language        string    `json:"language"`
	Release         string    `json:"release"`
	HearingImpaired bool      `json:"hearingImpaired"`
	Source          string    `json:"source"`
}

func externalSubtitle(id uuid.UUID, provider, language, release string, hearingImpaired bool) externalSubtitleDTO {
	return externalSubtitleDTO{ID: id, Language: language, Release: release, HearingImpaired: hearingImpaired, Source: provider}
}

// subtitleSearchAvailable tells clients whether to offer the online search.
func (s *Server) subtitleSearchAvailable(ctx context.Context) (bool, error) {
	if s.OpenSubtitles == nil {
		return false, nil
	}
	return s.OpenSubtitles.Configured(ctx)
}

// subtitleTarget loads the item and the version named in the request, checking the caller may play them.
func (s *Server) subtitleTarget(w http.ResponseWriter, r *http.Request, versionID uuid.UUID) (store.Item, store.GetPlaybackFileRow, bool) {
	id, ok := idParam(w, r, "item")
	if !ok {
		return store.Item{}, store.GetPlaybackFileRow{}, false
	}
	ctx := r.Context()
	it, err := s.q.GetItem(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "item not found")
		return it, store.GetPlaybackFileRow{}, false
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return it, store.GetPlaybackFileRow{}, false
	}
	allowed, err := s.canAccessLibrary(r, it.LibraryID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return it, store.GetPlaybackFileRow{}, false
	}
	if !allowed {
		httpx.Error(w, http.StatusNotFound, "not_found", "item not found")
		return it, store.GetPlaybackFileRow{}, false
	}
	f, err := s.q.GetPlaybackFile(ctx, versionID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (f.ItemID == nil || *f.ItemID != it.ID)) {
		httpx.Error(w, http.StatusNotFound, "not_found", "version not found or unreadable")
		return it, f, false
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return it, f, false
	}
	return it, f, true
}

// subtitleQuery identifies the video by TMDB IDs when it is matched, by title otherwise, and by its file hash.
func (s *Server) subtitleQuery(ctx context.Context, it store.Item, path, language string) (opensubtitles.Query, error) {
	q := opensubtitles.Query{Language: language, Text: it.Title}
	if hash, err := opensubtitles.MovieHash(path); err == nil {
		q.MovieHash = hash
	} else {
		slog.DebugContext(ctx, "subtitle hash", "path", path, "err", err)
	}
	matched := func(i store.Item) bool {
		return i.TmdbID != nil && (i.MatchStatus == "matched" || i.MatchStatus == "manual")
	}
	switch it.Kind {
	case "movie":
		if matched(it) {
			q.TMDBID = int(*it.TmdbID)
		}
	case "episode":
		season := it.SeasonNumber
		parent := it.ParentID
		for parent != nil {
			p, err := s.q.GetItem(ctx, *parent)
			if err != nil {
				return q, err
			}
			if p.Kind == "season" && season == nil {
				season = p.SeasonNumber
			}
			if p.Kind == "series" {
				q.Text = p.Title
				if matched(p) && season != nil && it.EpisodeNumber != nil {
					q.ParentTMDBID, q.Season, q.Episode = int(*p.TmdbID), int(*season), int(*it.EpisodeNumber)
				}
				break
			}
			parent = p.ParentID
		}
	}
	return q, nil
}

type subtitleSearchResponse struct {
	Results []opensubtitles.Result `json:"results"`
}

// searchSubtitles lists the subtitles OpenSubtitles holds for a version, in one language.
func (s *Server) searchSubtitles(w http.ResponseWriter, r *http.Request) {
	versionID, err := uuid.Parse(r.URL.Query().Get("versionId"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "versionId must be a UUID")
		return
	}
	language := r.URL.Query().Get("language")
	if !subtitleLanguageRe.MatchString(language) {
		httpx.Error(w, http.StatusBadRequest, "validation", "language must look like fr or pt-BR")
		return
	}
	it, f, ok := s.subtitleTarget(w, r, versionID)
	if !ok {
		return
	}
	if s.OpenSubtitles == nil {
		subtitleProviderError(w, r, opensubtitles.ErrNotConfigured)
		return
	}
	q, err := s.subtitleQuery(r.Context(), it, f.Path, language)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	results, err := s.OpenSubtitles.Search(r.Context(), q)
	if err != nil {
		subtitleProviderError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, subtitleSearchResponse{Results: results})
}

type downloadSubtitleRequest struct {
	VersionID       uuid.UUID `json:"versionId"`
	FileID          int       `json:"fileId"`
	Language        string    `json:"language"`
	Release         string    `json:"release"`
	HearingImpaired bool      `json:"hearingImpaired"`
}

// downloadSubtitle stores a subtitle found by searchSubtitles; it is then offered to every user of the version.
func (s *Server) downloadSubtitle(w http.ResponseWriter, r *http.Request) {
	var req downloadSubtitleRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if req.FileID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "validation", "fileId must be a positive integer")
		return
	}
	if !subtitleLanguageRe.MatchString(req.Language) {
		httpx.Error(w, http.StatusBadRequest, "validation", "language must look like fr or pt-BR")
		return
	}
	req.Release = strings.TrimSpace(req.Release)
	if !utf8.ValidString(req.Release) || utf8.RuneCountInString(req.Release) > maxReleaseLength {
		httpx.Error(w, http.StatusBadRequest, "validation", "release must be at most 255 characters")
		return
	}
	_, f, ok := s.subtitleTarget(w, r, req.VersionID)
	if !ok {
		return
	}
	ctx := r.Context()
	providerID := strconv.Itoa(req.FileID)
	existing, err := s.q.GetExternalSubtitleByProvider(ctx, store.GetExternalSubtitleByProviderParams{
		FileID: f.ID, Provider: providerOpenSubtitles, ProviderFileID: providerID,
	})
	if err == nil {
		// Already downloaded for this version: no quota is spent.
		httpx.JSON(w, http.StatusOK, externalSubtitle(existing.ID, existing.Provider, existing.Language, existing.ReleaseName, existing.HearingImpaired))
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		httpx.InternalError(w, r, err)
		return
	}
	if s.OpenSubtitles == nil {
		subtitleProviderError(w, r, opensubtitles.ErrNotConfigured)
		return
	}
	srt, err := s.OpenSubtitles.Download(ctx, req.FileID)
	if err != nil {
		subtitleProviderError(w, r, err)
		return
	}
	vtt, err := opensubtitles.SRTToVTT(srt)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "subtitles_invalid", "the downloaded subtitle file could not be read")
		return
	}
	p, _ := auth.PrincipalFrom(ctx)
	row, err := s.q.InsertExternalSubtitle(ctx, store.InsertExternalSubtitleParams{
		FileID: f.ID, Provider: providerOpenSubtitles, ProviderFileID: providerID, Language: req.Language,
		ReleaseName: req.Release, HearingImpaired: req.HearingImpaired, Vtt: vtt, CreatedBy: &p.UserID,
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, externalSubtitle(row.ID, row.Provider, row.Language, row.ReleaseName, row.HearingImpaired))
}

// deleteSubtitle removes a downloaded subtitle that turned out wrong; reserved to admins.
func (s *Server) deleteSubtitle(w http.ResponseWriter, r *http.Request) {
	subID, err := uuid.Parse(chi.URLParam(r, "subtitleId"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "subtitle not found")
		return
	}
	versionID, err := uuid.Parse(r.URL.Query().Get("versionId"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "versionId must be a UUID")
		return
	}
	_, f, ok := s.subtitleTarget(w, r, versionID)
	if !ok {
		return
	}
	n, err := s.q.DeleteExternalSubtitle(r.Context(), store.DeleteExternalSubtitleParams{ID: subID, FileID: f.ID})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if n == 0 {
		httpx.Error(w, http.StatusNotFound, "not_found", "subtitle not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func subtitleProviderError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *opensubtitles.APIError
	switch {
	case errors.Is(err, opensubtitles.ErrNotConfigured):
		httpx.Error(w, http.StatusConflict, "subtitles_not_configured", "online subtitle search is not configured on this server")
	case errors.Is(err, opensubtitles.ErrUnauthorized):
		slog.WarnContext(r.Context(), "OpenSubtitles rejected the server credentials", "err", err)
		httpx.Error(w, http.StatusBadGateway, "subtitles_unauthorized", "OpenSubtitles rejected the server credentials")
	case errors.Is(err, opensubtitles.ErrQuota):
		httpx.Error(w, http.StatusTooManyRequests, "subtitles_quota", "the OpenSubtitles download quota is reached, try again later")
	case errors.Is(err, opensubtitles.ErrRateLimited):
		httpx.Error(w, http.StatusTooManyRequests, "subtitles_rate_limited", "OpenSubtitles received too many requests, try again in a moment")
	case errors.Is(err, context.Canceled):
		return
	case errors.As(err, &apiErr):
		slog.WarnContext(r.Context(), "OpenSubtitles request failed", "err", err)
		httpx.Error(w, http.StatusBadGateway, "subtitles_rejected", apiErr.Detail())
	default:
		slog.WarnContext(r.Context(), "OpenSubtitles request failed", "err", err)
		httpx.Error(w, http.StatusBadGateway, "subtitles_unreachable", "OpenSubtitles could not be reached")
	}
}
