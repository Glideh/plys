package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/auth"
	"toto/api/internal/database"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	resetSchema(t, pool)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	issuer := auth.NewIssuer([]byte("test-secret-test-secret-test-secret"), time.Minute)
	return New(pool, issuer, false).Routes()
}

func resetSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
}

type response struct {
	status int
	body   map[string]any
	header http.Header
}

func do(t *testing.T, h http.Handler, method, path, token string, body any, cookies ...*http.Cookie) response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := response{status: rec.Code, header: rec.Header()}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
	}
	return out
}

func setupAdmin(t *testing.T, h http.Handler) response {
	t.Helper()
	r := do(t, h, http.MethodPost, "/api/v1/setup/admin", "", map[string]any{"username": "admin", "password": "password123"})
	if r.status != http.StatusCreated {
		t.Fatalf("setup: %d %v", r.status, r.body)
	}
	return r
}

func TestSetupFlow(t *testing.T) {
	h := newTestServer(t)

	r := do(t, h, http.MethodGet, "/api/v1/setup/status", "", nil)
	if r.body["needsSetup"] != true {
		t.Fatalf("expected needsSetup=true, got %v", r.body)
	}

	r = setupAdmin(t, h)
	if r.body["accessToken"] == "" || r.body["refreshToken"] == "" {
		t.Fatalf("expected tokens, got %v", r.body)
	}

	r = do(t, h, http.MethodGet, "/api/v1/setup/status", "", nil)
	if r.body["needsSetup"] != false {
		t.Fatalf("expected needsSetup=false, got %v", r.body)
	}

	r = do(t, h, http.MethodPost, "/api/v1/setup/admin", "", map[string]any{"username": "other", "password": "password123"})
	if r.status != http.StatusConflict {
		t.Fatalf("second setup: expected 409, got %d", r.status)
	}
}

func TestSetupIsSingleUnderConcurrency(t *testing.T) {
	h := newTestServer(t)
	const n = 8
	statuses := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			body := map[string]any{"username": "admin" + string(rune('a'+i)), "password": "password123"}
			statuses[i] = do(t, h, http.MethodPost, "/api/v1/setup/admin", "", body).status
		})
	}
	wg.Wait()
	created := 0
	for _, s := range statuses {
		if s == http.StatusCreated {
			created++
		} else if s != http.StatusConflict {
			t.Errorf("unexpected status %d", s)
		}
	}
	if created != 1 {
		t.Fatalf("expected exactly one admin created, got %d", created)
	}
}

func TestLoginAndRefreshRotation(t *testing.T) {
	h := newTestServer(t)
	setupAdmin(t, h)

	r := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "ADMIN", "password": "wrong-password"})
	if r.status != http.StatusUnauthorized {
		t.Fatalf("wrong password: expected 401, got %d", r.status)
	}

	r = do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "ADMIN", "password": "password123", "deviceName": "test"})
	if r.status != http.StatusOK {
		t.Fatalf("login: %d %v", r.status, r.body)
	}
	first := r.body["refreshToken"].(string)

	r = do(t, h, http.MethodGet, "/api/v1/me", r.body["accessToken"].(string), nil)
	if r.status != http.StatusOK || r.body["username"] != "admin" {
		t.Fatalf("me: %d %v", r.status, r.body)
	}

	r = do(t, h, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": first})
	if r.status != http.StatusOK {
		t.Fatalf("refresh: %d %v", r.status, r.body)
	}
	second := r.body["refreshToken"].(string)
	if second == first {
		t.Fatal("refresh token must rotate")
	}

	// Replaying the rotated token revokes the whole family, including the latest token.
	if r = do(t, h, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": first}); r.status != http.StatusUnauthorized {
		t.Fatalf("reuse: expected 401, got %d", r.status)
	}
	if r = do(t, h, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": second}); r.status != http.StatusUnauthorized {
		t.Fatalf("after reuse: expected 401, got %d", r.status)
	}
}

func TestCookieDelivery(t *testing.T) {
	h := newTestServer(t)
	setupAdmin(t, h)

	r := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "admin", "password": "password123", "tokenDelivery": "cookie"})
	if _, ok := r.body["refreshToken"]; ok {
		t.Fatal("refresh token must not be in body when delivered by cookie")
	}
	cookie := findCookie(r.header, refreshCookieName)
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("expected HttpOnly SameSite=Strict cookie, got %+v", cookie)
	}

	r = do(t, h, http.MethodPost, "/api/v1/auth/refresh", "", nil, cookie)
	if r.status != http.StatusOK || findCookie(r.header, refreshCookieName) == nil {
		t.Fatalf("cookie refresh: %d %v", r.status, r.body)
	}

	r = do(t, h, http.MethodPost, "/api/v1/auth/logout", "", nil, findCookie(r.header, refreshCookieName))
	if r.status != http.StatusNoContent {
		t.Fatalf("logout: %d", r.status)
	}
}

func TestUserAdministration(t *testing.T) {
	h := newTestServer(t)
	admin := setupAdmin(t, h)
	adminToken := admin.body["accessToken"].(string)
	adminID := admin.body["user"].(map[string]any)["id"].(string)

	r := do(t, h, http.MethodPost, "/api/v1/users", adminToken, map[string]any{"username": "bob", "password": "password123"})
	if r.status != http.StatusCreated || r.body["role"] != auth.RoleUser {
		t.Fatalf("create user: %d %v", r.status, r.body)
	}
	bobID := r.body["id"].(string)

	if r = do(t, h, http.MethodPost, "/api/v1/users", adminToken, map[string]any{"username": "BOB", "password": "password123"}); r.status != http.StatusConflict {
		t.Fatalf("duplicate username: expected 409, got %d", r.status)
	}

	bob := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"})
	if r = do(t, h, http.MethodGet, "/api/v1/users", bob.body["accessToken"].(string), nil); r.status != http.StatusForbidden {
		t.Fatalf("non-admin list users: expected 403, got %d", r.status)
	}

	if r = do(t, h, http.MethodPatch, "/api/v1/users/"+adminID, adminToken, map[string]any{"role": "user"}); r.status != http.StatusConflict {
		t.Fatalf("demote last admin: expected 409, got %d", r.status)
	}
	if r = do(t, h, http.MethodDelete, "/api/v1/users/"+adminID, adminToken, nil); r.status != http.StatusConflict {
		t.Fatalf("delete self: expected 409, got %d", r.status)
	}

	if r = do(t, h, http.MethodPatch, "/api/v1/users/"+bobID, adminToken, map[string]any{"password": "newpassword"}); r.status != http.StatusOK {
		t.Fatalf("change password: %d %v", r.status, r.body)
	}
	if r = do(t, h, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": bob.body["refreshToken"]}); r.status != http.StatusUnauthorized {
		t.Fatalf("password change must revoke sessions, got %d", r.status)
	}

	if r = do(t, h, http.MethodDelete, "/api/v1/users/"+bobID, adminToken, nil); r.status != http.StatusNoContent {
		t.Fatalf("delete user: %d", r.status)
	}
}

func findCookie(h http.Header, name string) *http.Cookie {
	for _, c := range (&http.Response{Header: h}).Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}
