package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

const (
	// maxCollectionSize bounds the titles an administrator links in one collection.
	maxCollectionSize         = 100
	collectionCandidatesLimit = 20
	minCollectionSearchLength = 2
)

// collectionMemberDTO is a title of a collection as the administrator editing it sees it.
type collectionMemberDTO struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	Year      *int32    `json:"year"`
	PosterURL *string   `json:"posterUrl"`
}

func memberDTO(id uuid.UUID, kind, title string, year *int32, poster, thumb *string) collectionMemberDTO {
	url := imageURL(poster, "w185")
	if url == nil {
		url = imageURL(thumb, "w185")
	}
	return collectionMemberDTO{ID: id, Kind: kind, Title: title, Year: year, PosterURL: url}
}

// collectionCards returns the other titles of the item's collection that the caller can read, for its page.
func (s *Server) collectionCards(ctx context.Context, it store.Item) ([]itemCardDTO, error) {
	out := []itemCardDTO{}
	if it.CollectionID == nil {
		return out, nil
	}
	p, _ := auth.PrincipalFrom(ctx)
	rows, err := s.q.ListCollectionCards(ctx, store.ListCollectionCardsParams{
		UserID: p.UserID, ItemID: it.ID, IsAdmin: p.Role == auth.RoleAdmin,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		poster := imageURL(row.PosterPath, "w342")
		if poster == nil {
			poster = imageURL(row.ThumbPath, "w342")
		}
		out = append(out, itemCardDTO{
			ID: row.ID, Kind: row.Kind, Title: row.Title, Year: row.Year, PosterURL: poster,
			Rating: row.Rating, MatchStatus: row.MatchStatus, Watched: row.Watched,
		})
	}
	return out, nil
}

// loadCollectableItem loads the movie or video named in the URL, the only titles that can join a collection.
func (s *Server) loadCollectableItem(w http.ResponseWriter, r *http.Request) (store.Item, bool) {
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
	if it.ParentID != nil || (it.Kind != "movie" && it.Kind != "video") {
		httpx.Error(w, http.StatusBadRequest, "validation", "only movies and videos can be grouped in a collection")
		return it, false
	}
	return it, true
}

// getCollection lists every other title of the item's collection, for the administrator editing it.
func (s *Server) getCollection(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadCollectableItem(w, r)
	if !ok {
		return
	}
	s.respondCollection(w, r, it.ID)
}

func (s *Server) respondCollection(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	rows, err := s.q.ListCollectionMembers(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	items := make([]collectionMemberDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, memberDTO(row.ID, row.Kind, row.Title, row.Year, row.PosterPath, row.ThumbPath))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// searchCollectionCandidates finds the movies and videos of every library that can join the item's collection.
func (s *Server) searchCollectionCandidates(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadCollectableItem(w, r)
	if !ok {
		return
	}
	raw := strings.TrimSpace(r.URL.Query().Get("query"))
	if utf8.RuneCountInString(raw) < minCollectionSearchLength {
		httpx.Error(w, http.StatusBadRequest, "validation", "query must be at least 2 characters")
		return
	}
	rows, err := s.q.SearchCollectionCandidates(r.Context(), store.SearchCollectionCandidatesParams{
		ItemID: it.ID, Query: "%" + likeEscaper.Replace(raw) + "%", Lim: collectionCandidatesLimit,
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	items := make([]collectionMemberDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, memberDTO(row.ID, row.Kind, row.Title, row.Year, row.PosterPath, row.ThumbPath))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type setCollectionRequest struct {
	ItemIDs []uuid.UUID `json:"itemIds"`
}

// setCollection makes the item and the listed titles one collection. Titles that were in the item's collection and
// are no longer listed leave it and listed titles from another collection move to this one. An empty list takes
// only the item out of its collection.
func (s *Server) setCollection(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadCollectableItem(w, r)
	if !ok {
		return
	}
	var req setCollectionRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if req.ItemIDs == nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "itemIds is required")
		return
	}
	members := []uuid.UUID{it.ID}
	for _, id := range req.ItemIDs {
		if !slices.Contains(members, id) {
			members = append(members, id)
		}
	}
	if len(members) > maxCollectionSize {
		httpx.Error(w, http.StatusBadRequest, "validation", "a collection holds at most 100 titles")
		return
	}

	ctx := r.Context()
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		n, err := q.CountCollectableItems(ctx, members)
		if err != nil {
			return err
		}
		if int(n) != len(members) {
			return errNotCollectable
		}
		if len(members) == 1 {
			// Only the item leaves: the other titles stay together.
			if err := q.JoinCollection(ctx, store.JoinCollectionParams{CollectionID: nil, Ids: members}); err != nil {
				return err
			}
			return q.DissolveSingleCollections(ctx)
		}
		collection := it.CollectionID
		if collection == nil {
			id := uuid.New()
			collection = &id
		}
		if err := q.LeaveCollection(ctx, store.LeaveCollectionParams{CollectionID: collection, Keep: members}); err != nil {
			return err
		}
		if err := q.JoinCollection(ctx, store.JoinCollectionParams{CollectionID: collection, Ids: members}); err != nil {
			return err
		}
		// Also clears the item alone in its collection, and the collections the listed titles were taken from.
		return q.DissolveSingleCollections(ctx)
	})
	if errors.Is(err, errNotCollectable) {
		httpx.Error(w, http.StatusBadRequest, "validation", "itemIds must name movies and videos")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	s.respondCollection(w, r, it.ID)
}

var errNotCollectable = errors.New("not collectable")
