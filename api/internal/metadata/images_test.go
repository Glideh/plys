package metadata

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPrefetchFetchesOnlyMissingImages(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/w342/gone.jpg" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("jpeg"))
	}))
	defer srv.Close()

	cache := NewImageCache(t.TempDir(), srv.URL)
	files := []string{"a.jpg", "b.jpg", "gone.jpg", "c.png"}
	if n := cache.Prefetch(context.Background(), "w342", files); n != 3 {
		t.Fatalf("fetched %d images, want 3", n)
	}
	for _, f := range []string{"a.jpg", "b.jpg", "c.png"} {
		if _, ok := cache.Cached("w342", f); !ok {
			t.Errorf("%s should be cached", f)
		}
	}
	if _, ok := cache.Cached("w342", "gone.jpg"); ok {
		t.Error("a 404 must not leave a cached file")
	}

	before := hits.Load()
	if n := cache.Prefetch(context.Background(), "w342", []string{"a.jpg", "b.jpg", "c.png"}); n != 0 || hits.Load() != before {
		t.Errorf("cached images must not be requested again (fetched %d, extra hits %d)", n, hits.Load()-before)
	}
}
