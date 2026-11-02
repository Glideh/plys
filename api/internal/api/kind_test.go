package api

import (
	"net/http"
	"testing"
)

func TestKindBrowsing(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "kids/Bambi.1942.mkv", "adults/Alien.1979.mkv", "series/Show.S01E01.mkv")

	bobID := do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"}).body["id"].(string)
	do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "carol", "password": "password123"})
	bob := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)
	carol := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "carol", "password": "password123"}).body["accessToken"].(string)

	kids := e.createScannedLibrary(t, admin, "Enfants", "movies", "kids", nil)
	e.createScannedLibrary(t, admin, "Adultes", "movies", "adults", map[string]any{"restricted": true, "userIds": []string{bobID}})
	e.createScannedLibrary(t, admin, "Séries", "series", "series", nil)

	total := func(token, path string) float64 {
		t.Helper()
		r := do(t, e.h, http.MethodGet, "/api/v1/kinds/"+path, token, nil)
		if r.status != http.StatusOK {
			t.Fatalf("%s: %d %v", path, r.status, r.body)
		}
		return r.body["total"].(float64)
	}

	if n := total(admin, "movies/items"); n != 2 {
		t.Errorf("admin movies: expected 2, got %v", n)
	}
	if n := total(admin, "series/items"); n != 1 {
		t.Errorf("admin series: expected 1, got %v", n)
	}
	if n := total(admin, "movies/items?library="+kids); n != 1 {
		t.Errorf("library filter: expected 1, got %v", n)
	}
	if n := total(bob, "movies/items"); n != 2 {
		t.Errorf("bob movies: expected 2, got %v", n)
	}
	if n := total(carol, "movies/items"); n != 1 {
		t.Errorf("restricted library must be hidden from carol: expected 1, got %v", n)
	}

	if r := do(t, e.h, http.MethodGet, "/api/v1/kinds/bogus/items", admin, nil); r.status != http.StatusBadRequest {
		t.Errorf("unknown kind: expected 400, got %d", r.status)
	}
	if r := do(t, e.h, http.MethodGet, "/api/v1/kinds/movies/items?library=nope", admin, nil); r.status != http.StatusBadRequest {
		t.Errorf("invalid library: expected 400, got %d", r.status)
	}
}
