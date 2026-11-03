package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func avatarRequest(h http.Handler, method, path, token string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func avatarTestImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	return img
}

func encodeAvatarPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func decodeJSONBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON body %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestAvatarLifecycle(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)

	me := e.get(t, admin, "/api/v1/me")
	if _, ok := me["avatarUpdatedAt"]; ok {
		t.Fatalf("a new user has no avatar: %v", me)
	}
	avatarPath := "/api/v1/images/avatars/" + me["id"].(string)

	// A non-square PNG is accepted and stored as a 256x256 square.
	rec := avatarRequest(e.h, http.MethodPut, "/api/v1/me/avatar", admin, encodeAvatarPNG(t, avatarTestImage(800, 400)))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload PNG: %d %s", rec.Code, rec.Body.String())
	}
	if body := decodeJSONBody(t, rec); body["avatarUpdatedAt"] == nil {
		t.Fatalf("the upload must set avatarUpdatedAt: %v", body)
	}
	img := avatarRequest(e.h, http.MethodGet, avatarPath, "", nil)
	if img.Code != http.StatusOK || img.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("serve avatar: %d %v", img.Code, img.Header())
	}
	decoded, err := png.Decode(img.Body)
	if err != nil {
		t.Fatal(err)
	}
	if b := decoded.Bounds(); b.Dx() != avatarSize || b.Dy() != avatarSize {
		t.Errorf("avatar must be %dx%d, got %v", avatarSize, avatarSize, b)
	}

	// JPEG is accepted too.
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, avatarTestImage(300, 300), nil); err != nil {
		t.Fatal(err)
	}
	if rec := avatarRequest(e.h, http.MethodPut, "/api/v1/me/avatar", admin, jpg.Bytes()); rec.Code != http.StatusOK {
		t.Errorf("upload JPEG: %d %s", rec.Code, rec.Body.String())
	}

	// Other formats are rejected.
	if rec := avatarRequest(e.h, http.MethodPut, "/api/v1/me/avatar", admin, []byte("not an image")); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("reject non-image: %d %s", rec.Code, rec.Body.String())
	}

	// Files above 2 MB are rejected.
	if rec := avatarRequest(e.h, http.MethodPut, "/api/v1/me/avatar", admin, make([]byte, maxAvatarBytes+1)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("reject oversized upload: %d", rec.Code)
	}

	// Removal clears the avatar.
	rec = avatarRequest(e.h, http.MethodDelete, "/api/v1/me/avatar", admin, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete avatar: %d %s", rec.Code, rec.Body.String())
	}
	if body := decodeJSONBody(t, rec); body["avatarUpdatedAt"] != nil {
		t.Errorf("delete must clear avatarUpdatedAt: %v", body)
	}
	if img := avatarRequest(e.h, http.MethodGet, avatarPath, "", nil); img.Code != http.StatusNotFound {
		t.Errorf("avatar must be gone after delete: %d", img.Code)
	}
	if me := e.get(t, admin, "/api/v1/me"); me["avatarUpdatedAt"] != nil {
		t.Errorf("/me must not report a removed avatar: %v", me)
	}

	// Authentication and identifiers are checked.
	if rec := avatarRequest(e.h, http.MethodPut, "/api/v1/me/avatar", "", encodeAvatarPNG(t, avatarTestImage(10, 10))); rec.Code != http.StatusUnauthorized {
		t.Errorf("upload needs authentication: %d", rec.Code)
	}
	if rec := avatarRequest(e.h, http.MethodGet, "/api/v1/images/avatars/not-a-uuid", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("invalid avatar id: %d", rec.Code)
	}
}
