package opensubtitles

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSRTToVTT(t *testing.T) {
	srt := "\uFEFF1\r\n00:00:01,500 --> 00:00:03,000\r\n{\\an8}<i>Bonjour</i>\r\nA --> B\r\n\r\n2\r\n1:02:03,004 --> 1:02:04,000\r\nFin\r\n"
	got, err := SRTToVTT([]byte(srt))
	if err != nil {
		t.Fatal(err)
	}
	want := "WEBVTT\n\n00:00:01.500 --> 00:00:03.000\n<i>Bonjour</i>\nA -> B\n\n01:02:03.004 --> 01:02:04.000\nFin\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	latin := []byte("1\n00:00:01,000 --> 00:00:02,000\nD\xe9j\xe0 vu \x80\n")
	if got, _ := SRTToVTT(latin); !strings.Contains(got, "Déjà vu €") {
		t.Errorf("Windows-1252 text must be decoded: %q", got)
	}
	if _, err := SRTToVTT([]byte("not a subtitle")); err == nil {
		t.Error("a file without cues must be refused")
	}
}

func TestMovieHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.mkv")
	data := make([]byte, 3*hashChunk)
	binary.LittleEndian.PutUint64(data[0:], 1)
	binary.LittleEndian.PutUint64(data[len(data)-8:], 2)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := MovieHash(path)
	if err != nil {
		t.Fatal(err)
	}
	// size (0x30000) + 1 + 2
	if got != "0000000000030003" {
		t.Errorf("got %s", got)
	}
}

func TestSearchAndDownload(t *testing.T) {
	var logins, downloads atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/file.srt" && r.Header.Get("Api-Key") != "key" || !strings.HasPrefix(r.Header.Get("User-Agent"), "Plys") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/v1/login":
			logins.Add(1)
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["password"] != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "tok", "base_url": "ignored.example"})
		case "/api/v1/subtitles":
			q := r.URL.Query()
			if q.Get("parent_tmdb_id") != "1399" || q.Get("season_number") != "1" || q.Get("episode_number") != "2" || q.Get("languages") != "pt-br" {
				t.Errorf("unexpected query %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[
				{"attributes":{"language":"pt-BR","release":"Popular","download_count":900,"files":[{"file_id":1}]}},
				{"attributes":{"language":"pt-BR","release":"Two CDs","download_count":5000,"files":[{"file_id":2},{"file_id":3}]}},
				{"attributes":{"language":"pt-BR","release":"","download_count":10,"moviehash_match":true,"files":[{"file_id":4,"file_name":"exact.srt"}]}}
			]}`))
		case "/api/v1/download":
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if downloads.Add(1) > 1 {
				w.WriteHeader(http.StatusNotAcceptable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"link": srv.URL + "/file.srt"})
		case "/api/v1/discover/popular":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/file.srt":
			_, _ = w.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nOlá\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	creds := Credentials{APIKey: "key", Username: "me", Password: "secret"}
	c := New(srv.URL+"/api/v1", "Plys vtest", func(context.Context) (Credentials, error) { return creds, nil })
	ctx := context.Background()

	res, err := c.Search(ctx, Query{Language: "pt-BR", ParentTMDBID: 1399, Season: 1, Episode: 2, TMDBID: 5, Text: "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].FileID != 4 || res[0].Release != "exact.srt" || res[1].FileID != 1 {
		t.Errorf("hash matches first, multi-file subtitles dropped: %+v", res)
	}

	data, err := c.Download(ctx, 4)
	if err != nil || !strings.Contains(string(data), "Olá") {
		t.Fatalf("download: %v %q", err, data)
	}
	if _, err := c.Download(ctx, 4); !errors.Is(err, ErrQuota) {
		t.Errorf("expected the quota error, got %v", err)
	}
	if logins.Load() != 1 {
		t.Errorf("the login token must be reused, %d logins", logins.Load())
	}

	if err := c.Validate(ctx, Credentials{APIKey: "key", Username: "me", Password: "secret"}); err != nil {
		t.Errorf("valid key and login: %v", err)
	}
	if err := c.Validate(ctx, Credentials{APIKey: "key", Username: "me", Password: "wrong"}); !errors.Is(err, ErrUnauthorized) || !errors.Is(err, ErrInvalidLogin) || errors.Is(err, ErrInvalidAPIKey) {
		t.Errorf("wrong password: %v", err)
	}
	if err := c.Validate(ctx, Credentials{APIKey: "bad", Username: "me", Password: "secret"}); !errors.Is(err, ErrInvalidAPIKey) || errors.Is(err, ErrInvalidLogin) {
		t.Errorf("wrong key with a login: %v", err)
	}
	if err := c.Validate(ctx, Credentials{APIKey: "bad"}); !errors.Is(err, ErrUnauthorized) || !errors.Is(err, ErrInvalidAPIKey) {
		t.Errorf("wrong key: %v", err)
	}
	creds = Credentials{}
	if _, err := c.Search(ctx, Query{Language: "fr"}); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("without key: %v", err)
	}
}

func TestErrors(t *testing.T) {
	var logins atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/discover/popular":
			if r.Header.Get("Api-Key") != "key" {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"message":"You cannot consume this service"}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/login":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			switch body["username"] {
			case "busy":
				// The first login is limited, the retry gets through.
				if logins.Add(1) == 1 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				_, _ = w.Write([]byte(`{"token":"tok"}`))
			case "flood":
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
			case "me@example.com":
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"Error, invalid username/password"}`))
			default:
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"errors":["maintenance"]}`))
			}
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "Plys vtest", func(context.Context) (Credentials, error) { return Credentials{}, nil })
	ctx := context.Background()

	if err := c.Validate(ctx, Credentials{APIKey: "key"}); err != nil {
		t.Errorf("valid key: %v", err)
	}
	var apiErr *APIError
	err := c.Validate(ctx, Credentials{APIKey: "bad"})
	if !errors.Is(err, ErrInvalidAPIKey) || !errors.As(err, &apiErr) || apiErr.Message != "You cannot consume this service" {
		t.Errorf("unknown key: %v", err)
	}
	if err := c.Validate(ctx, Credentials{APIKey: "key", Username: "busy", Password: "p"}); err != nil {
		t.Errorf("a short rate limit must be retried once: %v", err)
	}
	if err := c.Validate(ctx, Credentials{APIKey: "key", Username: "flood", Password: "p"}); !errors.Is(err, ErrRateLimited) {
		t.Errorf("a long rate limit must not be waited for: %v", err)
	}
	err = c.Validate(ctx, Credentials{APIKey: "key", Username: "me@example.com", Password: "p"})
	if !errors.Is(err, ErrInvalidLogin) || !errors.As(err, &apiErr) || apiErr.Detail() != "Error, invalid username/password (HTTP 400)" {
		t.Errorf("a login refused with 400 means wrong credentials, with the OpenSubtitles message: %v", err)
	}
	err = c.Validate(ctx, Credentials{APIKey: "key", Username: "other", Password: "p"})
	if errors.Is(err, ErrUnauthorized) || !errors.As(err, &apiErr) || apiErr.Detail() != "maintenance (HTTP 503)" {
		t.Errorf("other statuses keep the OpenSubtitles message: %v", err)
	}
}
