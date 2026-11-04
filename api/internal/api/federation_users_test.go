package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
)

const peerUsersPath = "/api/v1/federation/users"

// signedPeerRequest builds a listing request as the given peer key would sign it.
func signedPeerRequest(peerID uuid.UUID, priv ed25519.PrivateKey, at time.Time) *http.Request {
	req := httptest.NewRequest(http.MethodGet, peerUsersPath, nil)
	stamp := strconv.FormatInt(at.Unix(), 10)
	sig := ed25519.Sign(priv, peerRequestMessage(http.MethodGet, peerUsersPath, stamp))
	req.Header.Set(headerPeerID, peerID.String())
	req.Header.Set(headerTimestamp, stamp)
	req.Header.Set(headerSignature, base64.RawURLEncoding.EncodeToString(sig))
	return req
}

func findUser(items []any, username string) map[string]any {
	for _, it := range items {
		if u := it.(map[string]any); u["username"] == username {
			return u
		}
	}
	return nil
}

func TestPeerUsersRequireValidSignature(t *testing.T) {
	env, peer, _ := pairedEnv(t)
	serve := func(req *http.Request) int {
		rec := httptest.NewRecorder()
		env.h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := serve(httptest.NewRequest(http.MethodGet, peerUsersPath, nil)); code != http.StatusUnauthorized {
		t.Fatalf("unsigned request: expected 401, got %d", code)
	}
	if code := serve(signedPeerRequest(peer.id, peer.priv, time.Now())); code != http.StatusOK {
		t.Fatalf("signed by paired peer: expected 200, got %d", code)
	}
	_, stranger, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if code := serve(signedPeerRequest(peer.id, stranger, time.Now())); code != http.StatusUnauthorized {
		t.Fatalf("forged signature: expected 401, got %d", code)
	}
	if code := serve(signedPeerRequest(uuid.New(), peer.priv, time.Now())); code != http.StatusUnauthorized {
		t.Fatalf("unknown peer: expected 401, got %d", code)
	}
	if code := serve(signedPeerRequest(peer.id, peer.priv, time.Now().Add(-10*time.Minute))); code != http.StatusUnauthorized {
		t.Fatalf("stale timestamp: expected 401, got %d", code)
	}
}

func TestRemoteUsersAreShadowRows(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	aliceID := uuid.New()
	peer.users = []peerUserDTO{{ID: aliceID, Username: "alice"}}
	syncPath := "/api/v1/admin/federation/peers/" + peer.id.String() + "/sync"

	if r := do(t, env.h, http.MethodPost, syncPath, admin, nil); r.status != http.StatusOK || r.body["users"] != float64(1) {
		t.Fatalf("sync: %d %v", r.status, r.body)
	}
	list := do(t, env.h, http.MethodGet, "/api/v1/users", admin, nil)
	alice := findUser(list.body["items"].([]any), "alice")
	if alice == nil {
		t.Fatalf("remote user missing from admin list: %v", list.body)
	}
	if alice["remote"] != true || alice["originServer"] != "Peer" || alice["id"] != aliceID.String() {
		t.Fatalf("remote user fields: %v", alice)
	}

	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "alice", "password": "correct-horse-1"}); r.status != http.StatusUnauthorized {
		t.Fatalf("remote user login: expected 401, got %d", r.status)
	}
	userPath := "/api/v1/users/" + aliceID.String()
	if r := do(t, env.h, http.MethodPatch, userPath, admin, map[string]any{"password": "new-password-1"}); r.status != http.StatusConflict {
		t.Fatalf("password change on remote user: expected 409, got %d", r.status)
	}
	if r := do(t, env.h, http.MethodDelete, userPath, admin, nil); r.status != http.StatusConflict {
		t.Fatalf("delete remote user: expected 409, got %d", r.status)
	}

	peer.users = nil
	if r := do(t, env.h, http.MethodPost, syncPath, admin, nil); r.status != http.StatusOK || r.body["users"] != float64(0) {
		t.Fatalf("sync after removal: %d %v", r.status, r.body)
	}
	list = do(t, env.h, http.MethodGet, "/api/v1/users", admin, nil)
	if findUser(list.body["items"].([]any), "alice") != nil {
		t.Fatal("user removed by the peer is still listed")
	}
}

func TestListingUsersRefreshesPeerUsers(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	peer.users = []peerUserDTO{{ID: uuid.New(), Username: "alice"}}
	if r := do(t, env.h, http.MethodGet, "/api/v1/users", admin, nil); r.body["syncing"] != true {
		t.Fatalf("listing did not start a peer sync: %v", r.body)
	}
	waitForUser(t, env, admin, "alice")

	peer.users = []peerUserDTO{{ID: uuid.New(), Username: "bob"}}
	if r := do(t, env.h, http.MethodGet, "/api/v1/users", admin, nil); r.body["syncing"] != false || findUser(r.body["items"].([]any), "bob") != nil {
		t.Fatalf("peer re-read within the interval: %v", r.body)
	}

	env.srv.PeerUserSyncInterval = time.Nanosecond
	if r := do(t, env.h, http.MethodGet, "/api/v1/users", admin, nil); r.body["syncing"] != true {
		t.Fatalf("peer not due after the interval: %v", r.body)
	}
	waitForUser(t, env, admin, "bob")
}

func waitForUser(t *testing.T, env testEnv, admin, username string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		r := do(t, env.h, http.MethodGet, "/api/v1/users", admin, nil)
		if findUser(r.body["items"].([]any), username) != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("user %s never appeared: %v", username, r.body)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestLibrariesAreRestrictedByDefault(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	aliceID := uuid.New()
	peer.users = []peerUserDTO{{ID: aliceID, Username: "alice"}}
	if r := do(t, env.h, http.MethodPost, "/api/v1/admin/federation/peers/"+peer.id.String()+"/sync", admin, nil); r.status != http.StatusOK {
		t.Fatalf("sync: %d", r.status)
	}

	base := map[string]any{"name": "Films", "kind": "movies", "paths": libraryDir(t, env, "films")}
	r := do(t, env.h, http.MethodPost, "/api/v1/libraries", admin, base)
	if r.status != http.StatusCreated || r.body["restricted"] != true {
		t.Fatalf("library without restricted field: expected restricted, got %d %v", r.status, r.body)
	}

	granted := map[string]any{"name": "Series", "kind": "series", "paths": libraryDir(t, env, "series"), "userIds": []string{aliceID.String()}}
	r = do(t, env.h, http.MethodPost, "/api/v1/libraries", admin, granted)
	if r.status != http.StatusCreated || r.body["restricted"] != true {
		t.Fatalf("library granted to a remote user: %d %v", r.status, r.body)
	}

	open := map[string]any{"name": "Autres", "kind": "other", "paths": libraryDir(t, env, "other"), "restricted": false}
	if r := do(t, env.h, http.MethodPost, "/api/v1/libraries", admin, open); r.status != http.StatusCreated || r.body["restricted"] != false {
		t.Fatalf("explicit unrestricted library: %d %v", r.status, r.body)
	}
}

func libraryDir(t *testing.T, env testEnv, name string) []string {
	t.Helper()
	dir := filepath.Join(env.root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{dir}
}
