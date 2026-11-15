package api

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"
	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

type sourcedHomeItem struct {
	homeItemDTO
	Source cardSourceDTO `json:"source"`
}

type homeResponse struct {
	ContinueWatching   []sourcedHomeItem `json:"continueWatching"`
	NextUp             []sourcedHomeItem `json:"nextUp"`
	UnavailableServers int               `json:"unavailableServers"`
	// UnavailableServerNames names the peers left out, so that clients can say which servers are missing.
	UnavailableServerNames []string `json:"unavailableServerNames"`
}

func (s *Server) localHome(ctx context.Context, userID uuid.UUID, isAdmin bool) (homeResponse, error) {
	response := homeResponse{ContinueWatching: []sourcedHomeItem{}, NextUp: []sourcedHomeItem{}, UnavailableServerNames: []string{}}
	continued, err := s.localContinueWatching(ctx, userID, isAdmin)
	if err != nil {
		return response, err
	}
	next, err := s.localNextUp(ctx, userID, isAdmin)
	if err != nil {
		return response, err
	}
	name, err := s.Settings.ServerName(ctx)
	if err != nil {
		return response, err
	}
	for _, item := range continued {
		response.ContinueWatching = append(response.ContinueWatching, sourcedHomeItem{homeItemDTO: item, Source: cardSourceDTO{ServerID: s.Identity.ID, ServerName: name, ItemID: item.ID}})
	}
	for _, item := range next {
		response.NextUp = append(response.NextUp, sourcedHomeItem{homeItemDTO: item, Source: cardSourceDTO{ServerID: s.Identity.ID, ServerName: name, ItemID: item.ID}})
	}
	return response, nil
}

func (s *Server) peerHome(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.federatedUser(w, r)
	if !ok {
		return
	}
	response, err := s.localHome(r.Context(), userID, false)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFrom(r.Context())
	response, err := s.localHome(r.Context(), principal.UserID, principal.Role == auth.RoleAdmin)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	results, unavailable, err := fanOutPeers(r.Context(), s, "home", func(ctx context.Context, peer store.Peer) (homeResponse, error) {
		return cachedPeerCall(ctx, s, s.peerHomes, peer, principal.UserID.String(), func(ctx context.Context) (homeResponse, error) {
			return s.fetchPeerHome(ctx, peer, principal.UserID)
		})
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	response.UnavailableServers, response.UnavailableServerNames = len(unavailable), unavailable
	for _, result := range results {
		response.ContinueWatching = append(response.ContinueWatching, result.ContinueWatching...)
		response.NextUp = append(response.NextUp, result.NextUp...)
	}
	order := func(left, right sourcedHomeItem) int {
		if compared := right.UpdatedAt.Compare(left.UpdatedAt); compared != 0 {
			return compared
		}
		return strings.Compare(left.Source.ServerID.String()+left.ID.String(), right.Source.ServerID.String()+right.ID.String())
	}
	slices.SortStableFunc(response.ContinueWatching, order)
	slices.SortStableFunc(response.NextUp, order)
	response.ContinueWatching = response.ContinueWatching[:min(len(response.ContinueWatching), homeRailSize)]
	response.NextUp = response.NextUp[:min(len(response.NextUp), homeRailSize)]
	httpx.JSON(w, http.StatusOK, response)
}

// fetchPeerHome reads the home rails of one peer for the user; callers go through callPeer.
func (s *Server) fetchPeerHome(ctx context.Context, peer store.Peer, userID uuid.UUID) (homeResponse, error) {
	var response homeResponse
	token, err := s.federatedTokenFor(peer.ID, userID)
	if err != nil {
		return response, err
	}
	if err := s.getPeerCatalog(ctx, peer.Url+"/api/v1/federation/home", token, &response); err != nil {
		return response, err
	}
	for _, rail := range [][]sourcedHomeItem{response.ContinueWatching, response.NextUp} {
		for index := range rail {
			item := &rail[index]
			item.Source = cardSourceDTO{ServerID: peer.ID, ServerName: peerDisplayName(peer), ServerURL: peer.Url, ItemID: item.ID}
			if item.PosterURL != nil && strings.HasPrefix(*item.PosterURL, "/") {
				absolute := peer.Url + *item.PosterURL
				item.PosterURL = &absolute
			}
		}
	}
	return response, nil
}
