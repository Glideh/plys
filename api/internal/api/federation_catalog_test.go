package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"toto/api/internal/store"
)

func sourced(kind string, tmdb *int32, title string, serverID uuid.UUID) sourcedCard {
	return sourcedCard{
		federatedCard: federatedCard{
			itemCardDTO: itemCardDTO{ID: uuid.New(), Kind: kind, Title: title},
			TMDBID:      tmdb,
			SortTitle:   title,
			AddedAt:     time.Now(),
		},
		source: cardSourceDTO{ServerID: serverID, ServerName: "srv", ItemID: uuid.New()},
	}
}

func TestFederatedCatalogForwardsFiltersAndSkipsIncompatiblePeers(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	received := make(chan string, 2)
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/federation/catalog/kinds/movies/items" {
			received <- r.URL.Query().Get("filters")
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{{"id": uuid.New(), "kind": "movie", "title": "Remote title", "sortTitle": "remote title", "tmdbId": 603, "addedAt": time.Now()}}, "total": 1})
			return
		}
		peer.handler.ServeHTTP(w, r)
	}))
	t.Cleanup(remote.Close)
	env.srv.HTTPClient = remote.Client()
	if _, err := env.srv.q.UpdatePeerURL(context.Background(), store.UpdatePeerURLParams{ID: peer.id, Url: remote.URL}); err != nil {
		t.Fatal(err)
	}
	filters := `{"genres":[878],"ratingMin":7}`
	result := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items?filters="+url.QueryEscape(filters), admin, nil)
	if result.status != http.StatusOK || result.body["total"] != float64(1) {
		t.Fatalf("federated filters: %d %v", result.status, result.body)
	}
	select {
	case got := <-received:
		if got != filters {
			t.Fatalf("filters changed: %s", got)
		}
	default:
		t.Fatal("peer was not queried")
	}
	peer.version = "1.0.0"
	result = do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items", admin, nil)
	if result.status != http.StatusOK || result.body["total"] != float64(0) || result.body["unavailableServers"] != float64(1) {
		t.Fatalf("incompatible peer included: %d %v", result.status, result.body)
	}
	select {
	case <-received:
		t.Fatal("incompatible peer catalog was queried")
	default:
	}
}

func TestGroupCardsMergesVersionsOfSameTitle(t *testing.T) {
	id := int32(603)
	local, peer := uuid.New(), uuid.New()
	cards := []sourcedCard{
		sourced("movie", &id, "Matrix", local),
		sourced("movie", &id, "Matrix", peer),
		sourced("movie", nil, "Matrix", peer),
		sourced("series", &id, "Matrix", peer),
	}
	groups := groupCards(cards)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups (movie merged, untagged movie and series apart), got %d", len(groups))
	}
	if len(groups[0].Sources) != 2 || groups[0].Sources[0].ServerID != local {
		t.Fatalf("merged card: expected 2 sources with the first one primary, got %+v", groups[0].Sources)
	}
	if len(groups[1].Sources) != 1 || len(groups[2].Sources) != 1 {
		t.Fatal("untagged or other-kind cards must not be merged")
	}
}

func TestCompareCardsOrdersByTitleThenID(t *testing.T) {
	s := uuid.New()
	a := sourced("movie", nil, "Alien", s)
	b := sourced("movie", nil, "Brazil", s)
	if compareCards(a, b, "title") >= 0 {
		t.Fatal("Alien must sort before Brazil")
	}
	if compareCards(a, b, "title") != -compareCards(b, a, "title") {
		t.Fatal("comparison must be antisymmetric")
	}
}

func TestPeerKindItemsRejectsUnauthenticatedRequests(t *testing.T) {
	env, peer, _ := pairedEnv(t)
	const path = "/api/v1/federation/catalog/kinds/movies/items"

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	env.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("without token: expected 401, got %d", rec.Code)
	}

	// A correctly signed token for a user that is not a shadow of this peer must be refused.
	token := forgeFederatedToken(peer.priv, federatedClaims{
		Issuer: peer.id, Subject: uuid.New(), Audience: env.srv.Identity.ID,
		IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	req = httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	env.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown subject: expected 401, got %d", rec.Code)
	}
}

func TestFederatedFilterOptionsFallBackToEnglishForOlderPeers(t *testing.T) {
	env, peer, admin := pairedEnv(t)
	received := make(chan string, 4)
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/federation/catalog/kinds/movies/filters" {
			locale := r.URL.Query().Get("locale")
			received <- locale
			if locale != "fr" && locale != "en" {
				// A peer released before pt-BR support rejects the locale.
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"options": map[string]any{"languages": []map[string]any{{"id": "ja", "label": "Japanese"}}}})
			return
		}
		peer.handler.ServeHTTP(w, r)
	}))
	t.Cleanup(remote.Close)
	env.srv.HTTPClient = remote.Client()
	if _, err := env.srv.q.UpdatePeerURL(context.Background(), store.UpdatePeerURLParams{ID: peer.id, Url: remote.URL}); err != nil {
		t.Fatal(err)
	}
	result := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/filters?category=languages&locale=pt-BR", admin, nil)
	if result.status != http.StatusOK || result.body["unavailableServers"] != float64(0) {
		t.Fatalf("older peer must answer in English: %d %v", result.status, result.body)
	}
	if languages, _ := result.body["options"].(map[string]any)["languages"].([]any); len(languages) != 1 {
		t.Fatalf("peer suggestions missing: %v", result.body)
	}
	if first, second := <-received, <-received; first != "pt-BR" || second != "en" {
		t.Fatalf("locales sent to the peer: %s then %s", first, second)
	}
}
