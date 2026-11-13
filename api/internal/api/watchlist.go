package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

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
		if listed {
			err = s.q.AddToWatchlist(r.Context(), store.AddToWatchlistParams{UserID: p.UserID, ItemID: it.ID})
		} else {
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
			ID: row.ID, Kind: row.Kind, Title: row.Title, Year: row.Year, PosterURL: poster, AddedAt: row.AddedAt,
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
		item.Source = cardSourceDTO{ServerID: peer.ID, ServerName: peer.Name, ServerURL: peer.Url, ItemID: item.ID}
		if item.PosterURL != nil && strings.HasPrefix(*item.PosterURL, "/") {
			absolute := peer.Url + *item.PosterURL
			item.PosterURL = &absolute
		}
	}
	return response.Items, nil
}
