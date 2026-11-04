package api

import (
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func init() { Version = "0.2.0-dev" }

func TestCompatiblePeerVersions(t *testing.T) {
	for _, test := range []struct {
		local, remote string
		want          bool
	}{
		{"0.2.0", "0.9.0", true},
		{"0.2.0-dev", "0.3.0", true},
		{"1.2.3-rc.1+build.4", "1.0.0", true},
		{"0.2.0", "1.0.0", false},
		{"1.0.0", "10.0.0", false},
		{"dev", "dev", false},
		{"0.2.0", "", false},
		{"0.2", "0.2.0", false},
		{"00.2.0", "0.2.0", false},
		{"0.2.0-01", "0.2.0", false},
		{"0.2.0", "0.2.0 trailing", false},
	} {
		t.Run(test.local+"/"+test.remote, func(t *testing.T) {
			if got := compatiblePeerVersions(test.local, test.remote); got != test.want {
				t.Fatalf("compatiblePeerVersions(%q, %q) = %v, want %v", test.local, test.remote, got, test.want)
			}
		})
	}
}

func TestIncompatiblePeerCannotUseExistingToken(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	userID := uuid.New()
	peer.users = []peerUserDTO{{ID: userID, Username: "remote-user"}}
	if result := do(t, env.h, http.MethodPost, "/api/v1/admin/federation/peers/"+peer.id.String()+"/sync", admin, nil); result.status != http.StatusOK {
		t.Fatalf("sync: %v", result.body)
	}
	token := forgeFederatedToken(peer.priv, federatedClaims{Issuer: peer.id, Subject: userID, Audience: env.srv.Identity.ID, IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix()})
	path := "/api/v1/items/" + uuid.NewString()
	if result := do(t, env.h, http.MethodGet, path, token, nil); result.status != http.StatusNotFound {
		t.Fatalf("valid federated user: %d %v", result.status, result.body)
	}
	peer.version = "1.0.0"
	if result := do(t, env.h, http.MethodGet, path, token, nil); result.status != http.StatusUnauthorized {
		t.Fatalf("incompatible token accepted: %d %v", result.status, result.body)
	}
}

func TestPeerVersionChanges(t *testing.T) {
	peer := newFakePeer(t)
	env := newTestEnvWith(t, func(deps *Deps) { deps.HTTPClient = peer.srv.Client() })
	admin := setupAdmin(t, env.h).body["accessToken"].(string)
	path := "/api/v1/admin/federation/peers"
	result := do(t, env.h, http.MethodPost, path, admin, map[string]any{"code": pairingCodeFor(t, peer.srv.URL, peer.pub)})
	if result.status != http.StatusCreated || result.body["status"] != "compatible" {
		t.Fatalf("pair: %d %v", result.status, result.body)
	}
	for _, test := range []struct{ version, status string }{{"1.0.0", "incompatible"}, {"dev", "unverified"}, {"0.9.0-dev", "compatible"}} {
		peer.version = test.version
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+admin)
		recorder := httptest.NewRecorder()
		env.h.ServeHTTP(recorder, req)
		var peers []peerDTO
		if err := json.Unmarshal(recorder.Body.Bytes(), &peers); err != nil {
			t.Fatal(err)
		}
		if len(peers) != 1 || peers[0].Status != test.status || peers[0].Version == nil || *peers[0].Version != test.version {
			t.Fatalf("peer status: %s", recorder.Body.String())
		}
		token := do(t, env.h, http.MethodPost, "/api/v1/me/federation-token", admin, map[string]any{"audience": peer.id})
		if test.status == "compatible" && token.status != http.StatusOK || test.status != "compatible" && token.status != http.StatusConflict {
			t.Fatalf("token: %d %v", token.status, token.body)
		}
	}
}
