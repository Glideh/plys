package api

import (
	"net/http"
	"testing"
)

func TestPlaybackSessionAccess(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "private/Alien.1979.mkv")

	do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "carol", "password": "password123"})
	carol := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "carol", "password": "password123"}).body["accessToken"].(string)
	lib := e.createScannedLibrary(t, admin, "Privé", "movies", "private", map[string]any{"restricted": true})

	itemID := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+lib+"/items", admin, nil).body["items"].([]any)[0].(map[string]any)["id"].(string)
	version := do(t, e.h, http.MethodGet, "/api/v1/items/"+itemID, admin, nil).body["versions"].([]any)[0].(map[string]any)["id"].(string)

	start := func(token string, body map[string]any) response {
		body["versionId"] = version
		body["caps"] = map[string]any{"videoCodecs": []string{"h264", "hevc"}, "audioCodecs": []string{"aac"}}
		return do(t, e.h, http.MethodPost, "/api/v1/playback/sessions", token, body)
	}

	if r := start(carol, map[string]any{}); r.status != http.StatusNotFound {
		t.Errorf("restricted version: expected 404, got %d", r.status)
	}
	if r := start(admin, map[string]any{"audioIndex": 0}); r.status != http.StatusBadRequest {
		t.Errorf("video stream given as audio: expected 400, got %d", r.status)
	}

	// The fake probe describes 1080p HDR HEVC with E-AC3 and forced PGS subtitles.
	r := start(admin, map[string]any{"audioIndex": 2, "subtitleIndex": 3, "start": 60})
	if r.status != http.StatusCreated {
		t.Fatalf("start: %d %v", r.status, r.body)
	}
	if r.body["copyVideo"] != false || r.body["burnedSubtitle"] != true || r.body["audioIndex"].(float64) != 2 || r.body["subtitleUrl"] != nil {
		t.Errorf("image subtitles must be burned into a transcoded video: %v", r.body)
	}
	sessionID := r.body["id"].(string)

	if r := start(admin, map[string]any{}); r.status != http.StatusServiceUnavailable {
		t.Errorf("the second transcode must be refused while the limit is 1, got %d", r.status)
	}
	if r := do(t, e.h, http.MethodGet, "/api/v1/stream/"+sessionID+"/../../etc/passwd", "", nil); r.status != http.StatusNotFound {
		t.Errorf("path traversal: expected 404, got %d", r.status)
	}
	if r := do(t, e.h, http.MethodDelete, "/api/v1/playback/sessions/"+sessionID, carol, nil); r.status != http.StatusNotFound {
		t.Errorf("another user must not stop the session, got %d", r.status)
	}
	if r := do(t, e.h, http.MethodDelete, "/api/v1/playback/sessions/"+sessionID, admin, nil); r.status != http.StatusNoContent {
		t.Errorf("stop: %d", r.status)
	}
	if r := do(t, e.h, http.MethodGet, "/api/v1/stream/"+sessionID+"/index.m3u8", "", nil); r.status != http.StatusNotFound {
		t.Errorf("stopped session must be gone, got %d", r.status)
	}
}
