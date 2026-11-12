package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"toto/api/internal/opensubtitles"
)

// fakeOpenSubtitles accepts the API key "key" and serves one French subtitle for "Alien".
func fakeOpenSubtitles(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Api-Key") != "key" && r.URL.Path != "/file" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/infos/formats":
			_, _ = w.Write([]byte(`{"data":{}}`))
		case "/subtitles":
			if r.URL.Query().Get("query") != "alien" || r.URL.Query().Get("languages") != "fr" {
				t.Errorf("unexpected search %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[{"attributes":{"language":"fr","release":"Alien.1979.BluRay","download_count":12,"files":[{"file_id":77}]}}]}`))
		case "/download":
			_ = json.NewEncoder(w).Encode(map[string]any{"link": srv.URL + "/file"})
		case "/file":
			_, _ = w.Write([]byte("1\r\n00:01:10,000 --> 00:01:12,000\r\nDans l'espace\r\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOnlineSubtitles(t *testing.T) {
	fake := fakeOpenSubtitles(t)
	e := newTestEnvWith(t, func(d *Deps) {
		st := d.Settings
		d.OpenSubtitles = opensubtitles.New(fake.URL, "Plys test", func(ctx context.Context) (opensubtitles.Credentials, error) {
			v, err := st.OpenSubtitles(ctx)
			return opensubtitles.Credentials{APIKey: v.APIKey, Username: v.Username, Password: v.Password}, err
		})
	})
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "movies/Alien.1979.mkv")
	lib := e.createScannedLibrary(t, admin, "Films", "movies", "movies", nil)
	itemID := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+lib+"/items", admin, nil).body["items"].([]any)[0].(map[string]any)["id"].(string)
	item := do(t, e.h, http.MethodGet, "/api/v1/items/"+itemID, admin, nil).body
	version := item["versions"].([]any)[0].(map[string]any)["id"].(string)
	if item["subtitleSearch"] != false {
		t.Errorf("search must be off until configured: %v", item["subtitleSearch"])
	}
	search := "/api/v1/items/" + itemID + "/subtitles/search?language=fr&versionId=" + version
	if r := do(t, e.h, http.MethodGet, search, admin, nil); r.status != http.StatusConflict {
		t.Errorf("unconfigured search: expected 409, got %d", r.status)
	}

	if r := do(t, e.h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"openSubtitlesApiKey": "bad"}); r.status != http.StatusBadRequest {
		t.Errorf("rejected key: expected 400, got %d", r.status)
	}
	r := do(t, e.h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"openSubtitlesApiKey": " key "})
	if r.status != http.StatusOK || r.body["openSubtitlesConfigured"] != true || r.body["openSubtitlesApiKey"] != nil {
		t.Fatalf("configure: %d %v", r.status, r.body)
	}
	if r := do(t, e.h, http.MethodPatch, "/api/v1/admin/settings", admin, map[string]any{"openSubtitlesUsername": "me"}); r.status != http.StatusBadRequest {
		t.Errorf("username without password: expected 400, got %d", r.status)
	}
	if item := do(t, e.h, http.MethodGet, "/api/v1/items/"+itemID, admin, nil).body; item["subtitleSearch"] != true {
		t.Errorf("search must be offered once configured")
	}

	if r := do(t, e.h, http.MethodGet, "/api/v1/items/"+itemID+"/subtitles/search?language=french&versionId="+version, admin, nil); r.status != http.StatusBadRequest {
		t.Errorf("bad language: expected 400, got %d", r.status)
	}
	r = do(t, e.h, http.MethodGet, search, admin, nil)
	results, _ := r.body["results"].([]any)
	if r.status != http.StatusOK || len(results) != 1 || results[0].(map[string]any)["fileId"].(float64) != 77 {
		t.Fatalf("search: %d %v", r.status, r.body)
	}

	download := map[string]any{"versionId": version, "fileId": 77, "language": "fr", "release": "Alien.1979.BluRay"}
	r = do(t, e.h, http.MethodPost, "/api/v1/items/"+itemID+"/subtitles", admin, download)
	if r.status != http.StatusCreated || r.body["language"] != "fr" || r.body["source"] != "opensubtitles" {
		t.Fatalf("download: %d %v", r.status, r.body)
	}
	subID := r.body["id"].(string)
	if again := do(t, e.h, http.MethodPost, "/api/v1/items/"+itemID+"/subtitles", admin, download); again.status != http.StatusOK || again.body["id"] != subID {
		t.Errorf("a second download must reuse the stored subtitle: %d %v", again.status, again.body)
	}
	versions := do(t, e.h, http.MethodGet, "/api/v1/items/"+itemID, admin, nil).body["versions"].([]any)
	if ext := versions[0].(map[string]any)["externalSubtitles"].([]any); len(ext) != 1 || ext[0].(map[string]any)["id"] != subID {
		t.Errorf("the version must list the downloaded subtitle: %v", ext)
	}

	caps := map[string]any{"videoCodecs": []string{"h264", "hevc"}, "audioCodecs": []string{"aac", "eac3"}, "hdr": true, "maxHeight": 2160}
	if r := do(t, e.h, http.MethodPost, "/api/v1/playback/sessions", admin, map[string]any{"versionId": version, "caps": caps, "externalSubtitleId": itemID}); r.status != http.StatusBadRequest {
		t.Errorf("unknown external subtitle: expected 400, got %d", r.status)
	}
	r = do(t, e.h, http.MethodPost, "/api/v1/playback/sessions", admin, map[string]any{"versionId": version, "caps": caps, "externalSubtitleId": subID, "start": 60})
	if r.status != http.StatusCreated || r.body["externalSubtitleId"] != subID {
		t.Fatalf("start: %d %v", r.status, r.body)
	}
	subURL, _ := r.body["subtitleUrl"].(string)
	sessionStart := r.body["start"].(float64)
	if !strings.HasSuffix(subURL, "/subtitles/external.vtt") {
		t.Fatalf("subtitle URL: %q", subURL)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, subURL, nil))
	body, _ := io.ReadAll(rec.Body)
	wantCue := map[float64]string{0: "00:01:10.000 --> 00:01:12.000", 60: "00:00:10.000 --> 00:00:12.000"}[sessionStart]
	if rec.Code != http.StatusOK || !strings.HasPrefix(string(body), "WEBVTT") || (wantCue != "" && !strings.Contains(string(body), wantCue)) {
		t.Errorf("subtitle served: %d %q (start %v)", rec.Code, body, sessionStart)
	}
	do(t, e.h, http.MethodDelete, "/api/v1/playback/sessions/"+r.body["id"].(string), admin, nil)

	if r := do(t, e.h, http.MethodDelete, "/api/v1/items/"+itemID+"/subtitles/"+subID+"?versionId="+version, admin, nil); r.status != http.StatusNoContent {
		t.Errorf("delete: %d", r.status)
	}
	versions = do(t, e.h, http.MethodGet, "/api/v1/items/"+itemID, admin, nil).body["versions"].([]any)
	if ext := versions[0].(map[string]any)["externalSubtitles"].([]any); len(ext) != 0 {
		t.Errorf("deleted subtitle still listed: %v", ext)
	}
}
