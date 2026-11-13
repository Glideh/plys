package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

const (
	requestDomain   = "plys-request-v1:"
	requestMaxSkew  = 5 * time.Minute
	headerPeerID    = "X-Plys-Server"
	headerTimestamp = "X-Plys-Timestamp"
	headerSignature = "X-Plys-Signature"
	// defaultPeerUserSyncInterval throttles user syncs triggered by listing users.
	defaultPeerUserSyncInterval = time.Minute
	peerUserSyncTimeout         = 10 * time.Second
)

var (
	errInvalidPeerRequest = errors.New("invalid peer request")
	errInvalidPeerData    = errors.New("invalid peer data")
	errRemoteUser         = errors.New("remote user")
)

// peerRequestMessage binds a signature to one method, path and timestamp.
func peerRequestMessage(method, path, timestamp string) []byte {
	return []byte(requestDomain + method + "\n" + path + "\n" + timestamp)
}

// signPeerRequest identifies this server to a peer. Every server-to-server call goes through it.
func (s *Server) signPeerRequest(req *http.Request) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := s.Identity.Sign(peerRequestMessage(req.Method, req.URL.Path, ts))
	req.Header.Set(headerPeerID, s.Identity.ID.String())
	req.Header.Set(headerTimestamp, ts)
	req.Header.Set(headerSignature, base64.RawURLEncoding.EncodeToString(sig))
}

// authenticatePeer returns the paired peer that signed the request.
// Replays inside the time window are accepted: the endpoints it protects are read-only.
func (s *Server) authenticatePeer(r *http.Request) (store.Peer, error) {
	peerID, err := uuid.Parse(r.Header.Get(headerPeerID))
	if err != nil {
		return store.Peer{}, errInvalidPeerRequest
	}
	ts := r.Header.Get(headerTimestamp)
	unix, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || time.Since(time.Unix(unix, 0)).Abs() > requestMaxSkew {
		return store.Peer{}, errInvalidPeerRequest
	}
	sig, err := base64.RawURLEncoding.DecodeString(r.Header.Get(headerSignature))
	if err != nil {
		return store.Peer{}, errInvalidPeerRequest
	}
	peer, err := s.q.GetPeer(r.Context(), peerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Peer{}, errInvalidPeerRequest
	}
	if err != nil {
		return store.Peer{}, err
	}
	if !ed25519.Verify(peer.PublicKey, peerRequestMessage(r.Method, r.URL.Path, ts), sig) {
		return store.Peer{}, errInvalidPeerRequest
	}
	// A signed request proves the peer is up again, without waiting for the monitor.
	s.markPeerSeen(r.Context(), peer)
	if _, err := s.checkPeerVersion(r.Context(), peer); err != nil {
		return store.Peer{}, errInvalidPeerRequest
	}
	return peer, nil
}

type peerUserDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
}

// peerUsers lists this server's own users for a paired peer. Shadow users are never re-exported.
func (s *Server) peerUsers(w http.ResponseWriter, r *http.Request) {
	_, err := s.authenticatePeer(r)
	if errors.Is(err, errInvalidPeerRequest) {
		httpx.Error(w, http.StatusUnauthorized, "invalid_signature", "invalid peer signature")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	users, err := s.q.ListLocalUsers(r.Context())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	out := make([]peerUserDTO, 0, len(users))
	for _, u := range users {
		out = append(out, peerUserDTO{ID: u.ID, Username: u.Username})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

// syncPeerUsers mirrors the users of a paired peer as shadow rows and removes those the peer no longer lists.
// It also works as a manual check: an offline peer that answers is back online.
func (s *Server) syncPeerUsers(ctx context.Context, peer store.Peer) (int, error) {
	var list struct {
		Items []peerUserDTO `json:"items"`
	}
	err := func() error {
		if _, err := s.checkPeerVersion(ctx, peer); err != nil {
			return err
		}
		return s.fetchPeerJSON(ctx, http.MethodGet, peer.Url+"/api/v1/federation/users", nil, &list)
	}()
	s.recordPeerResult(ctx, peer, err)
	if err != nil {
		return 0, err
	}
	seen := make(map[uuid.UUID]bool, len(list.Items))
	for _, u := range list.Items {
		if u.ID == uuid.Nil || validateUsername(u.Username) != nil {
			return 0, errInvalidPeerData
		}
		seen[u.ID] = true
	}

	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		// Stale rows go first: a remaining user may have taken a username freed by a removed one.
		existing, err := q.ListRemoteUserIDs(ctx, &peer.ID)
		if err != nil {
			return err
		}
		for _, id := range existing {
			if !seen[id] {
				if err := q.DeleteUser(ctx, id); err != nil {
					return err
				}
			}
		}
		for _, u := range list.Items {
			if err := q.UpsertRemoteUser(ctx, store.UpsertRemoteUserParams{ID: u.ID, Username: u.Username, OriginPeerID: &peer.ID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(list.Items), nil
}

// refreshStalePeerUsers claims the peers due for a user sync and re-reads them in the background.
// It reports whether a sync was started.
func (s *Server) refreshStalePeerUsers(ctx context.Context) bool {
	peers, err := s.q.ListPeers(ctx)
	if err != nil {
		slog.Error("list peers for user sync", "err", err)
		return false
	}
	interval := s.PeerUserSyncInterval
	if interval == 0 {
		interval = defaultPeerUserSyncInterval
	}
	var due []store.Peer
	for _, p := range peers {
		if peerSkipped(p) {
			continue
		}
		now := time.Now()
		cutoff := now.Add(-interval)
		claimed, err := s.q.ClaimPeerUserSync(ctx, store.ClaimPeerUserSyncParams{
			AttemptedAt: &now,
			ID:          p.ID,
			Cutoff:      &cutoff,
		})
		if err != nil {
			slog.Error("claim peer user sync", "err", err)
			continue
		}
		if claimed > 0 {
			due = append(due, p)
		}
	}
	if len(due) == 0 {
		return false
	}
	bg := context.WithoutCancel(ctx)
	go func() {
		for _, p := range due {
			syncCtx, cancel := context.WithTimeout(bg, peerUserSyncTimeout)
			_, err := s.syncPeerUsers(syncCtx, p)
			cancel()
			if err != nil {
				logPeerSyncFailure(bg, p, err)
			}
		}
	}()
	return true
}

// syncPeer re-reads the users of one paired peer.
func (s *Server) syncPeer(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	peer, err := s.q.GetPeer(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "peer not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	n, err := s.syncPeerUsers(ctx, peer)
	if errors.Is(err, errPeerIncompatible) {
		httpx.Error(w, http.StatusConflict, "peer_incompatible", "the peer must use the same major server version")
		return
	}
	if errors.Is(err, errPeerNotPaired) {
		httpx.Error(w, http.StatusBadGateway, "peer_not_paired", "the peer does not recognize this server: pair this server on the peer too")
		return
	}
	if errors.Is(err, errPeerUnreachable) || errors.Is(err, errInvalidPeerData) {
		httpx.Error(w, http.StatusBadGateway, "peer_unreachable", "the peer did not return a valid user list")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"users": n})
}

// logPeerSyncFailure keeps pairing successful when the first user sync fails; the admin can retry.
func logPeerSyncFailure(ctx context.Context, peer store.Peer, err error) {
	slog.WarnContext(ctx, "peer user sync failed", "peer_id", peer.ID, "error", err)
}
