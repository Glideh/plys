package api

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func TestDeviceLoginFlow(t *testing.T) {
	env := newTestEnv(t)
	admin := setupAdmin(t, env.h).body["accessToken"].(string)
	do(t, env.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"})
	bob := do(t, env.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)

	start := do(t, env.h, http.MethodPost, "/api/v1/auth/device", "", map[string]any{"deviceName": "Living room TV"})
	if start.status != http.StatusCreated || start.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("start: %d %v", start.status, start.body)
	}
	deviceCode := start.body["deviceCode"].(string)
	userCode := start.body["userCode"].(string)
	if !regexp.MustCompile(`^[BCDFGHJKLMNPQRSTVWXZ]{4}-[BCDFGHJKLMNPQRSTVWXZ]{4}$`).MatchString(userCode) {
		t.Fatalf("user code: %q", userCode)
	}
	if start.body["verificationUrl"] != "" || start.body["interval"].(float64) <= 0 {
		t.Fatalf("without public URL: %v", start.body)
	}

	poll := func() response {
		return do(t, env.h, http.MethodPost, "/api/v1/auth/device/token", "", map[string]any{"deviceCode": deviceCode})
	}
	if r := poll(); r.status != http.StatusConflict || errorCode(r.body) != "authorization_pending" {
		t.Fatalf("pending: %d %v", r.status, r.body)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/device-logins/lookup", "", map[string]any{"userCode": userCode}); r.status != http.StatusUnauthorized {
		t.Fatalf("lookup without an account: %d", r.status)
	}

	// The code is accepted in lower case and without its dash, as typed on a phone.
	typed := strings.ToLower(strings.ReplaceAll(userCode, "-", ""))
	r := do(t, env.h, http.MethodPost, "/api/v1/device-logins/lookup", bob, map[string]any{"userCode": typed})
	if r.status != http.StatusOK || r.body["deviceName"] != "Living room TV" || r.body["userCode"] != userCode {
		t.Fatalf("lookup: %d %v", r.status, r.body)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/device-logins/approve", bob, map[string]any{"userCode": "BCDF-GHJK0"}); r.status != http.StatusNotFound || errorCode(r.body) != "device_login_invalid" {
		t.Fatalf("approve a malformed code: %d %v", r.status, r.body)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/device-logins/approve", bob, map[string]any{"userCode": typed}); r.status != http.StatusNoContent {
		t.Fatalf("approve: %d %v", r.status, r.body)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/device-logins/approve", admin, map[string]any{"userCode": userCode}); r.status != http.StatusNotFound {
		t.Fatalf("approve twice: %d %v", r.status, r.body)
	}

	// Only the device code opens the session, never the user code.
	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/device/token", "", map[string]any{"deviceCode": userCode}); r.status != http.StatusNotFound {
		t.Fatalf("poll with the user code: %d", r.status)
	}
	sess := poll()
	if sess.status != http.StatusOK || sess.body["refreshToken"] == "" || sess.body["user"].(map[string]any)["username"] != "bob" {
		t.Fatalf("session: %d %v", sess.status, sess.body)
	}
	if r := env.get(t, sess.body["accessToken"].(string), "/api/v1/me"); r["username"] != "bob" {
		t.Fatalf("me: %v", r)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": sess.body["refreshToken"]}); r.status != http.StatusOK {
		t.Fatalf("refresh: %d", r.status)
	}
	if r := poll(); r.status != http.StatusNotFound || errorCode(r.body) != "device_login_invalid" {
		t.Fatalf("second poll: %d %v", r.status, r.body)
	}
}

func TestDeviceLoginDeniedAndExpired(t *testing.T) {
	env := newTestEnv(t)
	admin := setupAdmin(t, env.h).body["accessToken"].(string)
	if r := do(t, env.h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"publicUrl": "https://plys.example"}); r.status != http.StatusOK {
		t.Fatalf("settings: %d %v", r.status, r.body)
	}

	start := do(t, env.h, http.MethodPost, "/api/v1/auth/device", "", nil)
	userCode := start.body["userCode"].(string)
	if start.body["verificationUrl"] != "https://plys.example/link?code="+userCode {
		t.Fatalf("verification url: %v", start.body["verificationUrl"])
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/device-logins/deny", admin, map[string]any{"userCode": userCode}); r.status != http.StatusNoContent {
		t.Fatalf("deny: %d %v", r.status, r.body)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/device/token", "", map[string]any{"deviceCode": start.body["deviceCode"]}); r.status != http.StatusNotFound {
		t.Fatalf("poll after deny: %d %v", r.status, r.body)
	}

	start = do(t, env.h, http.MethodPost, "/api/v1/auth/device", "", nil)
	userCode = start.body["userCode"].(string)
	if _, err := env.srv.Pool.Exec(context.Background(), `UPDATE device_logins SET expires_at = now() - interval '1 second' WHERE user_code = $1`, userCode); err != nil {
		t.Fatal(err)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/device-logins/approve", admin, map[string]any{"userCode": userCode}); r.status != http.StatusNotFound {
		t.Fatalf("approve an expired request: %d %v", r.status, r.body)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/device/token", "", map[string]any{"deviceCode": start.body["deviceCode"]}); r.status != http.StatusNotFound {
		t.Fatalf("poll an expired request: %d %v", r.status, r.body)
	}
}

func TestDeviceLoginStartIsThrottled(t *testing.T) {
	env := newTestEnv(t)
	setupAdmin(t, env.h)
	for i := range deviceLoginStartLimit {
		if r := do(t, env.h, http.MethodPost, "/api/v1/auth/device", "", nil); r.status != http.StatusCreated {
			t.Fatalf("start %d: %d", i, r.status)
		}
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/device", "", nil); r.status != http.StatusTooManyRequests {
		t.Fatalf("over the limit: %d", r.status)
	}
}

func TestNormalizeUserCode(t *testing.T) {
	for in, want := range map[string]string{"bcdf-ghjk": "BCDF-GHJK", "BCDFGHJK": "BCDF-GHJK", " bcdf ghjk ": "BCDF-GHJK"} {
		if got, ok := normalizeUserCode(in); !ok || got != want {
			t.Errorf("%q: %q %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "BCDF-GHJ", "BCDF-GHJKL", "ABCD-EFGH", "BCDF-GHJ1"} {
		if _, ok := normalizeUserCode(in); ok {
			t.Errorf("%q accepted", in)
		}
	}
}
