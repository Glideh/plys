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

func TestHomeAggregatesSourcesAndSkipsIncompatiblePeers(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	writeFiles(t, env.root, "films/Alien.1979.mkv")
	library := env.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	localID := env.get(t, admin, "/api/v1/libraries/"+library+"/items")["items"].([]any)[0].(map[string]any)["id"].(string)
	if result := env.putProgress(t, admin, localID, 100, 7200); result.status != http.StatusOK {
		t.Fatal(result.body)
	}
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/federation/home" {
			peer.handler.ServeHTTP(w, r)
			return
		}
		items := make([]sourcedHomeItem, 25)
		poster := "/api/v1/images/tmdb/w342/film.jpg"
		for index := range items {
			items[index] = sourcedHomeItem{homeItemDTO: homeItemDTO{ID: uuid.New(), Kind: "movie", Title: "Remote", PosterURL: &poster, PositionSeconds: 90, UpdatedAt: time.Now().Add(-time.Hour - time.Duration(index)*time.Minute)}}
		}
		_ = json.NewEncoder(w).Encode(homeResponse{ContinueWatching: items, NextUp: []sourcedHomeItem{}})
	}))
	t.Cleanup(remote.Close)
	env.srv.HTTPClient = remote.Client()
	if _, err := env.srv.q.UpdatePeerURL(context.Background(), store.UpdatePeerURLParams{ID: peer.id, Url: remote.URL}); err != nil {
		t.Fatal(err)
	}
	response := env.get(t, admin, "/api/v1/me/home")
	items := response["continueWatching"].([]any)
	if len(items) != homeRailSize || items[0].(map[string]any)["id"] != localID {
		t.Fatalf("home ordering: %v", response)
	}
	other := items[1].(map[string]any)
	source := other["source"].(map[string]any)
	if source["serverId"] != peer.id.String() || source["serverUrl"] != remote.URL || other["posterUrl"] != remote.URL+"/api/v1/images/tmdb/w342/film.jpg" {
		t.Fatalf("source: %v", other)
	}
	if old := env.get(t, admin, "/api/v1/me/continue-watching")["items"].([]any); len(old) != 1 {
		t.Fatalf("old rail changed: %v", old)
	}
	peer.version = "99.0.0"
	response = env.get(t, admin, "/api/v1/me/home")
	if response["unavailableServers"] != float64(1) || len(response["continueWatching"].([]any)) != 1 {
		t.Fatalf("incompatible home: %v", response)
	}
}

func TestPeerHomeRequiresShadowUser(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	if response := do(t, env.h, http.MethodGet, "/api/v1/federation/home", admin, nil); response.status != http.StatusUnauthorized {
		t.Fatalf("origin session accepted: %d", response.status)
	}
	userID := uuid.New()
	peer.users = []peerUserDTO{{ID: userID, Username: "remote-viewer"}}
	if result := do(t, env.h, http.MethodPost, "/api/v1/admin/federation/peers/"+peer.id.String()+"/sync", admin, nil); result.status != http.StatusOK {
		t.Fatal(result.body)
	}
	token := forgeFederatedToken(peer.priv, federatedClaims{Issuer: peer.id, Subject: userID, Audience: env.srv.Identity.ID, IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix()})
	result := do(t, env.h, http.MethodGet, "/api/v1/federation/home", token, nil)
	if result.status != http.StatusOK || len(result.body["continueWatching"].([]any)) != 0 || result.body["unavailableServers"] != float64(0) {
		t.Fatalf("peer home: %d %v", result.status, result.body)
	}
}
