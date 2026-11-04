package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFederatedTokenReadsItemsOnlyForShadowUsers(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	aliceID := uuid.New()
	peer.users = []peerUserDTO{{ID: aliceID, Username: "alice"}}
	syncPath := "/api/v1/admin/federation/peers/" + peer.id.String() + "/sync"
	if r := do(t, env.h, http.MethodPost, syncPath, admin, nil); r.status != http.StatusOK {
		t.Fatalf("sync: %d %v", r.status, r.body)
	}

	get := func(subject uuid.UUID) int {
		token := forgeFederatedToken(peer.priv, federatedClaims{
			Issuer: peer.id, Subject: subject, Audience: env.srv.Identity.ID,
			IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/items/"+uuid.New().String(), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		env.h.ServeHTTP(rec, req)
		return rec.Code
	}

	// The shadow user passes authentication; the random item then does not exist.
	if code := get(aliceID); code != http.StatusNotFound {
		t.Fatalf("shadow user: expected 404 for a missing item, got %d", code)
	}
	if code := get(uuid.New()); code != http.StatusUnauthorized {
		t.Fatalf("unknown subject: expected 401, got %d", code)
	}
	if r := do(t, env.h, http.MethodGet, "/api/v1/items/"+uuid.New().String(), "", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("no token: expected 401, got %d", r.status)
	}
}
