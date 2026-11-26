package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestNewer(t *testing.T) {
	tests := []struct {
		release, current string
		want             bool
	}{
		{"0.8.0", "0.7.9", true},
		{"v0.7.10", "0.7.9", true},
		{"1.0.0", "0.99.99", true},
		{"0.7.9", "0.7.9", false},
		{"0.7.8", "0.7.9", false},
		{"0.8.0", "dev", false},
		{"0.8.0", "0.2.0-dev", false},
		{"0.8.0-rc1", "0.7.9", false},
		{"0.8", "0.7.9", false},
		{"", "0.7.9", false},
	}
	for _, test := range tests {
		if got := Newer(test.release, test.current); got != test.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", test.release, test.current, got, test.want)
		}
	}
}

func TestCheck(t *testing.T) {
	tag := "0.8.0"
	var calls, conditional atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("User-Agent") != "Plys-Server/0.7.9" {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		etag := `"` + tag + `"`
		if r.Header.Get("If-None-Match") == etag {
			conditional.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte(`{"tag_name":"` + tag + `","html_url":"https://example.test/releases/` + tag + `","draft":false,"prerelease":false}`))
	}))
	defer srv.Close()
	ctx := context.Background()

	c := New(srv.URL, "0.7.9")
	if c.Available() != nil {
		t.Fatal("update before any check")
	}
	if err := c.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if u := c.Available(); u == nil || u.Version != "0.8.0" || u.URL != "https://example.test/releases/0.8.0" {
		t.Fatalf("Available() = %+v", u)
	}

	// An unchanged release answers 304 and keeps the known update.
	if err := c.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if conditional.Load() != 1 || c.Available() == nil {
		t.Fatalf("conditional = %d, update = %+v", conditional.Load(), c.Available())
	}

	// A release that is not newer clears it.
	tag = "0.7.9"
	if err := c.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if c.Available() != nil {
		t.Fatalf("Available() = %+v, want none", c.Available())
	}

	// A development build never reports an update.
	tag = "9.0.0"
	dev := New(srv.URL, "dev")
	dev.userAgent = "Plys-Server/0.7.9"
	if err := dev.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if dev.Available() != nil {
		t.Fatalf("dev Available() = %+v", dev.Available())
	}
	if calls.Load() != 4 {
		t.Fatalf("calls = %d", calls.Load())
	}

	var none *Checker
	if none.Available() != nil {
		t.Fatal("nil checker reported an update")
	}
}

func TestCheckFailureKeepsLatest(t *testing.T) {
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "rate limited", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"0.8.0","html_url":"u"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "0.7.9")
	if err := c.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	fail = true
	if err := c.Check(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
	if c.Available() == nil {
		t.Fatal("failed check dropped the known update")
	}
}
