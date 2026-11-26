package api

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestAdminPlaybackSessions(t *testing.T) {
	const path = "/api/v1/admin/playback/sessions"
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	user := do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "alice", "password": "password123"})
	userID := user.body["id"].(string)
	alice := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "alice", "password": "password123"}).body["accessToken"].(string)
	if r := do(t, e.h, http.MethodPut, "/api/v1/users/"+userID+"/alias", admin, map[string]any{"alias": "Salon"}); r.status != http.StatusOK {
		t.Fatalf("set alias: %d %v", r.status, r.body)
	}
	if r := avatarRequest(e.h, http.MethodPut, "/api/v1/me/avatar", alice, encodeAvatarPNG(t, avatarTestImage(256, 256))); r.Code != http.StatusOK {
		t.Fatalf("upload avatar: %d", r.Code)
	}
	if r := do(t, e.h, http.MethodGet, path, "", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request: %d", r.status)
	}
	assertEmpty := func() {
		t.Helper()
		if r := do(t, e.h, http.MethodGet, path, admin, nil); r.status != http.StatusOK || !reflect.DeepEqual(r.body["items"], []any{}) {
			t.Fatalf("expected empty list: %d %v", r.status, r.body)
		}
	}
	assertEmpty()

	writeFiles(t, e.root, "movies/Alien.1979.mkv")
	lib := e.createScannedLibrary(t, admin, "Movies", "movies", "movies", nil)
	item := e.get(t, admin, "/api/v1/libraries/"+lib+"/items")["items"].([]any)[0].(map[string]any)
	version := e.get(t, admin, "/api/v1/items/"+item["id"].(string))["versions"].([]any)[0].(map[string]any)
	start := do(t, e.h, http.MethodPost, "/api/v1/playback/sessions", alice, map[string]any{
		"versionId": version["id"], "subtitleIndex": 3,
		"caps": map[string]any{"videoCodecs": []string{"hevc"}, "audioCodecs": []string{"aac"}, "maxHeight": 720},
	})
	if start.status != http.StatusCreated {
		t.Fatalf("start playback: %d %v", start.status, start.body)
	}
	if r := do(t, e.h, http.MethodGet, path, alice, nil); r.status != http.StatusForbidden {
		t.Fatalf("non-admin request: %d", r.status)
	}
	r := do(t, e.h, http.MethodGet, path, admin, nil)
	if r.status != http.StatusOK || r.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("list sessions: %d %v %v", r.status, r.body, r.header)
	}
	items := r.body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one session: %v", items)
	}
	session := items[0].(map[string]any)
	identity := session["user"].(map[string]any)
	if identity["displayName"] != "Salon" || identity["id"] != userID || identity["avatarUpdatedAt"] == nil {
		t.Errorf("incorrect user details: %v", identity)
	}
	source := session["source"].(map[string]any)
	video := source["video"].(map[string]any)
	audio := source["audio"].(map[string]any)
	subtitle := source["subtitle"].(map[string]any)
	if source["title"] != item["title"] || source["container"] != "matroska,webm" ||
		video["codec"] != "hevc" || video["height"] != float64(800) || video["colorTransfer"] != "smpte2084" ||
		audio["codec"] != "eac3" || subtitle["codec"] != "hdmv_pgs_subtitle" {
		t.Errorf("incorrect source: %v", source)
	}
	caps := session["caps"].(map[string]any)
	if !reflect.DeepEqual(caps["videoCodecs"], []any{"hevc"}) || !reflect.DeepEqual(caps["audioCodecs"], []any{"aac"}) ||
		caps["hdr"] != false || caps["maxHeight"] != float64(720) {
		t.Errorf("incorrect capabilities: %v", caps)
	}
	plan := session["plan"].(map[string]any)
	if plan["copyVideo"] != false || plan["copyAudio"] != false || plan["tonemap"] != true ||
		plan["burnSubtitle"] != true || plan["height"] != float64(720) || session["outputAudioCodec"] != "aac" ||
		session["videoAccel"] != "none" {
		t.Errorf("incorrect processing: %v", session)
	}
	pipeline, ok := session["videoPipeline"].(map[string]any)
	if !ok || pipeline["decode"] != "cpu" || pipeline["toneMap"] != "cpu" ||
		pipeline["burnSubtitle"] != "cpu" || pipeline["scale"] != "cpu" || pipeline["encode"] != "cpu" {
		t.Errorf("incorrect video pipeline: %v", session["videoPipeline"])
	}
	if want := []any{"image_subtitle", "hdr_unsupported", "height_limit", "audio_client_codec"}; !reflect.DeepEqual(session["reasons"], want) {
		t.Errorf("reasons = %v, want %v", session["reasons"], want)
	}
	sessionID := start.body["id"].(string)
	if strings.Contains(session["id"].(string), sessionID) || session["playlistUrl"] != nil || source["path"] != nil {
		t.Errorf("list leaked playlist credentials or source path: %v", session)
	}
	if r := do(t, e.h, http.MethodDelete, "/api/v1/playback/sessions/"+sessionID, alice, nil); r.status != http.StatusNoContent {
		t.Fatalf("stop playback: %d %v", r.status, r.body)
	}
	assertEmpty()
}
