package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"toto/api/internal/metadata"
	"toto/api/internal/release"
)

func TestFanKaiEpisodeThumbnailIsServedFromCache(t *testing.T) {
	dir := t.TempDir()
	cache := metadata.NewImageCache(dir, "")
	file := "00000000-0000-0000-0000-000000000001-501-thumb.png"
	if err := os.MkdirAll(filepath.Join(dir, "fankai"), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("\x89PNG\r\n\x1a\n")
	if err := os.WriteFile(cache.FanKaiLocal(file), data, 0o644); err != nil {
		t.Fatal(err)
	}
	path := "fankai/" + file
	u := imageURL(&path, "w500")
	if u == nil || *u != "/api/v1/images/fankai/"+file {
		t.Fatalf("unexpected episode image URL: %v", u)
	}
	h := New(Deps{Images: cache}).Routes()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, *u, nil))
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("cached thumbnail not served: %d %s", w.Code, w.Header())
	}
}

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

func TestServerInfoReportsUpdate(t *testing.T) {
	releases := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"99.0.0","html_url":"https://github.com/Glideh/plys/releases/tag/99.0.0"}`))
	}))
	defer releases.Close()
	// Version is 0.2.0-dev in tests, which never sees an update: the checker is given a release version instead.
	checker := release.New(releases.URL, "0.7.9")
	if err := checker.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	e := newTestEnvWith(t, func(d *Deps) { d.Releases = checker })
	r := do(t, e.h, http.MethodGet, "/api/v1/server", "", nil)
	update, _ := r.body["update"].(map[string]any)
	if r.status != http.StatusOK || update["version"] != "99.0.0" || update["url"] != "https://github.com/Glideh/plys/releases/tag/99.0.0" {
		t.Fatalf("server info: %d %v", r.status, r.body)
	}

	if info := do(t, newTestServer(t), http.MethodGet, "/api/v1/server", "", nil); info.body["update"] != nil {
		t.Fatalf("update without a checker: %v", info.body)
	}
}

func TestServerIdentitySettings(t *testing.T) {
	h := newTestServer(t)
	admin := setupAdmin(t, h).body["accessToken"].(string)

	if r := do(t, h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"publicUrl": "ftp://plys.example.org"}); r.status != http.StatusBadRequest {
		t.Fatalf("non-http URL: expected 400, got %d", r.status)
	}
	if r := do(t, h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"publicUrl": "https://plys.example.org/library"}); r.status != http.StatusBadRequest {
		t.Fatalf("URL with path: expected 400, got %d", r.status)
	}
	if r := do(t, h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"serverName": "   "}); r.status != http.StatusBadRequest {
		t.Fatalf("blank name: expected 400, got %d", r.status)
	}

	r := do(t, h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"serverName": " Salon ", "publicUrl": "https://plys.example.org/"})
	if r.status != http.StatusOK || r.body["serverName"] != "Salon" || r.body["publicUrl"] != "https://plys.example.org" {
		t.Fatalf("update identity: %d %v", r.status, r.body)
	}
	if info := do(t, h, http.MethodGet, "/api/v1/server", "", nil); info.body["name"] != "Salon" {
		t.Fatalf("server info name: %v", info.body)
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
