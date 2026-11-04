package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
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
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
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
	sources, err := s.titleSources(r.Context(), p.UserID, p.Role == auth.RoleAdmin, kind, int32(tmdb))
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"sources": sources})
}

// titleSources lists the servers holding the title of a top-level item, as the caller sees them; unreachable peers are skipped.
func (s *Server) titleSources(ctx context.Context, userID uuid.UUID, isAdmin bool, kind string, tmdbID int32) ([]cardSourceDTO, error) {
	name, err := s.Settings.ServerName(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListVisibleTitlesByTMDB(ctx, store.ListVisibleTitlesByTMDBParams{
		ItemKind: kind, TmdbID: &tmdbID, IsAdmin: isAdmin, UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]cardSourceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, cardSourceDTO{ServerID: s.Identity.ID, ServerName: name, ItemID: row.ID})
	}

	peers, err := s.q.ListPeers(ctx)
	if err != nil {
		return nil, err
	}
	results := make([][]cardSourceDTO, len(peers))
	errs := make([]error, len(peers))
	var wg sync.WaitGroup
	for i, p := range peers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = s.fetchPeerTitleSources(ctx, p, userID, kind, tmdbID)
		}()
	}
	wg.Wait()
	for i, p := range peers {
		if errs[i] != nil {
			slog.Warn("federated title unavailable", "peer", p.ID, "err", errs[i])
			continue
		}
		out = append(out, results[i]...)
	}
	return out, nil
}

// fetchPeerTitleSources asks one peer which of its items carry the same title, for the user.
func (s *Server) fetchPeerTitleSources(ctx context.Context, p store.Peer, userID uuid.UUID, kind string, tmdbID int32) ([]cardSourceDTO, error) {
	ctx, cancel := context.WithTimeout(ctx, peerCatalogTimeout)
	defer cancel()
	token, err := s.federatedTokenFor(p.ID, userID)
	if err != nil {
		return nil, err
	}
	target := p.Url + "/api/v1/federation/catalog/titles/" + url.PathEscape(kind) + "/" + strconv.Itoa(int(tmdbID))
	var page struct {
		Items []titleRefDTO `json:"items"`
	}
	if err := s.getPeerCatalog(ctx, target, token, &page); err != nil {
		return nil, err
	}
	out := make([]cardSourceDTO, 0, len(page.Items))
	for _, it := range page.Items {
		out = append(out, cardSourceDTO{ServerID: p.ID, ServerName: p.Name, ServerURL: p.Url, ItemID: it.ID})
	}
	return out, nil
}
