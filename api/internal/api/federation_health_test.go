package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"toto/api/internal/store"
)

func TestPeerBackoffAndProbeSchedule(t *testing.T) {
	for failures, want := range map[int32]time.Duration{1: 30 * time.Second, 2: 30 * time.Second, 3: time.Minute, 4: 5 * time.Minute, 9: 5 * time.Minute} {
		if got := peerBackoff(failures); got != want {
			t.Errorf("peerBackoff(%d) = %s, want %s", failures, got, want)
		}
	}
	now := time.Now()
	recent, old := now.Add(-20*time.Second), now.Add(-10*time.Minute)
	for _, test := range []struct {
		name string
		peer store.Peer
		want bool
	}{
		{"never checked", store.Peer{}, true},
		{"fresh version", store.Peer{VersionCheckedAt: &recent}, false},
		{"stale version", store.Peer{VersionCheckedAt: &old}, true},
		{"paused", store.Peer{Paused: true}, false},
		{"failing, backoff running", store.Peer{ConsecutiveFailures: 2, LastFailureAt: &recent, OfflineSince: &recent}, false},
		{"failing, backoff over", store.Peer{ConsecutiveFailures: 3, LastFailureAt: &old, OfflineSince: &old}, true},
	} {
		if got := peerProbeDue(test.peer, now, 5*time.Minute); got != test.want {
			t.Errorf("%s: peerProbeDue = %v, want %v", test.name, got, test.want)
		}
	}
}

// healthEnv pairs a server with a peer whose home endpoint can be broken, and counts the calls it receives.
type healthEnv struct {
	testEnv
	peer        *fakePeer
	admin       string
	down        atomic.Bool
	homeCalls   atomic.Int32
	serverCalls atomic.Int32
}

func newHealthEnv(t *testing.T) *healthEnv {
	t.Helper()
	env, peer, admin := pairedEnv(t)
	h := &healthEnv{testEnv: env, peer: peer, admin: admin}
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.down.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		switch r.URL.Path {
		case "/api/v1/server":
			h.serverCalls.Add(1)
		case "/api/v1/federation/home":
			h.homeCalls.Add(1)
			_ = json.NewEncoder(w).Encode(homeResponse{ContinueWatching: []sourcedHomeItem{}, NextUp: []sourcedHomeItem{}})
			return
		}
		peer.handler.ServeHTTP(w, r)
	}))
	t.Cleanup(remote.Close)
	env.srv.HTTPClient = remote.Client()
	env.srv.PeerVersionTTL = time.Hour
	if _, err := env.srv.q.UpdatePeerURL(context.Background(), store.UpdatePeerURLParams{ID: peer.id, Url: remote.URL}); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h *healthEnv) peerRow(t *testing.T) store.Peer {
	t.Helper()
	p, err := h.srv.q.GetPeer(context.Background(), h.peer.id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// ageLastFailure moves the last failure back, as if the next one came after the spacing and the backoff.
func (h *healthEnv) ageLastFailure(t *testing.T) {
	t.Helper()
	if _, err := h.srv.Pool.Exec(context.Background(), `UPDATE peers SET last_failure_at = now() - interval '10 minutes' WHERE id = $1`, h.peer.id); err != nil {
		t.Fatal(err)
	}
}

func (h *healthEnv) listPeers(t *testing.T) []peerDTO {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/federation/peers", nil)
	req.Header.Set("Authorization", "Bearer "+h.admin)
	recorder := httptest.NewRecorder()
	h.h.ServeHTTP(recorder, req)
	var peers []peerDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &peers); err != nil {
		t.Fatalf("list peers: %s", recorder.Body.String())
	}
	return peers
}

func TestOfflinePeerIsSkippedAndNamedThenRecovers(t *testing.T) {
	h := newHealthEnv(t)

	// A fresh version is trusted: the peer is not asked for it before each call.
	for range 2 {
		if response := h.get(t, h.admin, "/api/v1/me/home"); response["unavailableServers"] != float64(0) {
			t.Fatalf("healthy home: %v", response)
		}
	}
	if h.homeCalls.Load() != 2 || h.serverCalls.Load() != 0 {
		t.Fatalf("calls: home %d, server %d", h.homeCalls.Load(), h.serverCalls.Load())
	}

	h.down.Store(true)
	response := h.get(t, h.admin, "/api/v1/me/home")
	names, _ := response["unavailableServerNames"].([]any)
	if response["unavailableServers"] != float64(1) || len(names) != 1 || names[0] != "Peer" {
		t.Fatalf("first failure: %v", response)
	}
	if p := h.peerRow(t); p.ConsecutiveFailures != 1 || p.OfflineSince != nil {
		t.Fatalf("one failure must not mark the peer offline: %+v", p)
	}
	// Concurrent requests failing together are the same outage.
	h.get(t, h.admin, "/api/v1/me/home")
	if p := h.peerRow(t); p.ConsecutiveFailures != 1 {
		t.Fatalf("failures within the spacing were counted: %d", p.ConsecutiveFailures)
	}
	h.ageLastFailure(t)
	h.get(t, h.admin, "/api/v1/me/home")
	if p := h.peerRow(t); p.ConsecutiveFailures != 2 || p.OfflineSince == nil {
		t.Fatalf("second failure must mark the peer offline: %+v", p)
	}

	// An offline peer is not called any more, but it is still named.
	h.down.Store(false)
	before := h.homeCalls.Load()
	response = h.get(t, h.admin, "/api/v1/me/home")
	names, _ = response["unavailableServerNames"].([]any)
	if h.homeCalls.Load() != before || response["unavailableServers"] != float64(1) || len(names) != 1 || names[0] != "Peer" {
		t.Fatalf("offline peer called or not named: %d calls, %v", h.homeCalls.Load()-before, response)
	}
	if peers := h.listPeers(t); len(peers) != 1 || peers[0].Status != "offline" || peers[0].OfflineSince == nil {
		t.Fatalf("admin status: %+v", peers)
	}
	if r := do(t, h.h, http.MethodPost, "/api/v1/me/federation-token", h.admin, map[string]any{"audience": h.peer.id}); r.status != http.StatusConflict {
		t.Fatalf("token for an offline peer: %d", r.status)
	}

	// The monitor waits for the backoff, then brings the peer back.
	h.srv.probeDuePeers(context.Background())
	if p := h.peerRow(t); p.OfflineSince == nil {
		t.Fatal("peer probed before its backoff")
	}
	h.ageLastFailure(t)
	h.srv.probeDuePeers(context.Background())
	if p := h.peerRow(t); p.OfflineSince != nil || p.ConsecutiveFailures != 0 || p.LastSeenAt == nil {
		t.Fatalf("probe did not bring the peer back: %+v", p)
	}
	if response := h.get(t, h.admin, "/api/v1/me/home"); response["unavailableServers"] != float64(0) || h.homeCalls.Load() != before+1 {
		t.Fatalf("recovered home: %v", response)
	}
}

func TestSignedRequestBringsOfflinePeerBack(t *testing.T) {
	h := newHealthEnv(t)
	userID := uuid.New()
	h.peer.users = []peerUserDTO{{ID: userID, Username: "remote-user"}}
	if r := do(t, h.h, http.MethodPost, "/api/v1/admin/federation/peers/"+h.peer.id.String()+"/sync", h.admin, nil); r.status != http.StatusOK {
		t.Fatalf("sync: %v", r.body)
	}
	now := time.Now()
	if _, err := h.srv.Pool.Exec(context.Background(), `UPDATE peers SET consecutive_failures = 2, last_failure_at = $2, offline_since = $2 WHERE id = $1`, h.peer.id, now); err != nil {
		t.Fatal(err)
	}
	token := forgeFederatedToken(h.peer.priv, federatedClaims{Issuer: h.peer.id, Subject: userID, Audience: h.srv.Identity.ID, IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
	if r := do(t, h.h, http.MethodGet, "/api/v1/federation/home", token, nil); r.status != http.StatusOK {
		t.Fatalf("federated home: %d %v", r.status, r.body)
	}
	if p := h.peerRow(t); p.OfflineSince != nil || p.ConsecutiveFailures != 0 {
		t.Fatalf("signed request did not bring the peer back: %+v", p)
	}
}

func TestPausedPeerIsLeftOut(t *testing.T) {
	h := newHealthEnv(t)
	path := "/api/v1/admin/federation/peers/" + h.peer.id.String() + "/paused"
	if r := do(t, h.h, http.MethodPut, path, h.admin, map[string]any{}); r.status != http.StatusBadRequest {
		t.Fatalf("missing paused: %d", r.status)
	}
	if r := do(t, h.h, http.MethodPut, "/api/v1/admin/federation/peers/"+uuid.NewString()+"/paused", h.admin, map[string]any{"paused": true}); r.status != http.StatusNotFound {
		t.Fatalf("unknown peer: %d", r.status)
	}
	if r := do(t, h.h, http.MethodPut, path, h.admin, map[string]any{"paused": true}); r.status != http.StatusOK || r.body["status"] != "paused" || r.body["paused"] != true {
		t.Fatalf("pause: %d %v", r.status, r.body)
	}
	response := h.get(t, h.admin, "/api/v1/me/home")
	if h.homeCalls.Load() != 0 || response["unavailableServers"] != float64(0) || len(response["unavailableServerNames"].([]any)) != 0 {
		t.Fatalf("paused peer called or reported: %d calls, %v", h.homeCalls.Load(), response)
	}
	if catalog := h.get(t, h.admin, "/api/v1/kinds/movies/items"); catalog["unavailableServers"] != float64(0) {
		t.Fatalf("paused peer reported in the catalog: %v", catalog)
	}
	if r := do(t, h.h, http.MethodPost, "/api/v1/me/federation-token", h.admin, map[string]any{"audience": h.peer.id}); r.status != http.StatusConflict {
		t.Fatalf("token for a paused peer: %d", r.status)
	}
	h.srv.probeDuePeers(context.Background())
	if h.serverCalls.Load() != 0 {
		t.Fatal("paused peer probed")
	}
	if peers := h.listPeers(t); len(peers) != 1 || peers[0].Status != "paused" {
		t.Fatalf("admin status: %+v", peers)
	}
	if r := do(t, h.h, http.MethodPut, path, h.admin, map[string]any{"paused": false}); r.status != http.StatusOK || r.body["status"] != "compatible" {
		t.Fatalf("resume: %d %v", r.status, r.body)
	}
	h.get(t, h.admin, "/api/v1/me/home")
	if h.homeCalls.Load() != 1 {
		t.Fatalf("resumed peer not called: %d", h.homeCalls.Load())
	}
}
