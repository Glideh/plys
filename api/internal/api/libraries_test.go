package api

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func mkdirs(t *testing.T, root string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLibraryLifecycle(t *testing.T) {
	h, root := newTestServerWithMedia(t)
	mkdirs(t, root, "Films/HD", "Films/4K", "Series")
	admin := setupAdmin(t, h).body["accessToken"].(string)
	films := filepath.Join(root, "Films")

	r := do(t, h, http.MethodPost, "/api/v1/libraries", admin, map[string]any{
		"name": " Films ", "kind": "movies", "paths": []string{films, films + "/"}, "restricted": false,
	})
	if r.status != http.StatusCreated {
		t.Fatalf("create: %d %v", r.status, r.body)
	}
	id := r.body["id"].(string)
	if r.body["name"] != "Films" || len(r.body["paths"].([]any)) != 1 {
		t.Fatalf("expected trimmed name and deduplicated paths, got %v", r.body)
	}

	conflicts := []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{"nested path", map[string]any{"name": "HD", "kind": "movies", "paths": []string{filepath.Join(films, "HD")}}, http.StatusConflict, "path_conflict"},
		{"parent path", map[string]any{"name": "All", "kind": "other", "paths": []string{root}}, http.StatusConflict, "path_conflict"},
		{"duplicate name", map[string]any{"name": "FILMS", "kind": "series", "paths": []string{filepath.Join(root, "Series")}}, http.StatusConflict, "name_taken"},
		{"outside roots", map[string]any{"name": "Etc", "kind": "other", "paths": []string{"/etc"}}, http.StatusBadRequest, "validation"},
		{"traversal", map[string]any{"name": "Up", "kind": "other", "paths": []string{root + "/../.."}}, http.StatusBadRequest, "validation"},
		{"bad kind", map[string]any{"name": "X", "kind": "music", "paths": []string{filepath.Join(root, "Series")}}, http.StatusBadRequest, "validation"},
		{"no paths", map[string]any{"name": "X", "kind": "series", "paths": []string{}}, http.StatusBadRequest, "validation"},
	}
	for _, c := range conflicts {
		r := do(t, h, http.MethodPost, "/api/v1/libraries", admin, c.body)
		if r.status != c.status || r.body["error"].(map[string]any)["code"] != c.code {
			t.Errorf("%s: got %d %v, want %d %s", c.name, r.status, r.body, c.status, c.code)
		}
	}

	r = do(t, h, http.MethodPatch, "/api/v1/libraries/"+id, admin, map[string]any{
		"name": "Movies", "paths": []string{filepath.Join(films, "HD"), filepath.Join(films, "4K")},
	})
	if r.status != http.StatusOK || r.body["name"] != "Movies" || len(r.body["paths"].([]any)) != 2 {
		t.Fatalf("update: %d %v", r.status, r.body)
	}

	if r = do(t, h, http.MethodPost, "/api/v1/libraries/"+id+"/scan", admin, nil); r.status != http.StatusAccepted || r.body["state"] != "queued" {
		t.Fatalf("scan: %d %v", r.status, r.body)
	}
	if r = do(t, h, http.MethodGet, "/api/v1/admin/scans", admin, nil); r.status != http.StatusOK || len(r.body["items"].([]any)) != 1 {
		t.Fatalf("scans: %d %v", r.status, r.body)
	}

	r = do(t, h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"})
	bob := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)
	r = do(t, h, http.MethodGet, "/api/v1/libraries", bob, nil)
	items := r.body["items"].([]any)
	if r.status != http.StatusOK || len(items) != 1 {
		t.Fatalf("list as user: %d %v", r.status, r.body)
	}
	if _, ok := items[0].(map[string]any)["paths"]; ok {
		t.Error("paths must not be exposed to non-admin users")
	}
	if r = do(t, h, http.MethodPost, "/api/v1/libraries", bob, map[string]any{}); r.status != http.StatusForbidden {
		t.Errorf("create as user: expected 403, got %d", r.status)
	}

	if r = do(t, h, http.MethodDelete, "/api/v1/libraries/"+id, admin, nil); r.status != http.StatusNoContent {
		t.Fatalf("delete: %d", r.status)
	}
	if r = do(t, h, http.MethodGet, "/api/v1/libraries/"+id, admin, nil); r.status != http.StatusNotFound {
		t.Fatalf("get deleted: expected 404, got %d", r.status)
	}
}

func TestBrowseFS(t *testing.T) {
	h, root := newTestServerWithMedia(t)
	mkdirs(t, root, "Films", "Series", ".trash")
	admin := setupAdmin(t, h).body["accessToken"].(string)

	r := do(t, h, http.MethodGet, "/api/v1/admin/fs/browse", admin, nil)
	if r.status != http.StatusOK || r.body["path"] != nil || len(r.body["entries"].([]any)) != 1 {
		t.Fatalf("roots: %d %v", r.status, r.body)
	}

	r = do(t, h, http.MethodGet, "/api/v1/admin/fs/browse?path="+url.QueryEscape(root), admin, nil)
	if r.status != http.StatusOK || r.body["parent"] != nil || len(r.body["entries"].([]any)) != 2 {
		t.Fatalf("root listing: %d %v", r.status, r.body)
	}

	r = do(t, h, http.MethodGet, "/api/v1/admin/fs/browse?path="+url.QueryEscape(filepath.Join(root, "Films")), admin, nil)
	if r.status != http.StatusOK || r.body["parent"] != root {
		t.Fatalf("subdir listing: %d %v", r.status, r.body)
	}

	for _, p := range []string{"/", "/etc", root + "/../"} {
		if r = do(t, h, http.MethodGet, "/api/v1/admin/fs/browse?path="+url.QueryEscape(p), admin, nil); r.status != http.StatusBadRequest {
			t.Errorf("browse %q: expected 400, got %d", p, r.status)
		}
	}
}

func TestLibraryMetadataLanguage(t *testing.T) {
	h, root := newTestServerWithMedia(t)
	mkdirs(t, root, "Films", "Series")
	admin := setupAdmin(t, h).body["accessToken"].(string)

	r := do(t, h, http.MethodPost, "/api/v1/libraries", admin, map[string]any{
		"name": "Films", "kind": "movies", "paths": []string{filepath.Join(root, "Films")},
	})
	if r.status != http.StatusCreated || r.body["metadataLanguage"] != nil {
		t.Fatalf("a library follows the server language by default: %d %v", r.status, r.body)
	}
	id := r.body["id"].(string)

	r = do(t, h, http.MethodPost, "/api/v1/libraries", admin, map[string]any{
		"name": "Séries", "kind": "series", "paths": []string{filepath.Join(root, "Series")}, "metadataLanguage": "pt-BR",
	})
	if r.status != http.StatusCreated || r.body["metadataLanguage"] != "pt-BR" {
		t.Fatalf("create with an override: %d %v", r.status, r.body)
	}

	if r = do(t, h, http.MethodPatch, "/api/v1/libraries/"+id, admin, map[string]any{"metadataLanguage": "english"}); r.status != http.StatusBadRequest {
		t.Fatalf("invalid language: expected 400, got %d", r.status)
	}
	r = do(t, h, http.MethodPatch, "/api/v1/libraries/"+id, admin, map[string]any{"metadataLanguage": "en-US"})
	if r.status != http.StatusOK || r.body["metadataLanguage"] != "en-US" {
		t.Fatalf("set override: %d %v", r.status, r.body)
	}
	r = do(t, h, http.MethodPatch, "/api/v1/libraries/"+id, admin, map[string]any{"name": "Movies"})
	if r.status != http.StatusOK || r.body["metadataLanguage"] != "en-US" {
		t.Fatalf("an omitted language must be kept: %d %v", r.status, r.body)
	}
	if r = do(t, h, http.MethodGet, "/api/v1/libraries/"+id, admin, nil); r.body["metadataLanguage"] != "en-US" {
		t.Fatalf("get: %v", r.body)
	}
	r = do(t, h, http.MethodPatch, "/api/v1/libraries/"+id, admin, map[string]any{"metadataLanguage": ""})
	if r.status != http.StatusOK || r.body["metadataLanguage"] != nil {
		t.Fatalf("an empty language removes the override: %d %v", r.status, r.body)
	}
}
