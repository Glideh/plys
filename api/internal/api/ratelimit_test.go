package api

import (
	"net/http"
	"testing"
	"time"
)

func TestBackoffLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := newBackoffLimiter(2, time.Minute, 5*time.Minute)
	l.now = func() time.Time { return now }

	for range 2 {
		if wait := l.allow("k"); wait != 0 {
			t.Fatalf("attempt refused: %v", wait)
		}
	}
	if wait := l.allow("k"); wait != time.Minute {
		t.Fatalf("first block: %v", wait)
	}
	if l.allow("other") != 0 {
		t.Fatal("keys must be independent")
	}
	now = now.Add(30 * time.Second)
	if wait := l.allow("k"); wait != 30*time.Second {
		t.Fatalf("while blocked: %v", wait)
	}

	// Each new block doubles, up to maxBlock.
	for _, want := range []time.Duration{2 * time.Minute, 4 * time.Minute, 5 * time.Minute} {
		now = now.Add(l.allow("k"))
		for range 2 {
			if wait := l.allow("k"); wait != 0 {
				t.Fatalf("attempt after the block refused: %v", wait)
			}
		}
		if wait := l.allow("k"); wait != want {
			t.Fatalf("block = %v, want %v", wait, want)
		}
	}

	// Strikes are forgotten after maxBlock without a block.
	now = now.Add(5*time.Minute + 5*time.Minute)
	l.allow("k")
	l.allow("k")
	if wait := l.allow("k"); wait != time.Minute {
		t.Fatalf("strikes not forgotten: %v", wait)
	}

	l.reset("k")
	if l.allow("k") != 0 {
		t.Fatal("reset key refused")
	}
}

func TestLoginRateLimit(t *testing.T) {
	h := newTestServer(t)
	setupAdmin(t, h)

	for range loginUserLimit {
		if r := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "Admin", "password": "wrong-password"}); r.status != http.StatusUnauthorized {
			t.Fatalf("login: %d %v", r.status, r.body)
		}
	}
	r := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "admin", "password": "password123"})
	if r.status != http.StatusTooManyRequests || r.header.Get("Retry-After") != "60" {
		t.Fatalf("expected 429 with Retry-After, got %d %v %v", r.status, r.body, r.header)
	}
	if code := r.body["error"].(map[string]any)["code"]; code != "rate_limited" {
		t.Fatalf("code = %v", code)
	}

	// Another account from the same address is still allowed, until the address limit.
	r = do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "someone", "password": "wrong-password"})
	if r.status != http.StatusUnauthorized {
		t.Fatalf("other account: %d %v", r.status, r.body)
	}
}

func TestLoginRateLimitPerAddress(t *testing.T) {
	h := newTestServer(t)
	setupAdmin(t, h)

	for i := range loginIPLimit {
		r := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "user" + string(rune('a'+i)), "password": "wrong-password"})
		if r.status != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d %v", i, r.status, r.body)
		}
	}
	r := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "admin", "password": "password123"})
	if r.status != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d %v", r.status, r.body)
	}
}

func TestSuccessfulLoginResetsAccountLimit(t *testing.T) {
	h := newTestServer(t)
	setupAdmin(t, h)

	for range 3 {
		for range loginUserLimit - 1 {
			do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "admin", "password": "wrong-password"})
		}
		if r := do(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "admin", "password": "password123"}); r.status != http.StatusOK {
			t.Fatalf("login: %d %v", r.status, r.body)
		}
	}
}
