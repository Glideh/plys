package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// fakePeer serves /server and the handshake; sign can be replaced between requests to simulate a forged signature.
type fakePeer struct {
	srv     *httptest.Server
	id      uuid.UUID
	pub     ed25519.PublicKey
	priv    ed25519.PrivateKey
	sign    func(msg []byte) []byte
	handler http.Handler
	// users is returned by GET /api/v1/federation/users; tests change it to simulate users added or removed.
	users []peerUserDTO
}

func newFakePeer(t *testing.T) *fakePeer {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakePeer{id: uuid.New(), pub: pub, priv: priv}
	p.sign = func(msg []byte) []byte { return ed25519.Sign(priv, msg) }
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/server", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": p.id, "name": "Peer", "publicKey": p.pub})
	})
	mux.HandleFunc("POST /api/v1/federation/handshake", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Nonce []byte `json:"nonce"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(map[string]any{"signature": p.sign(handshakeMessage(req.Nonce))})
	})
	mux.HandleFunc("GET /api/v1/federation/users", func(w http.ResponseWriter, _ *http.Request) {
		users := p.users
		if users == nil {
			users = []peerUserDTO{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": users})
	})
	p.handler = mux
	p.srv = httptest.NewTLSServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

func pairingCodeFor(t *testing.T, url string, key []byte) string {
	t.Helper()
	raw, err := json.Marshal(pairingCode{URL: url, Name: "Peer", PublicKey: key})
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func TestPairingWithPeer(t *testing.T) {
	peer := newFakePeer(t)
	env := newTestEnvWith(t, func(d *Deps) {
		d.FederationRequireHTTPS = true
		d.HTTPClient = peer.srv.Client()
	})
	h := env.h
	admin := setupAdmin(t, h).body["accessToken"].(string)
	const pairPath = "/api/v1/admin/federation/peers"

	if r := do(t, h, http.MethodGet, "/api/v1/admin/federation/pairing-code", admin, nil); r.status != http.StatusConflict {
		t.Fatalf("pairing code without identity: expected 409, got %d", r.status)
	}
	if r := do(t, h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"serverName": "Local", "publicUrl": "https://local.example.org"}); r.status != http.StatusOK {
		t.Fatalf("set identity: %d", r.status)
	}
	if r := do(t, h, http.MethodGet, "/api/v1/admin/federation/pairing-code", admin, nil); r.status != http.StatusOK || r.body["code"] == "" {
		t.Fatalf("pairing code: %d %v", r.status, r.body)
	}

	if r := do(t, h, http.MethodPost, pairPath, admin, map[string]any{"code": pairingCodeFor(t, "https://local.example.org", peer.pub)}); r.status != http.StatusBadRequest {
		t.Fatalf("own URL: expected 400, got %d", r.status)
	}
	if r := do(t, h, http.MethodPost, pairPath, admin, map[string]any{"code": pairingCodeFor(t, "http://peer.example.org", peer.pub)}); r.status != http.StatusBadRequest {
		t.Fatalf("plain HTTP with HTTPS required: expected 400, got %d", r.status)
	}
	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if r := do(t, h, http.MethodPost, pairPath, admin, map[string]any{"code": pairingCodeFor(t, peer.srv.URL, other)}); r.status != http.StatusBadRequest {
		t.Fatalf("key mismatch: expected 400, got %d", r.status)
	}
	peer.sign = func([]byte) []byte { return make([]byte, ed25519.SignatureSize) }
	if r := do(t, h, http.MethodPost, pairPath, admin, map[string]any{"code": pairingCodeFor(t, peer.srv.URL, peer.pub)}); r.status != http.StatusBadRequest {
		t.Fatalf("forged signature: expected 400, got %d", r.status)
	}
	if r := do(t, h, http.MethodPost, pairPath, admin, map[string]any{"code": "not-a-code"}); r.status != http.StatusBadRequest {
		t.Fatalf("invalid code: expected 400, got %d", r.status)
	}

	peer.sign = func(msg []byte) []byte { return ed25519.Sign(peer.priv, msg) }
	r := do(t, h, http.MethodPost, pairPath, admin, map[string]any{"code": pairingCodeFor(t, peer.srv.URL, peer.pub)})
	if r.status != http.StatusCreated || r.body["id"] != peer.id.String() {
		t.Fatalf("pair with valid peer: %d %v", r.status, r.body)
	}
	if r := do(t, h, http.MethodPost, pairPath, admin, map[string]any{"code": pairingCodeFor(t, peer.srv.URL, peer.pub)}); r.status != http.StatusConflict {
		t.Fatalf("second pairing: expected 409, got %d", r.status)
	}
	if r := do(t, h, http.MethodGet, pairPath, admin, nil); r.status != http.StatusOK {
		t.Fatalf("list peers: %d", r.status)
	}
	if r := do(t, h, http.MethodDelete, pairPath+"/"+peer.id.String(), admin, nil); r.status != http.StatusNoContent {
		t.Fatalf("delete peer: %d", r.status)
	}
	if r := do(t, h, http.MethodDelete, pairPath+"/"+peer.id.String(), admin, nil); r.status != http.StatusNotFound {
		t.Fatalf("delete missing peer: expected 404, got %d", r.status)
	}
}

func TestUpdatePeerURL(t *testing.T) {
	peer := newFakePeer(t)
	moved := httptest.NewTLSServer(peer.handler)
	t.Cleanup(moved.Close)
	env := newTestEnvWith(t, func(d *Deps) {
		d.FederationRequireHTTPS = true
		d.HTTPClient = peer.srv.Client()
	})
	h := env.h
	admin := setupAdmin(t, h).body["accessToken"].(string)
	const pairPath = "/api/v1/admin/federation/peers"
	if r := do(t, h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"serverName": "Local", "publicUrl": "https://local.example.org"}); r.status != http.StatusOK {
		t.Fatalf("set identity: %d", r.status)
	}
	if r := do(t, h, http.MethodPost, pairPath, admin, map[string]any{"code": pairingCodeFor(t, peer.srv.URL, peer.pub)}); r.status != http.StatusCreated {
		t.Fatalf("pair: %d", r.status)
	}
	patchPath := pairPath + "/" + peer.id.String()

	if r := do(t, h, http.MethodPatch, pairPath+"/"+uuid.NewString(), admin, map[string]any{"url": moved.URL}); r.status != http.StatusNotFound {
		t.Fatalf("unknown peer: expected 404, got %d", r.status)
	}
	if r := do(t, h, http.MethodPatch, patchPath, admin, map[string]any{"url": "ftp://peer.example.org"}); r.status != http.StatusBadRequest {
		t.Fatalf("invalid URL: expected 400, got %d", r.status)
	}
	if r := do(t, h, http.MethodPatch, patchPath, admin, map[string]any{"url": "http://peer.example.org"}); r.status != http.StatusBadRequest {
		t.Fatalf("plain HTTP with HTTPS required: expected 400, got %d", r.status)
	}
	if r := do(t, h, http.MethodPatch, patchPath, admin, map[string]any{"url": "https://local.example.org/"}); r.status != http.StatusBadRequest {
		t.Fatalf("own URL: expected 400, got %d", r.status)
	}
	other := newFakePeer(t)
	if r := do(t, h, http.MethodPatch, patchPath, admin, map[string]any{"url": other.srv.URL}); r.status != http.StatusBadRequest {
		t.Fatalf("different identity at new URL: expected 400, got %d", r.status)
	}
	if r := do(t, h, http.MethodPatch, patchPath, admin, map[string]any{"url": moved.URL + "/"}); r.status != http.StatusOK || r.body["url"] != moved.URL {
		t.Fatalf("update to the same server at a new URL: %d %v", r.status, r.body)
	}
}

func TestHandshakeIsRateLimited(t *testing.T) {
	h := newTestServer(t)
	body := map[string]any{"nonce": make([]byte, 16)}
	for i := range handshakeLimit {
		if r := do(t, h, http.MethodPost, "/api/v1/federation/handshake", "", body); r.status != http.StatusOK {
			t.Fatalf("request %d within the limit: expected 200, got %d", i+1, r.status)
		}
	}
	if r := do(t, h, http.MethodPost, "/api/v1/federation/handshake", "", body); r.status != http.StatusTooManyRequests {
		t.Fatalf("request over the limit: expected 429, got %d", r.status)
	}
}

func TestHandshakeRejectsShortNonce(t *testing.T) {
	h := newTestServer(t)
	if r := do(t, h, http.MethodPost, "/api/v1/federation/handshake", "", map[string]any{"nonce": []byte("short")}); r.status != http.StatusBadRequest {
		t.Fatalf("short nonce: expected 400, got %d", r.status)
	}
}
