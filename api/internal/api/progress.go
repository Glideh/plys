package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

const (
	// Below this position a started item is not offered for resuming.
	resumeMinSeconds = 30.0
	// Reaching this fraction of the duration marks an item as watched (the rest is credits).
	watchedFraction = 0.9
	homeRailSize    = 20
)

var playableKinds = []string{"movie", "episode", "part", "video"}

// accessibleItem loads an item the caller may see, answering 404 otherwise.
func (s *Server) accessibleItem(w http.ResponseWriter, r *http.Request) (store.Item, bool) {
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
	allowed, err := s.canAccessLibrary(r, it.LibraryID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return it, false
	}
	if !allowed {
		httpx.Error(w, http.StatusNotFound, "not_found", "item not found")
		return it, false
	}
	return it, true
}

type progressRequest struct {
	PositionSeconds float64 `json:"positionSeconds"`
	DurationSeconds float64 `json:"durationSeconds"`
}

func validSeconds(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }

// putProgress records where the caller stopped; reaching the end marks the item watched.
func (s *Server) putProgress(w http.ResponseWriter, r *http.Request) {
	it, ok := s.accessibleItem(w, r)
	if !ok {
		return
	}
	if !slices.Contains(playableKinds, it.Kind) {
		httpx.Error(w, http.StatusBadRequest, "validation", "only movies, episodes, parts and videos have a progress")
		return
	}
	var req progressRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if !validSeconds(req.PositionSeconds) || !validSeconds(req.DurationSeconds) {
		httpx.Error(w, http.StatusBadRequest, "validation", "positionSeconds and durationSeconds must be positive numbers")
		return
	}
	var duration *float64
	if req.DurationSeconds > 0 {
		duration = &req.DurationSeconds
		req.PositionSeconds = min(req.PositionSeconds, req.DurationSeconds)
	}
	watched := duration != nil && req.PositionSeconds >= req.DurationSeconds*watchedFraction

	p, _ := auth.PrincipalFrom(r.Context())
	watched, err := s.q.UpsertProgress(r.Context(), store.UpsertProgressParams{
		UserID: p.UserID, ItemID: it.ID, PositionSeconds: req.PositionSeconds, DurationSeconds: duration, Watched: watched,
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"watched": watched})
}

// setWatched marks an item, or everything under a series or season, as watched or not.
func (s *Server) setWatched(watched bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		it, ok := s.accessibleItem(w, r)
		if !ok {
			return
		}
		p, _ := auth.PrincipalFrom(r.Context())
		rows, err := s.q.ListPlayableUnder(r.Context(), store.ListPlayableUnderParams{UserID: p.UserID, Root: it.ID})
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		ids := make([]uuid.UUID, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		if watched {
			err = s.q.MarkWatched(r.Context(), store.MarkWatchedParams{UserID: p.UserID, ItemIds: ids})
		} else {
			err = s.q.MarkUnwatched(r.Context(), store.MarkUnwatchedParams{UserID: p.UserID, ItemIds: ids})
		}
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// resumePoint is the position worth resuming from, 0 when the item should start over.
func resumePoint(watched bool, position float64) float64 {
	if watched || position < resumeMinSeconds {
		return 0
	}
	return position
}

type playTargetDTO struct {
	ID              uuid.UUID `json:"id"`
	Kind            string    `json:"kind"`
	Title           string    `json:"title"`
	SeasonNumber    *int32    `json:"seasonNumber,omitempty"`
	EpisodeNumber   *int32    `json:"episodeNumber,omitempty"`
	PartNumber      *int32    `json:"partNumber,omitempty"`
	PositionSeconds float64   `json:"positionSeconds"`
	// Resume is set when the target was started; Rewatch when everything under the node was already watched.
	Resume  bool `json:"resume"`
	Rewatch bool `json:"rewatch"`
}

func targetOf(row store.ListPlayableUnderRow) playTargetDTO {
	return playTargetDTO{
		ID: row.ID, Kind: row.Kind, Title: row.Title, SeasonNumber: row.SeasonNumber,
		EpisodeNumber: row.EpisodeNumber, PartNumber: row.PartNumber,
	}
}

// chooseTarget picks what a "play" on a series, season or multi-part movie starts: the item in progress,
// else the first unwatched one after the furthest watched, else the first unwatched, else the first again.
func chooseTarget(rows []store.ListPlayableUnderRow) *playTargetDTO {
	if len(rows) == 0 {
		return nil
	}
	best := -1
	for i, row := range rows {
		if row.Watched || row.PositionSeconds < resumeMinSeconds {
			continue
		}
		if best < 0 || (row.ProgressAt != nil && (rows[best].ProgressAt == nil || row.ProgressAt.After(*rows[best].ProgressAt))) {
			best = i
		}
	}
	if best >= 0 {
		t := targetOf(rows[best])
		t.PositionSeconds, t.Resume = rows[best].PositionSeconds, true
		return &t
	}
	furthest := -1
	for i, row := range rows {
		if row.Watched {
			furthest = i
		}
	}
	for _, from := range []int{furthest + 1, 0} {
		for _, row := range rows[from:] {
			if !row.Watched {
				t := targetOf(row)
				return &t
			}
		}
	}
	t := targetOf(rows[0])
	t.Rewatch = true
	return &t
}

// nextAfter returns the item that follows id in viewing order; specials never follow regular episodes.
func nextAfter(rows []store.ListPlayableUnderRow, id uuid.UUID) *playTargetDTO {
	i := slices.IndexFunc(rows, func(r store.ListPlayableUnderRow) bool { return r.ID == id })
	if i < 0 || i+1 >= len(rows) {
		return nil
	}
	cur, next := rows[i], rows[i+1]
	if next.Kind == "episode" && ptrValue(next.SeasonNumber) == 0 && ptrValue(cur.SeasonNumber) != 0 {
		return nil
	}
	t := targetOf(next)
	return &t
}

func ptrValue[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// withProgress fills the watch state of an item detail.
func (s *Server) withProgress(r *http.Request, it store.Item, dto *itemDetailDTO) error {
	p, _ := auth.PrincipalFrom(r.Context())
	if slices.Contains(playableKinds, it.Kind) {
		pr, err := s.q.GetProgress(r.Context(), store.GetProgressParams{UserID: p.UserID, ItemID: it.ID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		dto.Watched, dto.PositionSeconds = pr.Watched, resumePoint(pr.Watched, pr.PositionSeconds)
		if it.Kind == "episode" || it.Kind == "part" {
			root := it.ID
			if len(dto.Ancestors) > 0 {
				root = dto.Ancestors[0].ID
			}
			rows, err := s.q.ListPlayableUnder(r.Context(), store.ListPlayableUnderParams{UserID: p.UserID, Root: root})
			if err != nil {
				return err
			}
			dto.Next = nextAfter(rows, it.ID)
		}
		return nil
	}
	rows, err := s.q.ListPlayableUnder(r.Context(), store.ListPlayableUnderParams{UserID: p.UserID, Root: it.ID})
	if err != nil {
		return err
	}
	dto.Watched = len(rows) > 0 && !slices.ContainsFunc(rows, func(r store.ListPlayableUnderRow) bool { return !r.Watched })
	dto.PlayTarget = chooseTarget(rows)
	return nil
}

type homeItemDTO struct {
	ID              uuid.UUID `json:"id"`
	Kind            string    `json:"kind"`
	Title           string    `json:"title"`
	ShowTitle       string    `json:"showTitle,omitempty"`
	SeasonNumber    *int32    `json:"seasonNumber,omitempty"`
	EpisodeNumber   *int32    `json:"episodeNumber,omitempty"`
	PartNumber      *int32    `json:"partNumber,omitempty"`
	PosterURL       *string   `json:"posterUrl"`
	PositionSeconds float64   `json:"positionSeconds"`
	DurationSeconds float64   `json:"durationSeconds"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (s *Server) continueWatching(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	items, err := s.localContinueWatching(r.Context(), p.UserID, p.Role == auth.RoleAdmin)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) localContinueWatching(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]homeItemDTO, error) {
	rows, err := s.q.ListContinueWatching(ctx, store.ListContinueWatchingParams{
		UserID: userID, MinSeconds: resumeMinSeconds, IsAdmin: isAdmin, Lim: homeRailSize,
	})
	if err != nil {
		return nil, err
	}
	items := make([]homeItemDTO, 0, len(rows))
	for _, row := range rows {
		poster := imageURL(row.PosterPath, "w342")
		if poster == nil {
			poster = imageURL(row.ThumbPath, "w342")
		}
		items = append(items, homeItemDTO{
			ID: row.ID, Kind: row.Kind, Title: row.Title, ShowTitle: row.ShowTitle, SeasonNumber: row.SeasonNumber,
			EpisodeNumber: row.EpisodeNumber, PartNumber: row.PartNumber, PosterURL: poster,
			PositionSeconds: row.PositionSeconds, DurationSeconds: ptrValue(row.DurationSeconds), UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

func (s *Server) nextUp(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	items, err := s.localNextUp(r.Context(), p.UserID, p.Role == auth.RoleAdmin)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) localNextUp(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]homeItemDTO, error) {
	rows, err := s.q.ListNextUp(ctx, store.ListNextUpParams{
		UserID: userID, MinSeconds: resumeMinSeconds, IsAdmin: isAdmin, Lim: homeRailSize,
	})
	if err != nil {
		return nil, err
	}
	items := make([]homeItemDTO, 0, len(rows))
	for _, row := range rows {
		poster := imageURL(row.PosterPath, "w342")
		if poster == nil {
			poster = imageURL(row.ThumbPath, "w342")
		}
		items = append(items, homeItemDTO{
			ID: row.ID, Kind: row.Kind, Title: row.Title, ShowTitle: row.ShowTitle, SeasonNumber: row.SeasonNumber,
			EpisodeNumber: row.EpisodeNumber, PartNumber: row.PartNumber, PosterURL: poster, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}
