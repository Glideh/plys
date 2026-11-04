package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFederatedTokenWritesProgressOnlyForShadowUsers(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	aliceID := uuid.New()
	peer.users = []peerUserDTO{{ID: aliceID, Username: "alice"}}
	syncPath := "/api/v1/admin/federation/peers/" + peer.id.String() + "/sync"
	if r := do(t, env.h, http.MethodPost, syncPath, admin, nil); r.status != http.StatusOK {
		t.Fatalf("sync: %d %v", r.status, r.body)
	}

	put := func(subject uuid.UUID) int {
		token := forgeFederatedToken(peer.priv, federatedClaims{
			Issuer: peer.id, Subject: subject, Audience: env.srv.Identity.ID,
			IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix(),
		})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/items/"+uuid.New().String()+"/progress",
			bytes.NewBufferString(`{"positionSeconds":10,"durationSeconds":100}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		env.h.ServeHTTP(rec, req)
		return rec.Code
	}

	// The shadow user passes authentication; the random item then does not exist.
	if code := put(aliceID); code != http.StatusNotFound {
		t.Fatalf("shadow user: expected 404 for a missing item, got %d", code)
	}
	if code := put(uuid.New()); code != http.StatusUnauthorized {
		t.Fatalf("unknown subject: expected 401, got %d", code)
	}
}
