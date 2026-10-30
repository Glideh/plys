package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func writeFiles(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func (e testEnv) createScannedLibrary(t *testing.T, token, name, kind, dir string, extra map[string]any) string {
	t.Helper()
	body := map[string]any{"name": name, "kind": kind, "paths": []string{filepath.Join(e.root, dir)}}
	for k, v := range extra {
		body[k] = v
	}
	r := do(t, e.h, http.MethodPost, "/api/v1/libraries", token, body)
	if r.status != http.StatusCreated {
		t.Fatalf("create library: %d %v", r.status, r.body)
	}
	id := r.body["id"].(string)
	if err := e.scan.ScanLibrary(context.Background(), uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCatalogBrowsing(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root,
		"films/Alien.1979.mkv",
		"films/Blade.Runner.1982.mkv",
		"films/Blade.Runner.1982.Final.Cut.mkv",
		"films/[ygg]Guerre et Paix.1966.DVD 1.MULTI/Guerre et Paix.avi",
		"films/[ygg]Guerre et Paix.1966.DVD 2.MULTI/Guerre et Paix.avi",
		"series/Show.S01E01.mkv",
		"series/Show.S01E02.mkv",
		"series/Show.S02E01.mkv",
	)
	films := e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	series := e.createScannedLibrary(t, admin, "Séries", "series", "series", nil)

	r := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items?sort=year", admin, nil)
	items := r.body["items"].([]any)
	if r.status != http.StatusOK || len(items) != 3 || r.body["total"].(float64) != 3 {
		t.Fatalf("list films: %d %v", r.status, r.body)
	}
	if first := items[0].(map[string]any); first["title"] != "Blade Runner" {
		t.Errorf("sort=year must put the most recent first, got %v", first["title"])
	}
	r = do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items?query=alien", admin, nil)
	if len(r.body["items"].([]any)) != 1 {
		t.Errorf("query must filter by title: %v", r.body)
	}
	if r = do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items?limit=1&offset=1", admin, nil); len(r.body["items"].([]any)) != 1 || r.body["total"].(float64) != 3 {
		t.Errorf("pagination: %v", r.body)
	}
	if r = do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items?sort=drop", admin, nil); r.status != http.StatusBadRequest {
		t.Errorf("invalid sort: expected 400, got %d", r.status)
	}

	byTitle := map[string]string{}
	for _, it := range do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items", admin, nil).body["items"].([]any) {
		m := it.(map[string]any)
		byTitle[m["title"].(string)] = m["id"].(string)
	}

	blade := do(t, e.h, http.MethodGet, "/api/v1/items/"+byTitle["Blade Runner"], admin, nil).body
	versions := blade["versions"].([]any)
	if len(versions) != 2 {
		t.Fatalf("Blade Runner must expose two versions: %v", blade)
	}
	labels := map[string]bool{}
	for _, v := range versions {
		vm := v.(map[string]any)
		labels[vm["label"].(string)] = true
		if len(vm["audio"].([]any)) != 2 || len(vm["subtitles"].([]any)) != 1 {
			t.Errorf("tracks missing: %v", vm)
		}
		if _, leaked := vm["path"]; leaked {
			t.Error("file paths must not be exposed")
		}
	}
	if !labels["1080p · HDR · HEVC"] || !labels["1080p · HDR · HEVC · Final Cut"] {
		t.Errorf("unexpected version labels: %v", labels)
	}

	war := do(t, e.h, http.MethodGet, "/api/v1/items/"+byTitle["Guerre et Paix"], admin, nil).body
	if parts := war["children"].([]any); len(parts) != 2 || parts[0].(map[string]any)["partNumber"].(float64) != 1 {
		t.Errorf("Guerre et Paix must list its 2 parts: %v", war["children"])
	}

	show := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+series+"/items", admin, nil).body["items"].([]any)[0].(map[string]any)
	if show["episodeCount"].(float64) != 3 {
		t.Errorf("series card must count episodes: %v", show)
	}
	seasons := do(t, e.h, http.MethodGet, "/api/v1/items/"+show["id"].(string), admin, nil).body["children"].([]any)
	if len(seasons) != 2 {
		t.Fatalf("expected 2 seasons, got %v", seasons)
	}
	s1 := do(t, e.h, http.MethodGet, "/api/v1/items/"+seasons[0].(map[string]any)["id"].(string), admin, nil).body
	episodes := s1["children"].([]any)
	if len(episodes) != 2 || len(s1["ancestors"].([]any)) != 1 {
		t.Fatalf("season 1: %v", s1)
	}
	ep := do(t, e.h, http.MethodGet, "/api/v1/items/"+episodes[1].(map[string]any)["id"].(string), admin, nil).body
	if ep["episodeNumber"].(float64) != 2 || len(ep["ancestors"].([]any)) != 2 || len(ep["versions"].([]any)) != 1 {
		t.Errorf("episode 2: %v", ep)
	}
}

func TestLibraryAccessControl(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "kids/Bambi.1942.mkv", "adults/Alien.1979.mkv")

	bobID := do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"}).body["id"].(string)
	do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "carol", "password": "password123"})
	bob := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)
	carol := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "carol", "password": "password123"}).body["accessToken"].(string)

	e.createScannedLibrary(t, admin, "Enfants", "movies", "kids", nil)
	adults := e.createScannedLibrary(t, admin, "Adultes", "movies", "adults", map[string]any{"restricted": true, "userIds": []string{bobID}})

	visible := func(token string) int {
		return len(do(t, e.h, http.MethodGet, "/api/v1/libraries", token, nil).body["items"].([]any))
	}
	if visible(admin) != 2 || visible(bob) != 2 || visible(carol) != 1 {
		t.Fatalf("visibility: admin=%d bob=%d carol=%d", visible(admin), visible(bob), visible(carol))
	}

	alien := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+adults+"/items", bob, nil).body["items"].([]any)[0].(map[string]any)["id"].(string)
	for _, p := range []string{"/api/v1/libraries/" + adults, "/api/v1/libraries/" + adults + "/items", "/api/v1/items/" + alien} {
		if r := do(t, e.h, http.MethodGet, p, carol, nil); r.status != http.StatusNotFound {
			t.Errorf("carol %s: expected 404, got %d", p, r.status)
		}
		if r := do(t, e.h, http.MethodGet, p, bob, nil); r.status != http.StatusOK {
			t.Errorf("bob %s: expected 200, got %d", p, r.status)
		}
	}

	r := do(t, e.h, http.MethodPatch, "/api/v1/libraries/"+adults, admin, map[string]any{"restricted": false})
	if r.status != http.StatusOK || r.body["restricted"] != false || len(r.body["userIds"].([]any)) != 1 {
		t.Fatalf("unrestrict must keep the user list: %d %v", r.status, r.body)
	}
	if visible(carol) != 2 {
		t.Error("carol must see the library once it is no longer restricted")
	}
	if r = do(t, e.h, http.MethodPatch, "/api/v1/libraries/"+adults, admin, map[string]any{"userIds": []string{uuid.NewString()}}); r.status != http.StatusBadRequest {
		t.Errorf("unknown user: expected 400, got %d", r.status)
	}
}
