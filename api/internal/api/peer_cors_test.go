package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPeerCORSAllowsPairedOriginOnly(t *testing.T) {
	env, peer, _ := pairedEnv(t)
	path := "/api/v1/items/" + uuid.New().String()

	preflight := func(origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodOptions, path, nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		req.Header.Set("Access-Control-Request-Headers", "authorization")
		rec := httptest.NewRecorder()
		env.h.ServeHTTP(rec, req)
		return rec
	}

	rec := preflight(peer.srv.URL)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != peer.srv.URL {
		t.Fatalf("paired origin preflight: %d %v", rec.Code, rec.Header())
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("authorization header not allowed: %v", rec.Header())
	}

	if rec := preflight("https://evil.example.org"); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unpaired origin allowed: %v", rec.Header())
	}
}
