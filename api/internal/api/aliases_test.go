package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"toto/api/internal/store"
)

func TestUserAlias(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	created := do(t, env.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"})
	if created.status != http.StatusCreated || created.body["displayName"] != "bob" || created.body["alias"] != nil {
		t.Fatalf("create: %d %v", created.status, created.body)
	}
	bobPath := "/api/v1/users/" + created.body["id"].(string) + "/alias"
	login := do(t, env.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"})
	bob := login.body["accessToken"].(string)

	if r := do(t, env.h, http.MethodPut, bobPath, bob, map[string]any{"alias": "Bobby"}); r.status != http.StatusForbidden {
		t.Fatalf("alias set by a user: %d", r.status)
	}
	if r := do(t, env.h, http.MethodPut, bobPath, admin, map[string]any{"alias": strings.Repeat("x", 65)}); r.status != http.StatusBadRequest {
		t.Fatalf("long alias: %d", r.status)
	}
	if r := do(t, env.h, http.MethodPut, "/api/v1/users/"+uuid.NewString()+"/alias", admin, map[string]any{"alias": "x"}); r.status != http.StatusNotFound {
		t.Fatalf("unknown user: %d", r.status)
	}
	if r := do(t, env.h, http.MethodPut, bobPath, admin, map[string]any{"alias": "  Uncle Bob  "}); r.status != http.StatusOK || r.body["alias"] != "Uncle Bob" || r.body["displayName"] != "Uncle Bob" || r.body["username"] != "bob" {
		t.Fatalf("set alias: %d %v", r.status, r.body)
	}
	if me := env.get(t, bob, "/api/v1/me"); me["displayName"] != "Uncle Bob" {
		t.Fatalf("me: %v", me)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "Uncle Bob", "password": "password123"}); r.status != http.StatusUnauthorized {
		t.Fatalf("login with the alias: %d", r.status)
	}

	// Remote users are managed by their server, but can still be named here.
	aliceID := uuid.New()
	peer.users = []peerUserDTO{{ID: aliceID, Username: "alice"}}
	if r := do(t, env.h, http.MethodPost, "/api/v1/admin/federation/peers/"+peer.id.String()+"/sync", admin, nil); r.status != http.StatusOK {
		t.Fatalf("sync: %d", r.status)
	}
	if r := do(t, env.h, http.MethodPut, "/api/v1/users/"+aliceID.String()+"/alias", admin, map[string]any{"alias": "Aunt Alice"}); r.status != http.StatusOK || r.body["displayName"] != "Aunt Alice" || r.body["originServer"] != "Peer" {
		t.Fatalf("remote alias: %d %v", r.status, r.body)
	}
	// A later sync keeps the alias.
	if r := do(t, env.h, http.MethodPost, "/api/v1/admin/federation/peers/"+peer.id.String()+"/sync", admin, nil); r.status != http.StatusOK {
		t.Fatalf("resync: %d", r.status)
	}
	items := env.get(t, admin, "/api/v1/users")["items"].([]any)
	if alice := findUser(items, "alice"); alice == nil || alice["displayName"] != "Aunt Alice" {
		t.Fatalf("remote alias after sync: %v", items)
	}
	// The listing is sorted by the name shown.
	if first := items[0].(map[string]any); first["username"] != "admin" || items[1].(map[string]any)["username"] != "alice" || items[2].(map[string]any)["username"] != "bob" {
		t.Fatalf("order: %v", items)
	}

	if r := do(t, env.h, http.MethodPut, bobPath, admin, map[string]any{"alias": ""}); r.status != http.StatusOK || r.body["alias"] != nil || r.body["displayName"] != "bob" {
		t.Fatalf("clear alias: %d %v", r.status, r.body)
	}
}

func TestPeerAlias(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/federation/home" {
			peer.handler.ServeHTTP(w, r)
			return
		}
		items := []sourcedHomeItem{{homeItemDTO: homeItemDTO{ID: uuid.New(), Kind: "movie", Title: "Remote", PositionSeconds: 90, UpdatedAt: time.Now()}}}
		_ = json.NewEncoder(w).Encode(homeResponse{ContinueWatching: items, NextUp: []sourcedHomeItem{}})
	}))
	t.Cleanup(remote.Close)
	env.srv.HTTPClient = remote.Client()
	if _, err := env.srv.q.UpdatePeerURL(context.Background(), store.UpdatePeerURLParams{ID: peer.id, Url: remote.URL}); err != nil {
		t.Fatal(err)
	}
	homeServer := func() any {
		items := env.get(t, admin, "/api/v1/me/home")["continueWatching"].([]any)
		return items[0].(map[string]any)["source"].(map[string]any)["serverName"]
	}
	if name := homeServer(); name != "Peer" {
		t.Fatalf("home before alias: %v", name)
	}

	path := "/api/v1/admin/federation/peers/" + peer.id.String() + "/alias"
	if r := do(t, env.h, http.MethodPut, "/api/v1/admin/federation/peers/"+uuid.NewString()+"/alias", admin, map[string]any{"alias": "x"}); r.status != http.StatusNotFound {
		t.Fatalf("unknown peer: %d", r.status)
	}
	if r := do(t, env.h, http.MethodPut, path, admin, map[string]any{"alias": "Grandma's"}); r.status != http.StatusOK || r.body["alias"] != "Grandma's" || r.body["displayName"] != "Grandma's" || r.body["name"] != "Peer" {
		t.Fatalf("set alias: %d %v", r.status, r.body)
	}
	// The cached home answer of the peer must not keep the previous name.
	if name := homeServer(); name != "Grandma's" {
		t.Fatalf("home after alias: %v", name)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/federation/peers", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	rec := httptest.NewRecorder()
	env.h.ServeHTTP(rec, req)
	var peers []peerDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &peers); err != nil || len(peers) != 1 || peers[0].DisplayName != "Grandma's" {
		t.Fatalf("peer list: %s", rec.Body.String())
	}

	if r := do(t, env.h, http.MethodPut, path, admin, map[string]any{"alias": nil}); r.status != http.StatusOK || r.body["alias"] != nil || r.body["displayName"] != "Peer" {
		t.Fatalf("clear alias: %d %v", r.status, r.body)
	}
}
