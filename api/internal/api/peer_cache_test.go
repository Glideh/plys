package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"toto/api/internal/store"
)

// catalogPeer serves a catalog of count untagged movies, refusing pages larger than maxLimit like an older peer.
func catalogPeer(t *testing.T, env testEnv, peer *fakePeer, count, maxLimit int) (*atomic.Int32, *atomic.Int32) {
	t.Helper()
	calls, refused := &atomic.Int32{}, &atomic.Int32{}
	cards := make([]map[string]any, count)
	for i := range cards {
		title := "Remote " + strconv.Itoa(10000+i)
		cards[i] = map[string]any{"id": uuid.New(), "kind": "movie", "title": title, "sortTitle": strings.ToLower(title), "addedAt": time.Now()}
	}
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/federation/catalog/kinds/movies/items" {
			peer.handler.ServeHTTP(w, r)
			return
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit > maxLimit {
			refused.Add(1)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		calls.Add(1)
		end := min(offset+limit, count)
		_ = json.NewEncoder(w).Encode(map[string]any{"items": cards[min(offset, end):end], "total": count})
	}))
	t.Cleanup(remote.Close)
	env.srv.HTTPClient = remote.Client()
	env.srv.PeerVersionTTL = time.Hour
	if _, err := env.srv.q.UpdatePeerURL(context.Background(), store.UpdatePeerURLParams{ID: peer.id, Url: remote.URL}); err != nil {
		t.Fatal(err)
	}
	return calls, refused
}

func TestFederatedCatalogIsReadInOneCallAndCachedAcrossPages(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	env.srv.peerCatalogs = newPeerCache[[]sourcedCard](time.Minute, 0, peerCatalogCacheSize)
	calls, refused := catalogPeer(t, env, peer, 450, maxPeerPageSize)

	first := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items?limit=60", admin, nil)
	if first.status != http.StatusOK || first.body["total"] != float64(450) || len(first.body["items"].([]any)) != 60 {
		t.Fatalf("first page: %d %v", first.status, first.body["total"])
	}
	if calls.Load() != 1 || refused.Load() != 0 {
		t.Fatalf("whole catalog must be read in one call: %d calls, %d refused", calls.Load(), refused.Load())
	}
	next := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items?limit=60&offset=60", admin, nil)
	if next.status != http.StatusOK || len(next.body["items"].([]any)) != 60 {
		t.Fatalf("second page: %d %v", next.status, next.body)
	}
	if calls.Load() != 1 {
		t.Fatalf("next page must come from the cache: %d calls", calls.Load())
	}
	// Another search is another answer.
	do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items?query=remote", admin, nil)
	if calls.Load() != 2 {
		t.Fatalf("another query must ask the peer: %d calls", calls.Load())
	}
	// A paused peer is not served from the cache.
	if _, err := env.srv.Pool.Exec(context.Background(), `UPDATE peers SET paused = true WHERE id = $1`, peer.id); err != nil {
		t.Fatal(err)
	}
	if paused := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items", admin, nil); paused.body["total"] != float64(0) {
		t.Fatalf("paused peer served from cache: %v", paused.body["total"])
	}
}

func TestFederatedCatalogPagesThroughOlderPeers(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	calls, refused := catalogPeer(t, env, peer, 450, 200)

	result := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items", admin, nil)
	if result.status != http.StatusOK || result.body["total"] != float64(450) || result.body["unavailableServers"] != float64(0) {
		t.Fatalf("older peer: %d %v", result.status, result.body)
	}
	if refused.Load() != 1 || calls.Load() != 3 {
		t.Fatalf("expected one refused large page then 3 pages: %d refused, %d calls", refused.Load(), calls.Load())
	}
	if p, _ := env.srv.q.GetPeer(context.Background(), peer.id); p.ConsecutiveFailures != 0 {
		t.Fatalf("the refused page must not count as a failure: %+v", p)
	}
}

func TestListingPageSizes(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	if result := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items?limit=1000", admin, nil); result.status != http.StatusOK {
		t.Fatalf("limit 1000: %d", result.status)
	}
	if result := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items?limit=1001", admin, nil); result.status != http.StatusBadRequest {
		t.Fatalf("limit 1001: %d", result.status)
	}
	userID := uuid.New()
	peer.users = []peerUserDTO{{ID: userID, Username: "remote-viewer"}}
	if result := do(t, env.h, http.MethodPost, "/api/v1/admin/federation/peers/"+peer.id.String()+"/sync", admin, nil); result.status != http.StatusOK {
		t.Fatal(result.body)
	}
	token := forgeFederatedToken(peer.priv, federatedClaims{Issuer: peer.id, Subject: userID, Audience: env.srv.Identity.ID, IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix()})
	if result := do(t, env.h, http.MethodGet, "/api/v1/federation/catalog/kinds/movies/items?limit="+strconv.Itoa(maxPeerPageSize), token, nil); result.status != http.StatusOK {
		t.Fatalf("peer page of %d: %d %v", maxPeerPageSize, result.status, result.body)
	}
}

func TestJSONAnswersAreCompressed(t *testing.T) {
	env := newTestEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/server", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	env.h.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("not compressed: %v", rec.Header())
	}
	reader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	var info map[string]any
	if err := json.NewDecoder(reader).Decode(&info); err != nil || info["id"] == nil {
		t.Fatalf("decoded: %v %v", info, err)
	}
}

func TestDecodePeerJSONReportsOversizedAnswers(t *testing.T) {
	var out map[string]any
	if err := decodePeerJSON(strings.NewReader(`{"a":"`+strings.Repeat("x", 100)+`"}`), 50, &out); !errors.Is(err, errPeerResponseTooLarge) {
		t.Fatalf("oversized answer: %v", err)
	}
	if err := decodePeerJSON(strings.NewReader(`{"a":1}`), 7, &out); err != nil || out["a"] != float64(1) {
		t.Fatalf("answer at the limit: %v %v", out, err)
	}
}

func TestPeerCacheExpiresAndStaysBounded(t *testing.T) {
	c := newPeerCache[int](time.Minute, 0, 2)
	c.set("a", 1)
	c.set("b", 2)
	c.set("c", 3)
	if len(c.entries) != 2 {
		t.Fatalf("cache grew past its bound: %d", len(c.entries))
	}
	if v, ok := c.get("c"); !ok || v != 3 {
		t.Fatal("latest answer missing")
	}
	c.entries["c"] = peerCacheEntry[int]{value: 3, expires: time.Now().Add(-time.Second)}
	if _, ok := c.get("c"); ok {
		t.Fatal("expired answer served")
	}
	disabled := newPeerCache[int](time.Minute, -1, 2)
	disabled.set("a", 1)
	if _, ok := disabled.get("a"); ok {
		t.Fatal("disabled cache kept an answer")
	}
}

func TestFlightGroupSharesCallsAndCancelsAbandonedOnes(t *testing.T) {
	var g flightGroup[int]
	var calls atomic.Int32
	release := make(chan struct{})
	call := func(ctx context.Context) (int, error) {
		calls.Add(1)
		select {
		case <-release:
			return 42, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	var wg sync.WaitGroup
	results := make([]int, 3)
	for i := range results {
		wg.Go(func() { results[i], _ = g.do(context.Background(), "k", call) })
	}
	for calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls.Load() != 1 || results[0] != 42 || results[1] != 42 || results[2] != 42 {
		t.Fatalf("calls %d, results %v", calls.Load(), results)
	}

	// Once every caller has left, the call is cancelled.
	cancelled := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_, _ = g.do(ctx, "slow", func(ctx context.Context) (int, error) {
			<-ctx.Done()
			close(cancelled)
			return 0, ctx.Err()
		})
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("abandoned call kept running")
	}
}
