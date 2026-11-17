package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

type titleRefDTO struct {
	ID uuid.UUID `json:"id"`
}

// federatedTokenFor signs a short-lived token letting a paired peer read the user's items.
func (s *Server) federatedTokenFor(audience, userID uuid.UUID) (string, error) {
	now := time.Now()
	return signFederatedToken(s.Identity.Sign, federatedClaims{
		Issuer: s.Identity.ID, Subject: userID, Audience: audience,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(federatedTokenTTL).Unix(),
	})
}

// peerTitleIDs lists the top-level items of one kind and TMDB ID visible to a user of a paired peer.
func (s *Server) peerTitleIDs(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	tmdb, err := strconv.ParseInt(chi.URLParam(r, "tmdbId"), 10, 32)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "tmdbId must be an integer")
		return
	}
	userID, ok := s.federatedUser(w, r)
	if !ok {
		return
	}
	tmdbID := int32(tmdb)
	rows, err := s.q.ListVisibleTitlesByTMDB(r.Context(), store.ListVisibleTitlesByTMDBParams{
		ItemKind: kind, TmdbID: &tmdbID, IsAdmin: false, UserID: userID,
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	items := make([]titleRefDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, titleRefDTO{ID: row.ID})
	}
	watchlisted, err := s.q.IsTitleInWatchlist(r.Context(), store.IsTitleInWatchlistParams{UserID: userID, ItemKind: kind, TmdbID: tmdbID})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, peerTitleResponse{Items: items, InWatchlist: watchlisted})
}

// peerTitleResponse lists a peer's copies of a title for one of its users.
type peerTitleResponse struct {
	Items []titleRefDTO `json:"items"`
	// InWatchlist tells whether the user added one of the copies to their watchlist; absent from older peers.
	InWatchlist bool `json:"inWatchlist"`
}

// titleSourcesResult describes a title across this server and its paired servers, for one user.
type titleSourcesResult struct {
	Sources []cardSourceDTO
	// InWatchlist is set when the user added any copy of the title to their watchlist, on any server.
	InWatchlist bool
	// PeersWatchlisting lists the paired servers on which the user added a copy.
	PeersWatchlisting map[uuid.UUID]bool
}

// listTitleSources lists the servers holding a TMDB title, as the authenticated user sees them.
func (s *Server) listTitleSources(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	tmdb, err := strconv.ParseInt(chi.URLParam(r, "tmdbId"), 10, 32)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "tmdbId must be an integer")
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())
	title, err := s.titleSources(r.Context(), p.UserID, p.Role == auth.RoleAdmin, kind, int32(tmdb))
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"sources": title.Sources, "inWatchlist": title.InWatchlist})
}

// titleSources lists the servers holding the title of a top-level item, as the caller sees them; unreachable peers are skipped.
func (s *Server) titleSources(ctx context.Context, userID uuid.UUID, isAdmin bool, kind string, tmdbID int32) (titleSourcesResult, error) {
	var out titleSourcesResult
	name, err := s.Settings.ServerName(ctx)
	if err != nil {
		return out, err
	}
	rows, err := s.q.ListVisibleTitlesByTMDB(ctx, store.ListVisibleTitlesByTMDBParams{
		ItemKind: kind, TmdbID: &tmdbID, IsAdmin: isAdmin, UserID: userID,
	})
	if err != nil {
		return out, err
	}
	out.Sources = make([]cardSourceDTO, 0, len(rows))
	for _, row := range rows {
		out.Sources = append(out.Sources, cardSourceDTO{ServerID: s.Identity.ID, ServerName: name, ItemID: row.ID})
	}
	if out.InWatchlist, err = s.q.IsTitleInWatchlist(ctx, store.IsTitleInWatchlistParams{UserID: userID, ItemKind: kind, TmdbID: tmdbID}); err != nil {
		return out, err
	}

	results, _, err := fanOutPeers(ctx, s, "title", func(ctx context.Context, p store.Peer) (peerTitle, error) {
		return s.fetchPeerTitleSources(ctx, p, userID, kind, tmdbID)
	})
	if err != nil {
		return out, err
	}
	out.PeersWatchlisting = map[uuid.UUID]bool{}
	for _, result := range results {
		out.Sources = append(out.Sources, result.sources...)
		if result.inWatchlist && len(result.sources) > 0 {
			out.InWatchlist = true
			out.PeersWatchlisting[result.sources[0].ServerID] = true
		}
	}
	return out, nil
}

// peerTitle is what one paired server holds of a title for the user.
type peerTitle struct {
	sources     []cardSourceDTO
	inWatchlist bool
}

// fetchPeerTitleSources asks one peer which of its items carry the same title, for the user.
func (s *Server) fetchPeerTitleSources(ctx context.Context, p store.Peer, userID uuid.UUID, kind string, tmdbID int32) (peerTitle, error) {
	var page peerTitleResponse
	err := s.callPeer(ctx, p, func(ctx context.Context) error {
		token, err := s.federatedTokenFor(p.ID, userID)
		if err != nil {
			return err
		}
		target := p.Url + "/api/v1/federation/catalog/titles/" + url.PathEscape(kind) + "/" + strconv.Itoa(int(tmdbID))
		return s.getPeerCatalog(ctx, target, token, &page)
	})
	if err != nil {
		return peerTitle{}, err
	}
	out := peerTitle{sources: make([]cardSourceDTO, 0, len(page.Items)), inWatchlist: page.InWatchlist}
	for _, it := range page.Items {
		out.sources = append(out.sources, cardSourceDTO{ServerID: p.ID, ServerName: peerDisplayName(p), ServerURL: p.Url, ItemID: it.ID})
	}
	return out, nil
}
