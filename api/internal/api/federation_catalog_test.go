package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
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
