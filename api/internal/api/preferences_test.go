package api

import (
	"net/http"
	"testing"
)

func TestAutoplayPreference(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)

	if me := e.get(t, admin, "/api/v1/me"); me["autoplayNext"] != false {
		t.Fatalf("autoplay must be off by default: %v", me)
	}
	r := do(t, e.h, http.MethodPatch, "/api/v1/me", admin, map[string]any{"autoplayNext": true})
	if r.status != http.StatusOK || r.body["autoplayNext"] != true {
		t.Fatalf("enable autoplay: %d %v", r.status, r.body)
	}
	if me := e.get(t, admin, "/api/v1/me"); me["autoplayNext"] != true {
		t.Errorf("the preference must persist: %v", me)
	}
	// An empty patch leaves the preference untouched.
	if r := do(t, e.h, http.MethodPatch, "/api/v1/me", admin, map[string]any{}); r.body["autoplayNext"] != true {
		t.Errorf("empty patch must not reset it: %v", r.body)
	}

	if r := do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "alice", "password": "password123"}); r.status != http.StatusCreated || r.body["autoplayNext"] != false {
		t.Fatalf("a new user starts with autoplay off: %d %v", r.status, r.body)
	}
	if r := do(t, e.h, http.MethodPatch, "/api/v1/me", "", map[string]any{"autoplayNext": true}); r.status != http.StatusUnauthorized {
		t.Errorf("preferences need authentication: %d", r.status)
	}
}

func TestLocalePreference(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)

	if r := do(t, e.h, http.MethodPatch, "/api/v1/me", admin, map[string]any{"locale": "en"}); r.status != http.StatusOK || r.body["locale"] != "en" {
		t.Fatalf("switch to en: %d %v", r.status, r.body)
	}
	if me := e.get(t, admin, "/api/v1/me"); me["locale"] != "en" {
		t.Errorf("the locale must persist: %v", me)
	}
	if r := do(t, e.h, http.MethodPatch, "/api/v1/me", admin, map[string]any{"locale": "de"}); r.status != http.StatusBadRequest {
		t.Errorf("unsupported locale must be rejected: %d %v", r.status, r.body)
	}
	if r := do(t, e.h, http.MethodPatch, "/api/v1/me", admin, map[string]any{"locale": ""}); r.status != http.StatusBadRequest {
		t.Errorf("empty locale must be rejected: %d %v", r.status, r.body)
	}
	if me := e.get(t, admin, "/api/v1/me"); me["locale"] != "en" {
		t.Errorf("a rejected update must not change the locale: %v", me)
	}
	if r := do(t, e.h, http.MethodPatch, "/api/v1/me", admin, map[string]any{"locale": "pt-br"}); r.status != http.StatusOK || r.body["locale"] != "pt-BR" {
		t.Errorf("pt-BR must be accepted case-insensitively and stored canonically: %d %v", r.status, r.body)
	}
	if r := do(t, e.h, http.MethodPatch, "/api/v1/me", admin, map[string]any{"locale": "pt"}); r.status != http.StatusBadRequest {
		t.Errorf("a bare pt locale must be rejected: %d %v", r.status, r.body)
	}
}
