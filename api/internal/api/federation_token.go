package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
)

const (
	federatedTokenDomain = "plys-federated-token-v1:"
	federatedTokenTTL    = 5 * time.Minute
)

var errInvalidFederatedToken = errors.New("invalid federated token")

// federatedClaims identifies a user of the issuing server (Issuer) for one peer (Audience).
type federatedClaims struct {
	Issuer    uuid.UUID `json:"iss"`
	Subject   uuid.UUID `json:"sub"`
	Audience  uuid.UUID `json:"aud"`
	IssuedAt  int64     `json:"iat"`
	ExpiresAt int64     `json:"exp"`
}

// signFederatedToken returns base64url(claims) + "." + base64url(signature).
func signFederatedToken(sign func([]byte) []byte, c federatedClaims) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	sig := sign(append([]byte(federatedTokenDomain), payload...))
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// verifyFederatedToken checks the signature against issuerKey and that the token is addressed to self and not expired.
func verifyFederatedToken(token string, issuerKey ed25519.PublicKey, self uuid.UUID, now time.Time) (federatedClaims, error) {
	payloadPart, sigPart, ok := strings.Cut(token, ".")
	if !ok || len(issuerKey) != ed25519.PublicKeySize {
		return federatedClaims{}, errInvalidFederatedToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return federatedClaims{}, errInvalidFederatedToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil || !ed25519.Verify(issuerKey, append([]byte(federatedTokenDomain), payload...), sig) {
		return federatedClaims{}, errInvalidFederatedToken
	}
	var c federatedClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return federatedClaims{}, errInvalidFederatedToken
	}
	if c.Audience != self || now.Unix() >= c.ExpiresAt {
		return federatedClaims{}, errInvalidFederatedToken
	}
	return c, nil
}

// authenticateFederatedToken resolves the issuer from the paired peers, then verifies the token with its key.
func (s *Server) authenticateFederatedToken(r *http.Request, token string) (federatedClaims, error) {
	payloadPart, _, ok := strings.Cut(token, ".")
	if !ok {
		return federatedClaims{}, errInvalidFederatedToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return federatedClaims{}, errInvalidFederatedToken
	}
	// The issuer is read unverified only to pick the key; the signature is checked right after.
	var unverified federatedClaims
	if err := json.Unmarshal(payload, &unverified); err != nil {
		return federatedClaims{}, errInvalidFederatedToken
	}
	peer, err := s.q.GetPeer(r.Context(), unverified.Issuer)
	if errors.Is(err, pgx.ErrNoRows) {
		return federatedClaims{}, errInvalidFederatedToken
	}
	if err != nil {
		return federatedClaims{}, err
	}
	claims, err := verifyFederatedToken(token, peer.PublicKey, s.Identity.ID, time.Now())
	if err != nil {
		return federatedClaims{}, err
	}
	if _, err := s.checkPeerVersion(r.Context(), peer); err != nil {
		return federatedClaims{}, err
	}
	return claims, nil
}

// federationToken issues a short-lived token for the logged-in user, usable only on the given paired peer.
func (s *Server) federationToken(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	var req struct {
		Audience uuid.UUID `json:"audience"`
	}
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	peer, err := s.q.GetPeer(r.Context(), req.Audience)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "peer not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if _, err := s.checkPeerVersion(r.Context(), peer); err != nil {
		httpx.Error(w, http.StatusConflict, "peer_incompatible", "the peer must be reachable and use the same major server version")
		return
	}
	now := time.Now()
	exp := now.Add(federatedTokenTTL)
	token, err := signFederatedToken(s.Identity.Sign, federatedClaims{
		Issuer: s.Identity.ID, Subject: p.UserID, Audience: req.Audience,
		IssuedAt: now.Unix(), ExpiresAt: exp.Unix(),
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"token": token, "expiresAt": exp})
}
