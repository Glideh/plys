package api

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// newInvitation creates an invitation as admin and returns its token and PIN.
func newInvitation(t *testing.T, env testEnv, admin string) (id, token, pin string) {
	t.Helper()
	r := do(t, env.h, http.MethodPost, "/api/v1/invitations", admin, map[string]any{"locale": "en"})
	if r.status != http.StatusCreated {
		t.Fatalf("create invitation: %d %v", r.status, r.body)
	}
	url := r.body["url"].(string)
	prefix := "https://plys.example/invite#"
	if !strings.HasPrefix(url, prefix) || len(url) <= len(prefix) {
		t.Fatalf("invitation url: %q", url)
	}
	pin = r.body["pin"].(string)
	if !regexp.MustCompile(`^\d{4}$`).MatchString(pin) {
		t.Fatalf("pin: %q", pin)
	}
	if r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control: %q", r.header.Get("Cache-Control"))
	}
	return r.body["id"].(string), strings.TrimPrefix(url, prefix), pin
}

func wrongPIN(pin string) string {
	if pin == "0000" {
		return "1111"
	}
	return "0000"
}

func TestInvitationFlow(t *testing.T) {
	env := newTestEnv(t)
	admin := setupAdmin(t, env.h).body["accessToken"].(string)
	// The tests come from a single address: forget its attempts between steps.
	resetIP := func() { env.srv.invitationIPs.reset("192.0.2.1") }

	if r := do(t, env.h, http.MethodPost, "/api/v1/invitations", admin, nil); r.status != http.StatusConflict || r.body["error"].(map[string]any)["code"] != "public_url_required" {
		t.Fatalf("without public URL: %d %v", r.status, r.body)
	}
	if r := do(t, env.h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"publicUrl": "https://plys.example"}); r.status != http.StatusOK {
		t.Fatalf("settings: %d %v", r.status, r.body)
	}

	do(t, env.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"})
	bob := do(t, env.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)
	if r := do(t, env.h, http.MethodPost, "/api/v1/invitations", bob, nil); r.status != http.StatusForbidden {
		t.Fatalf("invitation by a user: %d", r.status)
	}

	id, token, pin := newInvitation(t, env, admin)
	items := env.get(t, admin, "/api/v1/invitations")["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"] != id || items[0].(map[string]any)["pin"] != nil {
		t.Fatalf("pending invitations: %v", items)
	}

	r := do(t, env.h, http.MethodPost, "/api/v1/invitations/lookup", "", map[string]any{"token": token})
	if r.status != http.StatusOK || r.body["locale"] != "en" || r.header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("lookup: %d %v %v", r.status, r.body, r.header)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/invitations/lookup", "", map[string]any{"token": token + "x"}); r.status != http.StatusNotFound {
		t.Fatalf("lookup with a wrong token: %d", r.status)
	}

	accept := map[string]any{"token": token, "pin": wrongPIN(pin), "username": "carol", "password": "password123", "locale": "pt-br", "tokenDelivery": "body"}
	if r := do(t, env.h, http.MethodPost, "/api/v1/invitations/accept", "", accept); r.status != http.StatusForbidden || r.body["error"].(map[string]any)["code"] != "invalid_pin" {
		t.Fatalf("wrong pin: %d %v", r.status, r.body)
	}
	accept["pin"] = pin
	accept["username"] = "BOB"
	if r := do(t, env.h, http.MethodPost, "/api/v1/invitations/accept", "", accept); r.status != http.StatusConflict {
		t.Fatalf("taken username: %d %v", r.status, r.body)
	}
	accept["username"] = "carol"
	r = do(t, env.h, http.MethodPost, "/api/v1/invitations/accept", "", accept)
	if r.status != http.StatusCreated || r.body["refreshToken"] == nil {
		t.Fatalf("accept: %d %v", r.status, r.body)
	}
	user := r.body["user"].(map[string]any)
	if user["username"] != "carol" || user["role"] != "user" || user["locale"] != "pt-BR" {
		t.Fatalf("invited user: %v", user)
	}
	if me := env.get(t, r.body["accessToken"].(string), "/api/v1/me"); me["username"] != "carol" {
		t.Fatalf("me: %v", me)
	}

	// Single use: the invitation is gone once accepted.
	resetIP()
	accept["username"] = "dave"
	if r := do(t, env.h, http.MethodPost, "/api/v1/invitations/accept", "", accept); r.status != http.StatusNotFound || r.body["error"].(map[string]any)["code"] != "invitation_invalid" {
		t.Fatalf("reuse: %d %v", r.status, r.body)
	}
	if items := env.get(t, admin, "/api/v1/invitations")["items"].([]any); len(items) != 0 {
		t.Fatalf("pending after accept: %v", items)
	}

	// Revoked by the admin.
	id, token, _ = newInvitation(t, env, admin)
	if r := do(t, env.h, http.MethodDelete, "/api/v1/invitations/"+id, admin, nil); r.status != http.StatusNoContent {
		t.Fatalf("delete: %d", r.status)
	}
	if r := do(t, env.h, http.MethodDelete, "/api/v1/invitations/"+id, admin, nil); r.status != http.StatusNotFound {
		t.Fatalf("delete twice: %d", r.status)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/invitations/lookup", "", map[string]any{"token": token}); r.status != http.StatusNotFound {
		t.Fatalf("lookup after delete: %d", r.status)
	}

	// Expired.
	id, token, pin = newInvitation(t, env, admin)
	if _, err := env.srv.Pool.Exec(context.Background(), "UPDATE invitations SET expires_at = now() - interval '1 second' WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	resetIP()
	accept["token"], accept["pin"] = token, pin
	if r := do(t, env.h, http.MethodPost, "/api/v1/invitations/accept", "", accept); r.status != http.StatusNotFound {
		t.Fatalf("expired: %d %v", r.status, r.body)
	}
	if items := env.get(t, admin, "/api/v1/invitations")["items"].([]any); len(items) != 0 {
		t.Fatalf("expired invitation listed: %v", items)
	}
}

func TestInvitationRevokedAfterWrongPINs(t *testing.T) {
	env := newTestEnv(t)
	admin := setupAdmin(t, env.h).body["accessToken"].(string)
	do(t, env.h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"publicUrl": "https://plys.example"})
	_, token, pin := newInvitation(t, env, admin)

	accept := map[string]any{"token": token, "pin": wrongPIN(pin), "username": "carol", "password": "password123"}
	for i := range maxInvitationFailures {
		want := http.StatusForbidden
		if i == maxInvitationFailures-1 {
			want = http.StatusNotFound
		}
		if r := do(t, env.h, http.MethodPost, "/api/v1/invitations/accept", "", accept); r.status != want {
			t.Fatalf("attempt %d: %d %v", i+1, r.status, r.body)
		}
	}
	accept["pin"] = pin
	if r := do(t, env.h, http.MethodPost, "/api/v1/invitations/accept", "", accept); r.status != http.StatusNotFound {
		t.Fatalf("right pin after revocation: %d %v", r.status, r.body)
	}
}

func TestInvitationRateLimitedByAddress(t *testing.T) {
	env := newTestEnv(t)
	for range invitationIPLimit {
		if r := do(t, env.h, http.MethodPost, "/api/v1/invitations/lookup", "", map[string]any{"token": "nope"}); r.status != http.StatusNotFound {
			t.Fatalf("lookup: %d", r.status)
		}
	}
	r := do(t, env.h, http.MethodPost, "/api/v1/invitations/lookup", "", map[string]any{"token": "nope"})
	if r.status != http.StatusTooManyRequests || r.header.Get("Retry-After") == "" {
		t.Fatalf("throttled lookup: %d %v", r.status, r.header)
	}
}
