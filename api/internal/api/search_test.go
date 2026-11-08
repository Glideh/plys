package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"toto/api/internal/store"
)

func searchTitles(t *testing.T, group map[string]any) []string {
	t.Helper()
	var titles []string
	for _, it := range group["items"].([]any) {
		titles = append(titles, it.(map[string]any)["title"].(string))
	}
	return titles
}

func searchGroups(t *testing.T, body map[string]any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, g := range body["groups"].([]any) {
		m := g.(map[string]any)
		out[m["kind"].(string)] = m
	}
	return out
}

func TestGlobalSearch(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root,
		"films/Alien.1979.mkv",
		"films/Aliens.1986.mkv",
		"films/The.Alien.Factor.1978.mkv",
		"films/Blade.Runner.1982.mkv",
		"private/Alien.Covenant.2017.mkv",
		"series/Alien.Nation.S01E01.mkv",
		"series/Show.S01E01.mkv",
	)
	bobID := do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"}).body["id"].(string)
	do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "carol", "password": "password123"})
	bob := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)
	carol := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "carol", "password": "password123"}).body["accessToken"].(string)
	e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	e.createScannedLibrary(t, admin, "Privé", "movies", "private", map[string]any{"restricted": true, "userIds": []string{bobID}})
	e.createScannedLibrary(t, admin, "Séries", "series", "series", nil)

	for _, q := range []string{"", "al", "+al+", "%C3%A9t"} {
		if r := do(t, e.h, http.MethodGet, "/api/v1/search?query="+q, carol, nil); r.status != http.StatusBadRequest {
			t.Errorf("query %q: expected 400, got %d", q, r.status)
		}
	}
	if r := do(t, e.h, http.MethodGet, "/api/v1/search?query=alien&limit=0", carol, nil); r.status != http.StatusBadRequest {
		t.Errorf("limit 0: expected 400, got %d", r.status)
	}
	if r := do(t, e.h, http.MethodGet, "/api/v1/search?query=alien", "", nil); r.status != http.StatusUnauthorized {
		t.Errorf("without login: expected 401, got %d", r.status)
	}

	r := do(t, e.h, http.MethodGet, "/api/v1/search?query=ALIEN", carol, nil)
	if r.status != http.StatusOK || r.body["unavailableServers"] != float64(0) {
		t.Fatalf("search: %d %v", r.status, r.body)
	}
	groups := searchGroups(t, r.body)
	if len(groups) != 2 || groups["movies"] == nil || groups["series"] == nil {
		t.Fatalf("only kinds with matches must be returned: %v", r.body)
	}
	movies := searchTitles(t, groups["movies"])
	if len(movies) != 3 || movies[0] != "Alien" || movies[1] != "Aliens" || movies[2] != "The Alien Factor" {
		t.Errorf("titles starting with the query must come first: %v", movies)
	}
	if series := searchTitles(t, groups["series"]); len(series) != 1 || series[0] != "Alien Nation" {
		t.Errorf("series: %v", series)
	}

	r = do(t, e.h, http.MethodGet, "/api/v1/search?query=alien&limit=1", bob, nil)
	movieGroup := searchGroups(t, r.body)["movies"]
	if got := searchTitles(t, movieGroup); len(got) != 1 || movieGroup["total"] != float64(4) {
		t.Errorf("bob must see the restricted library, one item per kind and the full total: %v", movieGroup)
	}

	if r = do(t, e.h, http.MethodGet, "/api/v1/search?query=nothing%25", carol, nil); r.status != http.StatusOK || len(r.body["groups"].([]any)) != 0 {
		t.Errorf("no match must return no group: %d %v", r.status, r.body)
	}
}

func TestGlobalSearchIncludesPeers(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	queries := make(chan string, 3)
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/federation/catalog/kinds/movies/items":
			queries <- r.URL.Query().Get("query")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"id": uuid.New(), "kind": "movie", "title": "Remote Alien", "sortTitle": "remote alien", "tmdbId": 348, "addedAt": time.Now(), "posterUrl": "/api/v1/images/tmdb/w342/a.jpg"}}, "total": 1})
		case "/api/v1/federation/catalog/kinds/series/items", "/api/v1/federation/catalog/kinds/other/items":
			queries <- r.URL.Query().Get("query")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
		default:
			peer.handler.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(remote.Close)
	env.srv.HTTPClient = remote.Client()
	if _, err := env.srv.q.UpdatePeerURL(context.Background(), store.UpdatePeerURLParams{ID: peer.id, Url: remote.URL}); err != nil {
		t.Fatal(err)
	}

	r := do(t, env.h, http.MethodGet, "/api/v1/search?query=+alien+", admin, nil)
	if r.status != http.StatusOK || r.body["unavailableServers"] != float64(0) {
		t.Fatalf("search: %d %v", r.status, r.body)
	}
	for range 3 {
		if q := <-queries; q != "alien" {
			t.Errorf("peer received query %q", q)
		}
	}
	groups := searchGroups(t, r.body)
	if len(groups) != 1 || groups["movies"] == nil {
		t.Fatalf("peer movies: %v", r.body)
	}
	card := groups["movies"]["items"].([]any)[0].(map[string]any)
	source := card["sources"].([]any)[0].(map[string]any)
	if card["title"] != "Remote Alien" || source["serverId"] != peer.id.String() || card["posterUrl"] != remote.URL+"/api/v1/images/tmdb/w342/a.jpg" {
		t.Errorf("peer card: %v", card)
	}

	peer.version = "1.0.0"
	r = do(t, env.h, http.MethodGet, "/api/v1/search?query=alien", admin, nil)
	if r.status != http.StatusOK || len(r.body["groups"].([]any)) != 0 || r.body["unavailableServers"] != float64(1) {
		t.Fatalf("an incompatible peer must be counted once: %d %v", r.status, r.body)
	}
}
