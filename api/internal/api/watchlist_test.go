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

func watchlistIDs(body map[string]any) []string {
	var out []string
	for _, it := range body["items"].([]any) {
		out = append(out, it.(map[string]any)["id"].(string))
	}
	return out
}

func TestWatchlist(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "films/Alien.1979.mkv", "films/Brazil.1985.mkv", "series/Show.S01E01.mkv")
	films := e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	series := e.createScannedLibrary(t, admin, "Séries", "series", "series", nil)
	movies := map[string]string{}
	for _, card := range e.get(t, admin, "/api/v1/libraries/"+films+"/items")["items"].([]any) {
		movies[card.(map[string]any)["title"].(string)] = card.(map[string]any)["id"].(string)
	}
	alien, brazil := movies["Alien"], movies["Brazil"]
	show := e.get(t, admin, "/api/v1/libraries/"+series+"/items")["items"].([]any)[0].(map[string]any)["id"].(string)
	season := e.get(t, admin, "/api/v1/items/"+show)["children"].([]any)[0].(map[string]any)["id"].(string)

	if got := watchlistIDs(e.get(t, admin, "/api/v1/me/watchlist")); len(got) != 0 {
		t.Fatalf("empty watchlist expected: %v", got)
	}
	if d := e.get(t, admin, "/api/v1/items/"+alien); d["inWatchlist"] != false {
		t.Fatalf("not listed yet: %v", d["inWatchlist"])
	}
	for _, id := range []string{alien, show, alien, brazil} {
		if r := do(t, e.h, http.MethodPost, "/api/v1/items/"+id+"/watchlist", admin, nil); r.status != http.StatusNoContent {
			t.Fatalf("add %s: %d %v", id, r.status, r.body)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if r := do(t, e.h, http.MethodPost, "/api/v1/items/"+season+"/watchlist", admin, nil); r.status != http.StatusBadRequest {
		t.Errorf("a season cannot be listed: %d", r.status)
	}
	if r := do(t, e.h, http.MethodPost, "/api/v1/items/"+uuid.NewString()+"/watchlist", admin, nil); r.status != http.StatusNotFound {
		t.Errorf("unknown item: %d", r.status)
	}
	if r := do(t, e.h, http.MethodPost, "/api/v1/items/"+alien+"/watchlist", "", nil); r.status != http.StatusUnauthorized {
		t.Errorf("watchlist needs authentication: %d", r.status)
	}

	list := e.get(t, admin, "/api/v1/me/watchlist")
	// Adding again keeps the first date: Alien stays the oldest entry.
	if got := watchlistIDs(list); len(got) != 3 || got[0] != brazil || got[1] != show || got[2] != alien {
		t.Fatalf("watchlist order: %v", got)
	}
	first := list["items"].([]any)[0].(map[string]any)
	source := first["source"].(map[string]any)
	if first["kind"] != "movie" || first["title"] != "Brazil" || source["serverId"] != e.srv.Identity.ID.String() || source["itemId"] != brazil || list["unavailableServers"] != float64(0) {
		t.Errorf("watchlist entry: %v", list)
	}
	if d := e.get(t, admin, "/api/v1/items/"+alien); d["inWatchlist"] != true {
		t.Errorf("listed title detail: %v", d["inWatchlist"])
	}

	for range 2 {
		if r := do(t, e.h, http.MethodDelete, "/api/v1/items/"+alien+"/watchlist", admin, nil); r.status != http.StatusNoContent {
			t.Fatalf("remove: %d", r.status)
		}
	}
	if got := watchlistIDs(e.get(t, admin, "/api/v1/me/watchlist")); len(got) != 2 || got[1] != show {
		t.Errorf("after removal: %v", got)
	}

	// The watchlist is per user and follows library access.
	if r := do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"}); r.status != http.StatusCreated {
		t.Fatalf("create user: %d", r.status)
	}
	bob := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123", "tokenDelivery": "body"}).body["accessToken"].(string)
	if got := watchlistIDs(e.get(t, bob, "/api/v1/me/watchlist")); len(got) != 0 {
		t.Errorf("watchlist is per user: %v", got)
	}
	if r := do(t, e.h, http.MethodPost, "/api/v1/items/"+alien+"/watchlist", bob, nil); r.status != http.StatusNoContent {
		t.Fatalf("bob adds: %d", r.status)
	}
	if r := do(t, e.h, http.MethodPatch, "/api/v1/libraries/"+films, admin, map[string]any{"restricted": true}); r.status != http.StatusOK {
		t.Fatalf("restrict: %d %v", r.status, r.body)
	}
	if got := watchlistIDs(e.get(t, bob, "/api/v1/me/watchlist")); len(got) != 0 {
		t.Errorf("a restricted library hides its titles: %v", got)
	}
	if r := do(t, e.h, http.MethodDelete, "/api/v1/items/"+alien+"/watchlist", bob, nil); r.status != http.StatusNotFound {
		t.Errorf("a restricted item must look missing: %d", r.status)
	}
}

func TestWatchlistAggregatesPeers(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	writeFiles(t, env.root, "films/Alien.1979.mkv")
	library := env.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	localID := env.get(t, admin, "/api/v1/libraries/"+library+"/items")["items"].([]any)[0].(map[string]any)["id"].(string)
	if r := do(t, env.h, http.MethodPost, "/api/v1/items/"+localID+"/watchlist", admin, nil); r.status != http.StatusNoContent {
		t.Fatal(r.body)
	}

	// The fake peer has no watchlist endpoint, like a server older than the feature: it adds nothing.
	if response := env.get(t, admin, "/api/v1/me/watchlist"); len(response["items"].([]any)) != 1 || response["unavailableServers"] != float64(0) {
		t.Fatalf("peer without watchlist: %v", response)
	}

	remoteID := uuid.New()
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/federation/watchlist" {
			peer.handler.ServeHTTP(w, r)
			return
		}
		poster := "/api/v1/images/tmdb/w342/film.jpg"
		_ = json.NewEncoder(w).Encode(watchlistResponse{Items: []watchlistItemDTO{
			{ID: remoteID, Kind: "series", Title: "Remote", PosterURL: &poster, AddedAt: time.Now().Add(time.Hour)},
		}})
	}))
	t.Cleanup(remote.Close)
	env.srv.HTTPClient = remote.Client()
	if _, err := env.srv.q.UpdatePeerURL(context.Background(), store.UpdatePeerURLParams{ID: peer.id, Url: remote.URL}); err != nil {
		t.Fatal(err)
	}
	response := env.get(t, admin, "/api/v1/me/watchlist")
	if got := watchlistIDs(response); len(got) != 2 || got[0] != remoteID.String() || got[1] != localID {
		t.Fatalf("merged order: %v", response)
	}
	other := response["items"].([]any)[0].(map[string]any)
	source := other["source"].(map[string]any)
	if source["serverId"] != peer.id.String() || source["serverUrl"] != remote.URL || source["itemId"] != remoteID.String() || other["posterUrl"] != remote.URL+"/api/v1/images/tmdb/w342/film.jpg" {
		t.Fatalf("remote source: %v", other)
	}

	peer.version = "99.0.0"
	response = env.get(t, admin, "/api/v1/me/watchlist")
	if response["unavailableServers"] != float64(1) || len(response["items"].([]any)) != 1 {
		t.Fatalf("incompatible peer: %v", response)
	}
}

func TestFederatedUserWatchlist(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	writeFiles(t, env.root, "films/Alien.1979.mkv")
	library := env.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	item := env.get(t, admin, "/api/v1/libraries/"+library+"/items")["items"].([]any)[0].(map[string]any)["id"].(string)

	if r := do(t, env.h, http.MethodGet, "/api/v1/federation/watchlist", admin, nil); r.status != http.StatusUnauthorized {
		t.Fatalf("origin session accepted: %d", r.status)
	}
	userID := uuid.New()
	peer.users = []peerUserDTO{{ID: userID, Username: "remote-viewer"}}
	if r := do(t, env.h, http.MethodPost, "/api/v1/admin/federation/peers/"+peer.id.String()+"/sync", admin, nil); r.status != http.StatusOK {
		t.Fatal(r.body)
	}
	token := forgeFederatedToken(peer.priv, federatedClaims{Issuer: peer.id, Subject: userID, Audience: env.srv.Identity.ID, IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix()})

	// The peer's user adds a title of this server, which keeps the entry for its shadow account.
	if r := do(t, env.h, http.MethodPost, "/api/v1/items/"+item+"/watchlist", token, nil); r.status != http.StatusNoContent {
		t.Fatalf("federated add: %d %v", r.status, r.body)
	}
	if d := do(t, env.h, http.MethodGet, "/api/v1/items/"+item, token, nil); d.body["inWatchlist"] != true {
		t.Errorf("federated detail: %v", d.body["inWatchlist"])
	}
	r := do(t, env.h, http.MethodGet, "/api/v1/federation/watchlist", token, nil)
	if r.status != http.StatusOK || len(r.body["items"].([]any)) != 1 || r.body["items"].([]any)[0].(map[string]any)["id"] != item {
		t.Fatalf("peer watchlist: %d %v", r.status, r.body)
	}
	if got := watchlistIDs(env.get(t, admin, "/api/v1/me/watchlist")); len(got) != 0 {
		t.Errorf("the shadow user's entry is not the admin's: %v", got)
	}
}
