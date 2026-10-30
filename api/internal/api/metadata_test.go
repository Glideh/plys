package api

import (
	"net/http"
	"testing"
)

func TestServerInfoIsPublic(t *testing.T) {
	h := newTestServer(t)
	r := do(t, h, http.MethodGet, "/api/v1/server", "", nil)
	if r.status != http.StatusOK || r.body["id"] == "" || r.body["publicKey"] == "" || r.body["version"] != Version {
		t.Fatalf("server info: %d %v", r.status, r.body)
	}
	again := do(t, h, http.MethodGet, "/api/v1/server", "", nil)
	if again.body["id"] != r.body["id"] {
		t.Fatal("server identity must be stable")
	}
}

func TestSettings(t *testing.T) {
	h := newTestServer(t)
	admin := setupAdmin(t, h).body["accessToken"].(string)

	r := do(t, h, http.MethodGet, "/api/v1/admin/settings", admin, nil)
	if r.status != http.StatusOK || r.body["tmdbConfigured"] != false || r.body["metadataLanguage"] != "fr-FR" {
		t.Fatalf("defaults: %d %v", r.status, r.body)
	}
	if r = do(t, h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"metadataLanguage": "english"}); r.status != http.StatusBadRequest {
		t.Fatalf("invalid language: expected 400, got %d", r.status)
	}
	r = do(t, h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"metadataLanguage": "en-US"})
	if r.status != http.StatusOK || r.body["metadataLanguage"] != "en-US" {
		t.Fatalf("update language: %d %v", r.status, r.body)
	}
	if _, leaked := r.body["tmdbApiKey"]; leaked {
		t.Fatal("the TMDB key must never be returned")
	}

	do(t, h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"})
	bob := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)
	if r = do(t, h, http.MethodGet, "/api/v1/admin/settings", bob, nil); r.status != http.StatusForbidden {
		t.Fatalf("settings as user: expected 403, got %d", r.status)
	}
}

func TestImagesRefuseUnknownPaths(t *testing.T) {
	h := newTestServer(t)
	for _, p := range []string{
		"/api/v1/images/tmdb/w342/unknown.jpg",
		"/api/v1/images/tmdb/w9999/unknown.jpg",
		"/api/v1/images/tmdb/w342/..%2Fsecret.jpg",
		"/api/v1/images/local/../../etc/passwd",
		"/api/v1/images/local/00000000-0000-0000-0000-000000000000.jpg",
	} {
		if r := do(t, h, http.MethodGet, p, "", nil); r.status != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d", p, r.status)
		}
	}
}
