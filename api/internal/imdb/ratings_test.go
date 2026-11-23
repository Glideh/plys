package imdb

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/store"
	"toto/api/internal/testdb"
)

func gzipped(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func insertItem(t *testing.T, pool *pgxpool.Pool, lib, key string, imdbID *string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO items (library_id, kind, match_key, title, sort_title, imdb_id) VALUES ($1, 'movie', $2, $2, $2, $3)`,
		lib, key, imdbID)
	if err != nil {
		t.Fatal(err)
	}
}

func itemRating(t *testing.T, pool *pgxpool.Pool, key string) (*float32, *int32) {
	t.Helper()
	var r *float32
	var v *int32
	if err := pool.QueryRow(context.Background(), `SELECT imdb_rating, imdb_votes FROM items WHERE match_key = $1`, key).Scan(&r, &v); err != nil {
		t.Fatal(err)
	}
	return r, v
}

func TestSync(t *testing.T) {
	pool := testdb.New(t, "imdb_ratings")
	ctx := context.Background()
	var lib string
	if err := pool.QueryRow(ctx, `INSERT INTO libraries (name, kind) VALUES ('Films', 'movies') RETURNING id`).Scan(&lib); err != nil {
		t.Fatal(err)
	}
	alien, unknown := "tt0078748", "tt9999999"
	insertItem(t, pool, lib, "alien", &alien)
	insertItem(t, pool, lib, "unknown", &unknown)
	insertItem(t, pool, lib, "none", nil)

	dataset := "tconst\taverageRating\tnumVotes\ntt0078748\t8.5\t1012345\ntt0083658\t8.1\t850000\n"
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		_, _ = w.Write(gzipped(t, dataset))
	}))
	defer srv.Close()

	r := New(store.New(pool), t.TempDir(), srv.URL)
	if err := r.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if rating, votes := itemRating(t, pool, "alien"); rating == nil || *rating != 8.5 || votes == nil || *votes != 1012345 {
		t.Fatalf("alien = %v, %v", rating, votes)
	}
	for _, key := range []string{"unknown", "none"} {
		if rating, votes := itemRating(t, pool, key); rating != nil || votes != nil {
			t.Fatalf("%s = %v, %v, want no rating", key, rating, votes)
		}
	}

	// A fresh dataset is reused; a new IMDb ID is rated from it without downloading again.
	blade := "tt0083658"
	insertItem(t, pool, lib, "blade", &blade)
	if err := r.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if rating, _ := itemRating(t, pool, "blade"); rating == nil || *rating != 8.1 {
		t.Fatalf("blade = %v", rating)
	}
	if n := downloads.Load(); n != 1 {
		t.Fatalf("downloads = %d, want 1", n)
	}

	// After a day the dataset is downloaded again and its changes applied.
	dataset = "tconst\taverageRating\tnumVotes\ntt0078748\t8.4\t1020000\n"
	r.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if err := r.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if n := downloads.Load(); n != 2 {
		t.Fatalf("downloads = %d, want 2", n)
	}
	if rating, votes := itemRating(t, pool, "alien"); *rating != 8.4 || *votes != 1020000 {
		t.Fatalf("alien = %v, %v", *rating, *votes)
	}
	if rating, _ := itemRating(t, pool, "blade"); rating != nil {
		t.Fatalf("blade = %v, want no rating once it left the dataset", *rating)
	}
}

func TestDownloadKeepsPreviousDatasetOnCorruptArchive(t *testing.T) {
	body := gzipped(t, "tconst\taverageRating\tnumVotes\ntt0078748\t8.5\t10\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer srv.Close()
	r := New(nil, t.TempDir(), srv.URL)
	if err := r.download(context.Background()); err != nil {
		t.Fatal(err)
	}
	body = body[:len(body)/2]
	if err := r.download(context.Background()); err == nil {
		t.Fatal("truncated archive accepted")
	}
	got, err := readDataset(r.path, map[string]struct{}{"tt0078748": {}})
	if err != nil || got["tt0078748"].votes != 10 {
		t.Fatalf("dataset = %v, %v", got, err)
	}
	if entries, _ := os.ReadDir(r.path[:len(r.path)-len("/title.ratings.tsv.gz")]); len(entries) != 1 {
		t.Fatalf("leftover files: %v", entries)
	}
}
