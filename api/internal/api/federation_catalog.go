package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

const (
	// allRows asks a listing for every row, so that merged listings are paged after grouping.
	allRows            = 1<<31 - 1
	peerCatalogTimeout = 15 * time.Second
)

// federatedCard is a catalogue entry with the keys a peer exports so that the origin can merge and group it.
type federatedCard struct {
	itemCardDTO
	TMDBID    *int32    `json:"tmdbId"`
	SortTitle string    `json:"sortTitle"`
	AddedAt   time.Time `json:"addedAt"`
}

// sourcedCard is a catalogue entry together with the server that holds it.
type sourcedCard struct {
	federatedCard
	source cardSourceDTO
}

type cardSourceDTO struct {
	ServerID   uuid.UUID `json:"serverId"`
	ServerName string    `json:"serverName"`
	ServerURL  string    `json:"serverUrl,omitempty"`
	ItemID     uuid.UUID `json:"itemId"`
}

// catalogCardDTO is one title of a kind listing; the versions of a same TMDB title share a card.
type catalogCardDTO struct {
	itemCardDTO
	// Sources lists the servers holding a version of the title; the first one is the primary.
	Sources []cardSourceDTO `json:"sources"`
}

// kindCards reads the top-level entries of one kind that a user can see on this server.
func (s *Server) kindCards(ctx context.Context, userID uuid.UUID, isAdmin bool, kind string, libraryID *uuid.UUID, query, sort string, offset, limit int, filters json.RawMessage) ([]federatedCard, int32, error) {
	rows, err := s.q.ListKindItems(ctx, store.ListKindItemsParams{
		UserID: userID, IsAdmin: isAdmin, Kind: kind, LibraryID: libraryID,
		Query: query, Sort: sort, Off: int32(offset), Lim: int32(limit),
		Filters: filters,
	})
	if err != nil {
		return nil, 0, err
	}
	total := int32(0)
	cards := make([]federatedCard, 0, len(rows))
	for _, row := range rows {
		total = row.Total
		poster := imageURL(row.PosterPath, "w342")
		if poster == nil {
			poster = imageURL(row.ThumbPath, "w342")
		}
		cards = append(cards, federatedCard{
			itemCardDTO: itemCardDTO{
				ID: row.ID, Kind: row.Kind, Title: row.Title, Year: row.Year, PosterURL: poster,
				Rating: row.Rating, EpisodeCount: row.EpisodeCount, MatchStatus: row.MatchStatus,
				Watched: row.Watched || (row.Kind == "series" && row.UnwatchedCount == 0), UnwatchedCount: row.UnwatchedCount,
			},
			TMDBID: row.TmdbID, SortTitle: row.SortTitle, AddedAt: row.CreatedAt,
		})
	}
	return cards, total, nil
}

// federatedUser authenticates a request made with a peer's federated token; it answers the error itself.
func (s *Server) federatedUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "federated token required")
		return uuid.UUID{}, false
	}
	claims, err := s.authenticateFederatedToken(r, token)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid federated token")
		return uuid.UUID{}, false
	}
	// Only shadow users of the issuing peer may be read on its behalf.
	user, err := s.q.GetUserByID(r.Context(), claims.Subject)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (user.OriginPeerID == nil || *user.OriginPeerID != claims.Issuer)) {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid federated token")
		return uuid.UUID{}, false
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return uuid.UUID{}, false
	}
	return claims.Subject, true
}

// peerKindItems lists one kind for a user of a paired peer, under this server's own access rules.
func (s *Server) peerKindItems(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	if !slices.Contains(libraryKinds, kind) {
		httpx.Error(w, http.StatusBadRequest, "validation", "kind must be one of: movies, series, other")
		return
	}
	userID, ok := s.federatedUser(w, r)
	if !ok {
		return
	}
	params, ok := parseItemListParams(w, r)
	if !ok {
		return
	}
	cards, total, err := s.kindCards(r.Context(), userID, false, kind, nil, params.query, params.sort, params.offset, params.limit, params.filters)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": cards, "total": total})
}

// peerKindCards reads every visible card of one kind from each paired peer; unreachable peers are counted and skipped.
func (s *Server) peerKindCards(ctx context.Context, userID uuid.UUID, kind, query, sort string, filters json.RawMessage) ([]sourcedCard, int, error) {
	peers, err := s.q.ListPeers(ctx)
	if err != nil {
		return nil, 0, err
	}
	results := make([][]sourcedCard, len(peers))
	errs := make([]error, len(peers))
	var wg sync.WaitGroup
	for i, p := range peers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = s.fetchPeerCards(ctx, p, userID, kind, query, sort, filters)
		}()
	}
	wg.Wait()
	var out []sourcedCard
	unavailable := 0
	for i, p := range peers {
		if errs[i] != nil {
			unavailable++
			slog.Warn("federated catalog unavailable", "peer", p.ID, "err", errs[i])
			continue
		}
		out = append(out, results[i]...)
	}
	return out, unavailable, nil
}

// fetchPeerCards pages through the listing of one peer, with a federated token for the user.
func (s *Server) fetchPeerCards(ctx context.Context, p store.Peer, userID uuid.UUID, kind, query, sort string, filters json.RawMessage) ([]sourcedCard, error) {
	ctx, cancel := context.WithTimeout(ctx, peerCatalogTimeout)
	defer cancel()
	if _, err := s.checkPeerVersion(ctx, p); err != nil {
		return nil, err
	}
	token, err := s.federatedTokenFor(p.ID, userID)
	if err != nil {
		return nil, err
	}
	var out []sourcedCard
	for offset := 0; ; {
		target := p.Url + "/api/v1/federation/catalog/kinds/" + url.PathEscape(kind) + "/items?" + url.Values{
			"sort":    {sort},
			"query":   {query},
			"filters": {string(filters)},
			"offset":  {strconv.Itoa(offset)},
			"limit":   {strconv.Itoa(maxPageSize)},
		}.Encode()
		var page struct {
			Items []federatedCard `json:"items"`
			Total int32           `json:"total"`
		}
		if err := s.getPeerCatalog(ctx, target, token, &page); err != nil {
			return nil, err
		}
		for _, c := range page.Items {
			// Posters of a peer are relative to its own origin; the client needs the absolute address.
			if c.PosterURL != nil && strings.HasPrefix(*c.PosterURL, "/") {
				abs := p.Url + *c.PosterURL
				c.PosterURL = &abs
			}
			out = append(out, sourcedCard{federatedCard: c, source: cardSourceDTO{
				ServerID: p.ID, ServerName: p.Name, ServerURL: p.Url, ItemID: c.ID,
			}})
		}
		offset += len(page.Items)
		if len(page.Items) == 0 || offset >= int(page.Total) {
			return out, nil
		}
	}
}

func (s *Server) getPeerCatalog(ctx context.Context, target, token string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", errPeerUnreachable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %w", errPeerUnreachable, errPeerNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", errPeerUnreachable, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxPeerResponse)).Decode(out)
}

// compareCards orders cards like the local listing query: the sort key, then the sort title and the ID.
func compareCards(a, b sourcedCard, sort string) int {
	c := 0
	switch sort {
	case "year":
		c = compareDescNullsLast(a.Year, b.Year)
	case "added":
		c = b.AddedAt.Compare(a.AddedAt)
	case "rating":
		c = compareDescNullsLast(a.Rating, b.Rating)
	}
	if c != 0 {
		return c
	}
	if c := cmp.Compare(a.SortTitle, b.SortTitle); c != 0 {
		return c
	}
	return strings.Compare(a.ID.String(), b.ID.String())
}

// compareDescNullsLast orders larger values first and missing values last.
func compareDescNullsLast[T cmp.Ordered](a, b *T) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	}
	return cmp.Compare(*b, *a)
}

// groupCards merges the versions of one TMDB title into a card; the first card in sort order is the primary.
func groupCards(cards []sourcedCard) []catalogCardDTO {
	out := []catalogCardDTO{}
	byTitle := map[string]int{}
	for _, c := range cards {
		if c.TMDBID != nil {
			key := c.Kind + ":" + strconv.Itoa(int(*c.TMDBID))
			if i, ok := byTitle[key]; ok {
				g := &out[i]
				g.Sources = append(g.Sources, c.source)
				g.Watched = g.Watched || c.Watched
				g.UnwatchedCount = min(g.UnwatchedCount, c.UnwatchedCount)
				g.EpisodeCount = max(g.EpisodeCount, c.EpisodeCount)
				continue
			}
			byTitle[key] = len(out)
		}
		out = append(out, catalogCardDTO{itemCardDTO: c.itemCardDTO, Sources: []cardSourceDTO{c.source}})
	}
	return out
}
