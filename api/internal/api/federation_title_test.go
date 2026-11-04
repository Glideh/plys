package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPeerTitleIDsRequiresShadowUserToken(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	aliceID := uuid.New()
	peer.users = []peerUserDTO{{ID: aliceID, Username: "alice"}}
	syncPath := "/api/v1/admin/federation/peers/" + peer.id.String() + "/sync"
	if r := do(t, env.h, http.MethodPost, syncPath, admin, nil); r.status != http.StatusOK {
		t.Fatalf("sync: %d %v", r.status, r.body)
	}

	get := func(token string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/federation/catalog/titles/movie/603", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		env.h.ServeHTTP(rec, req)
		return rec.Result()
	}
	forge := func(subject uuid.UUID) string {
		return forgeFederatedToken(peer.priv, federatedClaims{
			Issuer: peer.id, Subject: subject, Audience: env.srv.Identity.ID,
			IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix(),
		})
	}

	res := get(forge(aliceID))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("shadow user: expected 200, got %d", res.StatusCode)
	}
	var body struct {
		Items []titleRefDTO `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 0 {
		t.Fatalf("expected no visible title for a fresh shadow user, got %d", len(body.Items))
	}
	if code := get(forge(uuid.New())).StatusCode; code != http.StatusUnauthorized {
		t.Fatalf("unknown subject: expected 401, got %d", code)
	}
	if code := get("").StatusCode; code != http.StatusUnauthorized {
		t.Fatalf("no token: expected 401, got %d", code)
	}
}

func TestListTitleSourcesRequiresToken(t *testing.T) {
	env, _, admin := pairedEnv(t)
	path := "/api/v1/kinds/movie/titles/603/sources"
	if r := do(t, env.h, http.MethodGet, path, admin, nil); r.status != http.StatusOK {
		t.Fatalf("admin: expected 200, got %d %v", r.status, r.body)
	}
	if r := do(t, env.h, http.MethodGet, path, "", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("no token: expected 401, got %d", r.status)
	}
}
