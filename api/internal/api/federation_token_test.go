package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

// pairedEnv returns a server that has paired with a fake peer and has a complete identity.
func pairedEnv(t *testing.T) (testEnv, *fakePeer, string) {
	t.Helper()
	peer := newFakePeer(t)
	env := newTestEnvWith(t, func(d *Deps) {
		d.FederationRequireHTTPS = true
		d.HTTPClient = peer.srv.Client()
	})
	admin := setupAdmin(t, env.h).body["accessToken"].(string)
	if r := do(t, env.h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"serverName": "Local", "publicUrl": "https://local.example.org"}); r.status != http.StatusOK {
		t.Fatalf("set identity: %d", r.status)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/admin/federation/peers", admin, map[string]any{"code": pairingCodeFor(t, peer.srv.URL, peer.pub)}); r.status != http.StatusCreated {
		t.Fatalf("pair: %d", r.status)
	}
	return env, peer, admin
}

func forgeFederatedToken(priv ed25519.PrivateKey, c federatedClaims) string {
	token, _ := signFederatedToken(func(msg []byte) []byte { return ed25519.Sign(priv, msg) }, c)
	return token
}

func TestFederationTokenIssuance(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	if r := do(t, env.h, http.MethodPost, "/api/v1/me/federation-token", "", map[string]any{"audience": peer.id}); r.status != http.StatusUnauthorized {
		t.Fatalf("without login: expected 401, got %d", r.status)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/me/federation-token", admin, map[string]any{"audience": uuid.New()}); r.status != http.StatusNotFound {
		t.Fatalf("unknown peer: expected 404, got %d", r.status)
	}
	r := do(t, env.h, http.MethodPost, "/api/v1/me/federation-token", admin, map[string]any{"audience": peer.id})
	token, _ := r.body["token"].(string)
	if r.status != http.StatusOK || token == "" {
		t.Fatalf("issue: %d %v", r.status, r.body)
	}

	now := time.Now()
	pub := env.srv.Identity.PublicKey
	claims, err := verifyFederatedToken(token, pub, peer.id, now)
	if err != nil {
		t.Fatalf("peer rejected a fresh token: %v", err)
	}
	if claims.Issuer != env.srv.Identity.ID || claims.Audience != peer.id || claims.Subject == uuid.Nil {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if _, err := verifyFederatedToken(token, pub, uuid.New(), now); err == nil {
		t.Fatal("token accepted by a server it was not issued for")
	}
	if _, err := verifyFederatedToken(token, pub, peer.id, now.Add(federatedTokenTTL+time.Second)); err == nil {
		t.Fatal("expired token accepted")
	}
	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyFederatedToken(token, otherPub, peer.id, now); err == nil {
		t.Fatal("token accepted with the wrong issuer key")
	}
}

func TestFederatedTokenFromPairedPeer(t *testing.T) {
	env, peer, _ := pairedEnv(t)
	subject := uuid.New()
	now := time.Now()
	valid := federatedClaims{Issuer: peer.id, Subject: subject, Audience: env.srv.Identity.ID, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	claims, err := env.srv.authenticateFederatedToken(req, forgeFederatedToken(peer.priv, valid))
	if err != nil || claims.Subject != subject {
		t.Fatalf("paired peer token rejected: %v", err)
	}

	unknown := valid
	unknown.Issuer = uuid.New()
	if _, err := env.srv.authenticateFederatedToken(req, forgeFederatedToken(peer.priv, unknown)); err == nil {
		t.Fatal("token from an unpaired issuer accepted")
	}

	wrongAudience := valid
	wrongAudience.Audience = uuid.New()
	if _, err := env.srv.authenticateFederatedToken(req, forgeFederatedToken(peer.priv, wrongAudience)); err == nil {
		t.Fatal("token addressed to another server accepted")
	}
}
