package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

// watchlistLimit caps the titles listed per server, which keeps a peer answer well under maxPeerResponse.
const watchlistLimit = 100

type watchlistItemDTO struct {
	ID        uuid.UUID     `json:"id"`
	Kind      string        `json:"kind"`
	Title     string        `json:"title"`
	Year      *int32        `json:"year"`
	TMDBID    *int32        `json:"tmdbId"`
	PosterURL *string       `json:"posterUrl"`
	AddedAt   time.Time     `json:"addedAt"`
	Source    cardSourceDTO `json:"source"`
}

type watchlistResponse struct {
	Items              []watchlistItemDTO `json:"items"`
	UnavailableServers int                `json:"unavailableServers"`
	// UnavailableServerNames names the peers left out, so that clients can say which servers are missing.
	UnavailableServerNames []string `json:"unavailableServerNames"`
}

// setWatchlisted adds a title to the caller's watchlist or removes it; both are idempotent.
func (s *Server) setWatchlisted(listed bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		it, ok := s.accessibleItem(w, r)
		if !ok {
			return
		}
		if it.ParentID != nil {
			httpx.Error(w, http.StatusBadRequest, "validation", "only movies, series and videos can be added to the watchlist")
			return
		}
		p, _ := auth.PrincipalFrom(r.Context())
		var err error
		switch {
		case listed:
			err = s.q.AddToWatchlist(r.Context(), store.AddToWatchlistParams{UserID: p.UserID, ItemID: it.ID})
		case it.TmdbID != nil && !isFederatedRequest(r.Context()):
			// A local user removes the title from every server, whichever copy was added.
			err = s.removeTitleFromWatchlist(r.Context(), p.UserID, p.Role == auth.RoleAdmin, it.Kind, *it.TmdbID)
		default:
			// The copies of the title held by this server; a peer removes the others itself.
			err = s.q.RemoveFromWatchlist(r.Context(), store.RemoveFromWatchlistParams{UserID: p.UserID, ItemID: it.ID})
		}
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) localWatchlist(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]watchlistItemDTO, error) {
	rows, err := s.q.ListWatchlist(ctx, store.ListWatchlistParams{UserID: userID, IsAdmin: isAdmin, Lim: watchlistLimit})
	if err != nil {
		return nil, err
	}
	name, err := s.Settings.ServerName(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]watchlistItemDTO, 0, len(rows))
	for _, row := range rows {
		poster := imageURL(row.PosterPath, "w342")
		if poster == nil {
			poster = imageURL(row.ThumbPath, "w342")
		}
		items = append(items, watchlistItemDTO{
			ID: row.ID, Kind: row.Kind, Title: row.Title, Year: row.Year, TMDBID: row.TmdbID, PosterURL: poster, AddedAt: row.AddedAt,
			Source: cardSourceDTO{ServerID: s.Identity.ID, ServerName: name, ItemID: row.ID},
		})
	}
	return items, nil
}

// peerWatchlist lists the watchlist of a paired server's user, under this server's own access rules.
func (s *Server) peerWatchlist(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.federatedUser(w, r)
	if !ok {
		return
	}
	items, err := s.localWatchlist(r.Context(), userID, false)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, watchlistResponse{Items: items, UnavailableServerNames: []string{}})
}

// watchlist merges the caller's watchlist on this server with the ones kept for them by the paired servers.
func (s *Server) watchlist(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFrom(r.Context())
	items, err := s.localWatchlist(r.Context(), principal.UserID, principal.Role == auth.RoleAdmin)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	results, unavailable, err := fanOutPeers(r.Context(), s, "watchlist", func(ctx context.Context, peer store.Peer) ([]watchlistItemDTO, error) {
		return s.fetchPeerWatchlist(ctx, peer, principal.UserID)
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	response := watchlistResponse{Items: items, UnavailableServers: len(unavailable), UnavailableServerNames: unavailable}
	for _, result := range results {
		response.Items = append(response.Items, result...)
	}
	slices.SortStableFunc(response.Items, func(left, right watchlistItemDTO) int {
		if compared := right.AddedAt.Compare(left.AddedAt); compared != 0 {
			return compared
		}
		return strings.Compare(left.Source.ServerID.String()+left.ID.String(), right.Source.ServerID.String()+right.ID.String())
	})
	response.Items = uniqueTitles(response.Items)
	httpx.JSON(w, http.StatusOK, response)
}

func (s *Server) fetchPeerWatchlist(ctx context.Context, peer store.Peer, userID uuid.UUID) ([]watchlistItemDTO, error) {
	var response watchlistResponse
	err := s.callPeer(ctx, peer, func(ctx context.Context) error {
		token, err := s.federatedTokenFor(peer.ID, userID)
		if err != nil {
			return err
		}
		err = s.getPeerCatalog(ctx, peer.Url+"/api/v1/federation/watchlist", token, &response)
		// A peer older than the watchlist cannot hold any entry for this user.
		if errors.Is(err, errPeerNotFound) {
			response.Items = nil
			return nil
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	for index := range response.Items {
		item := &response.Items[index]
		item.Source = cardSourceDTO{ServerID: peer.ID, ServerName: peerDisplayName(peer), ServerURL: peer.Url, ItemID: item.ID}
		if item.PosterURL != nil && strings.HasPrefix(*item.PosterURL, "/") {
			absolute := peer.Url + *item.PosterURL
			item.PosterURL = &absolute
		}
	}
	return response.Items, nil
}

// uniqueTitles keeps the most recent entry of each title when several copies of it were added, on this server or on
// paired ones; the items must be sorted most recent first.
func uniqueTitles(items []watchlistItemDTO) []watchlistItemDTO {
	seen := map[string]bool{}
	out := items[:0]
	for _, item := range items {
		if item.TMDBID != nil {
			key := item.Kind + ":" + strconv.Itoa(int(*item.TMDBID))
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, item)
	}
	return out
}

// removeTitleFromWatchlist removes every copy of a title from the user's watchlist: on this server, and on each paired
// server holding one. A peer that cannot be reached keeps its entry.
func (s *Server) removeTitleFromWatchlist(ctx context.Context, userID uuid.UUID, isAdmin bool, kind string, tmdbID int32) error {
	err := s.q.RemoveTitleFromWatchlist(ctx, store.RemoveTitleFromWatchlistParams{UserID: userID, ItemKind: kind, TmdbID: tmdbID})
	if err != nil {
		return err
	}
	title, err := s.titleSources(ctx, userID, isAdmin, kind, tmdbID)
	if err != nil {
		return err
	}
	_, _, err = fanOutPeers(ctx, s, "watchlist removal", func(ctx context.Context, peer store.Peer) (struct{}, error) {
		// The peer removes every copy it holds: one of its items is enough.
		for _, source := range title.Sources {
			if source.ServerID == peer.ID && title.PeersWatchlisting[peer.ID] {
				return struct{}{}, s.removePeerWatchlist(ctx, peer, userID, source.ItemID)
			}
		}
		return struct{}{}, nil
	})
	return err
}

// removePeerWatchlist removes a title held by a paired server from the user's watchlist there.
func (s *Server) removePeerWatchlist(ctx context.Context, peer store.Peer, userID, itemID uuid.UUID) error {
	return s.callPeer(ctx, peer, func(ctx context.Context) error {
		token, err := s.federatedTokenFor(peer.ID, userID)
		if err != nil {
			return err
		}
		target := peer.Url + "/api/v1/items/" + url.PathEscape(itemID.String()) + "/watchlist"
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := s.httpClient().Do(req)
		if err != nil {
			return fmt.Errorf("%w: %w: %v", errPeerUnreachable, errPeerDown, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return peerStatusError(resp.StatusCode)
		}
		return nil
	})
}

// deleteTitleFromWatchlist removes a title from the caller's watchlist on every server, whichever copy was added.
func (s *Server) deleteTitleFromWatchlist(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	tmdb, err := strconv.ParseInt(chi.URLParam(r, "tmdbId"), 10, 32)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "tmdbId must be an integer")
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())
	if err := s.removeTitleFromWatchlist(r.Context(), p.UserID, p.Role == auth.RoleAdmin, kind, int32(tmdb)); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
