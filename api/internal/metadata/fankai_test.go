package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

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
