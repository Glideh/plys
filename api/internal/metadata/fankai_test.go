package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

type fanKaiTestTransport func(*http.Request) (*http.Response, error)

func (f fanKaiTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFanKaiImportsExistingSeasonsAndEpisodeImages(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	lib := e.library(t, "series", "Unknown.Series.S01E01.mkv", "Unknown.Series.S01E02.mkv", "Unknown.Series.S01E04.mkv")
	var seriesID, episodeID uuid.UUID
	if err := e.pool.QueryRow(ctx, `SELECT id FROM items WHERE library_id=$1 AND kind='series'`, lib).Scan(&seriesID); err != nil {
		t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `SELECT id FROM items WHERE library_id=$1 AND kind='episode' AND episode_number=1`, lib).Scan(&episodeID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `UPDATE items SET title='Edited episode', locked_fields=ARRAY['title'] WHERE id=$1`, episodeID); err != nil {
		t.Fatal(err)
	}
	var imageRequests atomic.Int32
	images := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		imageRequests.Add(1)
		if r.URL.Path != "/episodes/501/image" && r.URL.Path != "/episodes/502/image" && r.URL.Path != "/seasons/90/image/poster" {
			t.Errorf("unexpected image request: %s", r.URL)
		}
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
			t.Error(err)
		}
	}))
	defer images.Close()
	target, err := url.Parse(images.URL)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewImageCache(t.TempDir(), "")
	transport := images.Client().Transport
	cache.http.Transport = fanKaiTestTransport(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return transport.RoundTrip(r)
	})
	e.svc.SetImages(cache)

	var metadataRequests atomic.Int32
	var failEpisodes atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadataRequests.Add(1)
		switch r.URL.Path {
		case "/series/77":
			_, _ = w.Write([]byte(`{"id":77,"title":"FanKai series"}`))
		case "/series/77/seasons":
			_, _ = w.Write([]byte(`{"seasons":[{"id":90,"season_number":1,"title":"FanKai season","plot":"Season synopsis","premiered":"2020-01-02","images":{"poster":"https://metadata.fankai.fr/seasons/90/image/poster"}},{"id":91,"season_number":2}]}`))
		case "/seasons/90/episodes":
			if failEpisodes.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"episodes":[{"id":501,"episode_number":1,"title":"FanKai episode","plot":"Episode synopsis","aired":"2020-01-03","duration":3769,"links":{"thumbnail":"/episodes/501/image"}},{"id":502,"episode_number":2,"title":"Second episode","thumb_image":"https://metadata.fankai.fr/episodes/502/image"},{"id":503,"episode_number":3,"thumb_image":"https://metadata.fankai.fr/episodes/503/image"},{"id":504,"episode_number":4,"title":"No image"}]}`))
		default:
			t.Errorf("unexpected metadata request: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	e.svc.fankaiBaseURL = server.URL
	if err := e.svc.SetFanKaiMatch(ctx, seriesID, 77); err != nil {
		t.Fatal(err)
	}
	episode, err := e.q.GetItem(ctx, episodeID)
	if err != nil {
		t.Fatal(err)
	}
	if episode.Title != "Edited episode" || episode.Overview == nil || *episode.Overview != "Episode synopsis" ||
		episode.RuntimeMinutes == nil || *episode.RuntimeMinutes != 62 || episode.TmdbID != nil ||
		episode.ThumbPath == nil || !strings.HasSuffix(*episode.ThumbPath, "-501-thumb.png") {
		t.Fatalf("unexpected imported episode: %+v", episode)
	}
	if _, err := os.Stat(filepath.Join(cache.dir, *episode.ThumbPath)); err != nil {
		t.Fatalf("thumbnail not persisted: %v", err)
	}
	seasonRows, err := e.q.ListChildren(ctx, &seriesID)
	if err != nil || len(seasonRows) != 1 {
		t.Fatalf("unexpected seasons: %+v (%v)", seasonRows, err)
	}
	season, err := e.q.GetItem(ctx, seasonRows[0].ID)
	if err != nil || season.PosterPath == nil || !strings.HasSuffix(*season.PosterPath, "-90-poster.png") || season.Title != "FanKai season" {
		t.Fatalf("unexpected imported season: %+v (%v)", season, err)
	}
	var childCount int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM items WHERE parent_id=$1`, season.ID).Scan(&childCount); err != nil || childCount != 3 {
		t.Fatalf("only existing episodes should be imported: count=%d (%v)", childCount, err)
	}
	var secondThumb *string
	if err := e.pool.QueryRow(ctx, `SELECT thumb_path FROM items WHERE parent_id=$1 AND episode_number=2`, season.ID).Scan(&secondThumb); err != nil ||
		secondThumb == nil || !strings.HasSuffix(*secondThumb, "-502-thumb.png") {
		t.Fatalf("thumb_image fallback not imported: %v (%v)", secondThumb, err)
	}
	if err := e.svc.RefreshLibrary(ctx, mustLibrary(t, e.q, lib), func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	if metadataRequests.Load() != 3 || imageRequests.Load() != 3 {
		t.Fatalf("unexpected requests after automatic refresh: metadata=%d images=%d", metadataRequests.Load(), imageRequests.Load())
	}
	if err := e.svc.RefreshItem(ctx, seriesID, nil); err != nil {
		t.Fatal(err)
	}
	if metadataRequests.Load() != 6 || imageRequests.Load() != 3 {
		t.Fatalf("manual refresh did not reuse images: metadata=%d images=%d", metadataRequests.Load(), imageRequests.Load())
	}
	failEpisodes.Store(true)
	if _, err := e.pool.Exec(ctx, `UPDATE items SET title='Before failure' WHERE id=$1`, seriesID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RefreshItem(ctx, seriesID, nil); !errors.Is(err, ErrFanKaiUnavailable) {
		t.Fatalf("expected explicit upstream error: %v", err)
	}
	root, err := e.q.GetItem(ctx, seriesID)
	if err != nil || root.Title != "Before failure" {
		t.Fatalf("failed import must roll back metadata: %+v (%v)", root, err)
	}
}

func TestSearchFanKaiUsesCachedSeriesList(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/series" || r.URL.Query().Get("paginate") != "false" {
			t.Errorf("unexpected FanKai request: %s", r.URL.String())
		}
		_ = json.NewEncoder(w).Encode([]FanKaiSeries{
			{ID: 1, Title: "One Piece Henshū", OriginalTitle: "ワンピース", Year: 1999},
			{ID: 2, Title: "Another Series", Year: 2020},
		})
	}))
	defer server.Close()

	svc := &Service{fankaiBaseURL: server.URL}
	results, err := svc.SearchFanKai(context.Background(), "One Piece", 1999)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != 1 {
		t.Fatalf("unexpected FanKai results: %+v", results)
	}
	if _, err := svc.SearchFanKai(context.Background(), "One Piece", 1999); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected one cached list request, got %d", got)
	}
}

func TestFanKaiImageRequiresFixedHTTPSHost(t *testing.T) {
	cache := NewImageCache(t.TempDir(), "")
	if _, err := cache.FanKai(context.Background(), "00000000-0000-0000-0000-000000000001-poster", "http://metadata.fankai.fr/poster"); err == nil {
		t.Fatal("expected non-HTTPS image URL to be rejected")
	}
	if _, err := cache.FanKai(context.Background(), "00000000-0000-0000-0000-000000000001-poster", "https://example.com/poster"); err == nil {
		t.Fatal("expected non-FanKai image host to be rejected")
	}
}
