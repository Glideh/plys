package api

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"toto/api/internal/httpx"
	"toto/api/internal/settings"
	"toto/api/internal/store"
)

const (
	handshakeDomain = "plys-handshake-v1:"
	peerTimeout     = 10 * time.Second
	maxPeerResponse = 64 << 10
)

var (
	errPeerUnreachable = errors.New("peer unreachable")
	// errPeerNotFound reports a 404 from a peer, such as an endpoint its version does not have yet.
	errPeerNotFound = errors.New("peer answered not found")
	errPeerRejected = errors.New("peer rejected the pairing")
)

// pairingCode is exchanged between admins out of band and contains only public information.
type pairingCode struct {
	URL       string `json:"url"`
	Name      string `json:"name"`
	PublicKey []byte `json:"publicKey"`
}

func (s *Server) pairingCodeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name, err := s.Settings.ServerName(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	publicURL, err := s.Settings.ServerPublicURL(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if name == "" || publicURL == "" {
		httpx.Error(w, http.StatusConflict, "identity_incomplete", "set the server name and public URL before pairing")
		return
	}
	raw, err := json.Marshal(pairingCode{URL: publicURL, Name: name, PublicKey: []byte(s.Identity.PublicKey)})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"code": base64.RawURLEncoding.EncodeToString(raw)})
}

type addPeerRequest struct {
	Code string `json:"code"`
}

type peerDTO struct {
	ID               uuid.UUID  `json:"id"`
	URL              string     `json:"url"`
	Name             string     `json:"name"`
	Version          *string    `json:"version"`
	LocalVersion     string     `json:"localVersion"`
	Status           string     `json:"status"`
	VersionCheckedAt *time.Time `json:"versionCheckedAt"`
}

func peerToDTO(p store.Peer) peerDTO {
	status := "unverified"
	if p.Version != nil && federationVersionPattern.MatchString(*p.Version) && federationVersionPattern.MatchString(Version) {
		status = "incompatible"
		if compatiblePeerVersions(Version, *p.Version) {
			status = "compatible"
		}
	}
	return peerDTO{ID: p.ID, URL: p.Url, Name: p.Name, Version: p.Version, LocalVersion: Version, Status: status, VersionCheckedAt: p.VersionCheckedAt}
}

func (s *Server) addPeer(w http.ResponseWriter, r *http.Request) {
	var req addPeerRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	ctx := r.Context()
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(req.Code))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_pairing_code", "pairing code is not valid")
		return
	}
	var code pairingCode
	if err := json.Unmarshal(raw, &code); err != nil || !settings.ValidServerName(code.Name) || len(code.PublicKey) != ed25519.PublicKeySize {
		httpx.Error(w, http.StatusBadRequest, "invalid_pairing_code", "pairing code is not valid")
		return
	}
	peerURL, ok := settings.NormalizePublicURL(code.URL)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_pairing_code", "pairing code is not valid")
		return
	}
	if s.FederationRequireHTTPS && !strings.HasPrefix(peerURL, "https://") {
		httpx.Error(w, http.StatusBadRequest, "peer_requires_https", "the peer must be served over HTTPS")
		return
	}
	ownURL, err := s.Settings.ServerPublicURL(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if peerURL == ownURL {
		httpx.Error(w, http.StatusBadRequest, "self_pairing", "this code belongs to this server")
		return
	}

	id, err := s.verifyPeer(ctx, peerURL, code.PublicKey)
	switch {
	case errors.Is(err, errPeerRejected):
		httpx.Error(w, http.StatusBadRequest, "peer_rejected", "the peer did not prove the key from the pairing code")
		return
	case errors.Is(err, errPeerUnreachable):
		httpx.Error(w, http.StatusBadGateway, "peer_unreachable", "the peer could not be reached")
		return
	case err != nil:
		httpx.InternalError(w, r, err)
		return
	}

	p, err := s.q.InsertPeer(ctx, store.InsertPeerParams{ID: id, Url: peerURL, Name: code.Name, PublicKey: code.PublicKey})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		httpx.Error(w, http.StatusConflict, "peer_exists", "this server is already paired")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	p, _ = s.checkPeerVersion(ctx, p)
	if _, err := s.syncPeerUsers(ctx, p); err != nil {
		logPeerSyncFailure(ctx, p, err)
	}
	httpx.JSON(w, http.StatusCreated, peerToDTO(p))
}

// verifyPeer checks that the URL answers with the key from the pairing code, then that it can sign a fresh nonce with it.
func (s *Server) verifyPeer(ctx context.Context, peerURL string, key []byte) (uuid.UUID, error) {
	var info struct {
		ID        uuid.UUID `json:"id"`
		PublicKey []byte    `json:"publicKey"`
	}
	if err := s.fetchPeerJSON(ctx, http.MethodGet, peerURL+"/api/v1/server", nil, &info); err != nil {
		return uuid.Nil, err
	}
	if !bytes.Equal(info.PublicKey, key) {
		return uuid.Nil, errPeerRejected
	}

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return uuid.Nil, err
	}
	var hs struct {
		Signature []byte `json:"signature"`
	}
	body := map[string][]byte{"nonce": nonce}
	if err := s.fetchPeerJSON(ctx, http.MethodPost, peerURL+"/api/v1/federation/handshake", body, &hs); err != nil {
		return uuid.Nil, err
	}
	if !ed25519.Verify(key, handshakeMessage(nonce), hs.Signature) {
		return uuid.Nil, errPeerRejected
	}
	return info.ID, nil
}

func (s *Server) fetchPeerJSON(ctx context.Context, method, target string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	ctx, cancel := context.WithTimeout(ctx, peerTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	s.signPeerRequest(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", errPeerUnreachable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", errPeerUnreachable, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxPeerResponse)).Decode(out)
}

func (s *Server) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: peerTimeout}
}

func handshakeMessage(nonce []byte) []byte {
	return append([]byte(handshakeDomain), nonce...)
}

// handshake lets a peer prove possession of this server's key; it only signs nonces with a fixed domain prefix.
func (s *Server) handshake(w http.ResponseWriter, r *http.Request) {
	if !s.handshakes.allow(clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many handshake attempts")
		return
	}
	var req struct {
		Nonce []byte `json:"nonce"`
	}
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if len(req.Nonce) < 16 || len(req.Nonce) > 64 {
		httpx.Error(w, http.StatusBadRequest, "validation", "nonce must be 16 to 64 bytes")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string][]byte{"signature": s.Identity.Sign(handshakeMessage(req.Nonce))})
}

func (s *Server) listPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := s.q.ListPeers(r.Context())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	out := make([]peerDTO, 0, len(peers))
	for _, p := range peers {
		checked, checkErr := s.checkPeerVersion(r.Context(), p)
		dto := peerToDTO(checked)
		if checkErr != nil && !errors.Is(checkErr, errPeerIncompatible) {
			dto.Status = "offline"
		}
		out = append(out, dto)
	}
	httpx.JSON(w, http.StatusOK, out)
}

type updatePeerRequest struct {
	URL string `json:"url"`
}

// updatePeer re-verifies the peer at the new URL, so a URL can only point to the same server identity and key.
func (s *Server) updatePeer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "peer not found")
		return
	}
	var req updatePeerRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	ctx := r.Context()
	peerURL, ok := settings.NormalizePublicURL(req.URL)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "validation", "url must be an absolute http or https URL")
		return
	}
	if s.FederationRequireHTTPS && !strings.HasPrefix(peerURL, "https://") {
		httpx.Error(w, http.StatusBadRequest, "peer_requires_https", "the peer must be served over HTTPS")
		return
	}
	ownURL, err := s.Settings.ServerPublicURL(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if peerURL == ownURL {
		httpx.Error(w, http.StatusBadRequest, "self_pairing", "this URL belongs to this server")
		return
	}

	p, err := s.q.GetPeer(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "peer not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}

	peerID, err := s.verifyPeer(ctx, peerURL, p.PublicKey)
	switch {
	case errors.Is(err, errPeerRejected), err == nil && peerID != p.ID:
		httpx.Error(w, http.StatusBadRequest, "peer_rejected", "the peer did not prove the key from the pairing code")
		return
	case errors.Is(err, errPeerUnreachable):
		httpx.Error(w, http.StatusBadGateway, "peer_unreachable", "the peer could not be reached")
		return
	case err != nil:
		httpx.InternalError(w, r, err)
		return
	}

	updated, err := s.q.UpdatePeerURL(ctx, store.UpdatePeerURLParams{ID: id, Url: peerURL})
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		httpx.Error(w, http.StatusNotFound, "not_found", "peer not found")
		return
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		httpx.Error(w, http.StatusConflict, "peer_url_taken", "another paired server already uses this URL")
		return
	case err != nil:
		httpx.InternalError(w, r, err)
		return
	}
	updated, _ = s.checkPeerVersion(ctx, updated)
	httpx.JSON(w, http.StatusOK, peerToDTO(updated))
}

func (s *Server) deletePeer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "peer not found")
		return
	}
	n, err := s.q.DeletePeer(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if n == 0 {
		httpx.Error(w, http.StatusNotFound, "not_found", "peer not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
