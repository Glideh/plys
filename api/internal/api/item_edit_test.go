package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
)

func pngImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func putRaw(t *testing.T, h http.Handler, path, token string, body []byte) response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "image/png")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := response{status: rec.Code, header: rec.Header()}
	if rec.Body.Len() > 0 {
		out.body = map[string]any{}
		_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
	}
	return out
}

func TestEditItemMetadata(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "films/Alien.1979.mkv")
	films := e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	_, err := e.srv.Pool.Exec(context.Background(), `UPDATE items SET
		cast_members = '[{"id": 10205, "name": "Sigourney Weaver", "character": "Ripley", "profilePath": "/sw.jpg"}]'
		WHERE library_id = $1`, films)
	if err != nil {
		t.Fatal(err)
	}
	id := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items", admin, nil).body["items"].([]any)[0].(map[string]any)["id"].(string)

	do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"})
	bob := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)
	if r := do(t, e.h, http.MethodPatch, "/api/v1/items/"+id+"/metadata", bob, map[string]any{"title": "X"}); r.status != http.StatusForbidden {
		t.Fatalf("edit as user: expected 403, got %d", r.status)
	}
	for _, body := range []map[string]any{
		{"title": "   "},
		{"releaseDate": "31/05/2024"},
		{"overview": strings.Repeat("a", 10001)},
	} {
		if r := do(t, e.h, http.MethodPatch, "/api/v1/items/"+id+"/metadata", admin, body); r.status != http.StatusBadRequest {
			t.Errorf("%v: expected 400, got %d", body, r.status)
		}
	}

	r := do(t, e.h, http.MethodPatch, "/api/v1/items/"+id+"/metadata", admin, map[string]any{
		"title": " Le Huitième Passager ", "releaseDate": "1979-09-12", "overview": "Dans l'espace...", "tagline": "",
		"directors": []string{"Ridley Scott", " ", "Ridley Scott"},
		"cast":      []map[string]any{{"name": "Sigourney Weaver", "character": "Ellen Ripley"}, {"name": "Tom Skerritt", "character": "Dallas"}, {"name": ""}},
	})
	if r.status != http.StatusOK || r.body["title"] != "Le Huitième Passager" {
		t.Fatalf("edit: %d %v", r.status, r.body)
	}
	item := do(t, e.h, http.MethodGet, "/api/v1/items/"+id, admin, nil).body
	if item["title"] != "Le Huitième Passager" || item["year"] != float64(1979) || item["overview"] != "Dans l'espace..." || item["tagline"] != nil {
		t.Errorf("edited values: %v", item)
	}
	if d := item["directors"].([]any); len(d) != 1 || d[0] != "Ridley Scott" {
		t.Errorf("directors: %v", d)
	}
	cast := item["cast"].([]any)
	if len(cast) != 2 {
		t.Fatalf("cast: %v", cast)
	}
	// A person already listed keeps their picture and TMDB ID.
	if c := cast[0].(map[string]any); c["character"] != "Ellen Ripley" || c["id"] != float64(10205) || c["photoUrl"] != "/api/v1/images/tmdb/w185/sw.jpg" {
		t.Errorf("kept person: %v", c)
	}
	edited := []string{}
	for _, f := range item["editedFields"].([]any) {
		edited = append(edited, f.(string))
	}
	slices.Sort(edited)
	if want := []string{"cast", "directors", "overview", "releaseDate", "tagline", "title"}; !slices.Equal(edited, want) {
		t.Errorf("editedFields: %v, want %v", edited, want)
	}
	if bobItem := do(t, e.h, http.MethodGet, "/api/v1/items/"+id, bob, nil).body; bobItem["editedFields"] != nil {
		t.Errorf("only admins see the edited fields: %v", bobItem["editedFields"])
	}
	// The edited title is the one searched.
	if r := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items?query=passager", admin, nil); len(r.body["items"].([]any)) != 1 {
		t.Errorf("search by edited title: %v", r.body)
	}

	// Posters: a JPEG fitting 1000x1500 is stored, and a new upload replaces (and deletes) the previous one.
	if r := putRaw(t, e.h, "/api/v1/items/"+id+"/poster", admin, []byte("not an image")); r.status != http.StatusUnsupportedMediaType {
		t.Fatalf("invalid poster: expected 415, got %d", r.status)
	}
	r = putRaw(t, e.h, "/api/v1/items/"+id+"/poster", admin, pngImage(t, 1200, 1800))
	first, _ := r.body["posterUrl"].(string)
	if r.status != http.StatusOK || !strings.HasPrefix(first, "/api/v1/images/local/") {
		t.Fatalf("poster: %d %v", r.status, r.body)
	}
	served := do(t, e.h, http.MethodGet, first, "", nil)
	if served.status != http.StatusOK {
		t.Fatalf("serve poster: %d", served.status)
	}
	firstFile := e.srv.Images.Local(strings.TrimPrefix(first, "/api/v1/images/"))
	data, err := os.ReadFile(firstFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 1000 || cfg.Height != 1500 {
		t.Errorf("poster must be a JPEG scaled to 1000x1500: %+v %v", cfg, err)
	}
	r = putRaw(t, e.h, "/api/v1/items/"+id+"/poster", admin, pngImage(t, 200, 300))
	if second, _ := r.body["posterUrl"].(string); r.status != http.StatusOK || second == first {
		t.Fatalf("second poster: %d %v", r.status, r.body)
	}
	if _, err := os.Stat(firstFile); !os.IsNotExist(err) {
		t.Errorf("replaced poster must be deleted: %v", err)
	}

	// Without a TMDB key the edits cannot be reverted, and stay locked.
	if r := do(t, e.h, http.MethodDelete, "/api/v1/items/"+id+"/metadata", admin, nil); r.status != http.StatusConflict {
		t.Fatalf("revert without TMDB: expected 409, got %d %v", r.status, r.body)
	}
	if item := do(t, e.h, http.MethodGet, "/api/v1/items/"+id, admin, nil).body; len(item["editedFields"].([]any)) != 7 {
		t.Errorf("edits must stay locked: %v", item["editedFields"])
	}
}

func TestVideoEditsAndRevert(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "videos/Vacances 2019.mkv")
	videos := e.createScannedLibrary(t, admin, "Vidéos", "other", "videos", nil)
	id := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+videos+"/items", admin, nil).body["items"].([]any)[0].(map[string]any)["id"].(string)

	if r := putRaw(t, e.h, "/api/v1/items/"+id+"/poster", admin, pngImage(t, 20, 30)); r.status != http.StatusBadRequest {
		t.Errorf("videos have no poster: expected 400, got %d", r.status)
	}
	if r := do(t, e.h, http.MethodPatch, "/api/v1/items/"+id+"/metadata", admin, map[string]any{"title": "Bretagne, été 2019"}); r.status != http.StatusOK {
		t.Fatalf("edit video: %d %v", r.status, r.body)
	}
	// Reverting a video needs no TMDB key: the edits are unlocked and the thumbnail is extracted again (which
	// fails here, the test file being no real video).
	if r := do(t, e.h, http.MethodDelete, "/api/v1/items/"+id+"/metadata", admin, nil); r.status == http.StatusConflict {
		t.Fatalf("revert video must not need TMDB: %v", r.body)
	}
	if item := do(t, e.h, http.MethodGet, "/api/v1/items/"+id, admin, nil).body; item["editedFields"] != nil {
		t.Errorf("revert must unlock the fields: %v", item["editedFields"])
	}
}
