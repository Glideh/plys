package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/store"
	"toto/api/internal/testdb"
)

type fakeProber struct{}

func (fakeProber) Probe(_ context.Context, path string) (ProbeResult, error) {
	if strings.Contains(path, "corrupt") {
		return ProbeResult{}, errors.New("invalid data found when processing input")
	}
	return ProbeResult{Duration: 2400, Container: "matroska,webm", Streams: []Stream{{Index: 0, Type: "video", Codec: "hevc"}}}, nil
}

func touch(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func newLibrary(t *testing.T, pool *pgxpool.Pool, kind, root string) uuid.UUID {
	t.Helper()
	q := store.New(pool)
	lib, err := q.CreateLibrary(context.Background(), store.CreateLibraryParams{Name: kind + "-" + uuid.NewString(), Kind: kind})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.AddLibraryPath(context.Background(), store.AddLibraryPathParams{LibraryID: lib.ID, Path: root}); err != nil {
		t.Fatal(err)
	}
	return lib.ID
}

func counts(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) map[string]int64 {
	t.Helper()
	rows, err := store.New(pool).CountItemsByLibrary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, r := range rows {
		if r.LibraryID == id {
			out[r.Kind] = r.Count
		}
	}
	return out
}

func status(s *Scanner, id uuid.UUID) Status {
	for _, st := range s.Statuses() {
		if st.LibraryID == id {
			return st
		}
	}
	return Status{}
}

func TestScanSeriesLibrary(t *testing.T) {
	pool := testdb.New(t, "test_scanner")
	root := t.TempDir()
	touch(t, root,
		"24.iNTEGRALE.MULTi.1080p.WEB.H265-FW/24.S01.MULTi.1080p.WEB.H265-FW/24.S01E01.MULTi.1080p.WEB.H265-FW.mkv",
		"24.iNTEGRALE.MULTi.1080p.WEB.H265-FW/24.S01.MULTi.1080p.WEB.H265-FW/24.S01E02.MULTi.1080p.WEB.H265-FW.mkv",
		"24.iNTEGRALE.MULTi.1080p.WEB.H265-FW/24.S02.MULTi.1080p.WEB.H265-FW/24.S02E01.MULTi.1080p.WEB.H265-FW.mkv",
		"24.iNTEGRALE.MULTi.1080p.WEB.H265-FW/24.iNTEGRALE.MULTi.1080p.WEB.H265-FW.nfo",
		"1923.S01E03.MULTi.1080p.WEB.x264-FW/1923.S01E03.MULTi.1080p.WEB.x264-FW.mkv",
		"1923.S02.MULTi.1080p.WEB.H265-FW/1923.S02E01.corrupt.mkv",
		"random video.mkv",
		"@eaDir/24.S09E01.mkv",
		"1923.S02.MULTi.1080p.WEB.H265-FW/._1923.S02E02.mkv",
		"1923.S02.MULTi.1080p.WEB.H265-FW/1923.S02E01.sample.mkv",
	)
	id := newLibrary(t, pool, "series", root)
	s := New(pool, fakeProber{}, nil, 0)
	ctx := context.Background()

	if err := s.ScanLibrary(ctx, id); err != nil {
		t.Fatal(err)
	}
	st := status(s, id)
	if st.State != StateDone || st.Total != 6 || st.Probed != 6 || st.Added != 5 || st.Unmatched != 1 || st.Failed != 1 || st.Incomplete {
		t.Fatalf("first scan status: %+v", st)
	}
	if c := counts(t, pool, id); c["series"] != 2 || c["season"] != 4 || c["episode"] != 5 {
		t.Fatalf("first scan items: %v", c)
	}

	if err := s.ScanLibrary(ctx, id); err != nil {
		t.Fatal(err)
	}
	if st := status(s, id); st.Probed != 0 || st.Added != 0 || st.Missing != 0 {
		t.Fatalf("unchanged rescan must not reprobe: %+v", st)
	}

	if err := os.Remove(filepath.Join(root, "1923.S01E03.MULTi.1080p.WEB.x264-FW/1923.S01E03.MULTi.1080p.WEB.x264-FW.mkv")); err != nil {
		t.Fatal(err)
	}
	if err := s.ScanLibrary(ctx, id); err != nil {
		t.Fatal(err)
	}
	if st := status(s, id); st.Missing != 1 {
		t.Fatalf("expected one missing file, got %+v", st)
	}
	if c := counts(t, pool, id); c["episode"] != 5 {
		t.Fatalf("missing files keep their item during the grace period, got %v", c)
	}
}

func TestScanDoesNotFlagMissingWhenRootIsEmpty(t *testing.T) {
	pool := testdb.New(t, "test_scanner")
	root := t.TempDir()
	touch(t, root, "Alien.1979.1080p.mkv", "Aliens.1986.1080p.mkv")
	id := newLibrary(t, pool, "movies", root)
	s := New(pool, fakeProber{}, nil, 0)
	ctx := context.Background()

	if err := s.ScanLibrary(ctx, id); err != nil {
		t.Fatal(err)
	}
	if c := counts(t, pool, id); c["movie"] != 2 {
		t.Fatalf("expected 2 movies, got %v", c)
	}

	// Simulates an unmounted share: the folder still exists but is empty.
	for _, f := range []string{"Alien.1979.1080p.mkv", "Aliens.1986.1080p.mkv"} {
		if err := os.Remove(filepath.Join(root, f)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ScanLibrary(ctx, id); err != nil {
		t.Fatal(err)
	}
	if st := status(s, id); !st.Incomplete || st.Missing != 0 {
		t.Fatalf("empty root must not flag files as missing: %+v", st)
	}
}

func TestScanMovieParts(t *testing.T) {
	pool := testdb.New(t, "test_scanner")
	root := t.TempDir()
	touch(t, root,
		"[yggtorrent.com]Guerre et Paix.1966.DVD 1.MULTI.DVDRip.XVid-AC3/Guerre et Paix.avi",
		"[yggtorrent.com]Guerre et Paix.1966.DVD 2.MULTI.DVDRip.XVid-AC3/Guerre et Paix.avi",
		"Guerre.et.Paix.1966.1080p.BluRay.mkv",
		"Loose.Change.Final.Cut.2007.VOSTFR.DVDRIP.avi/Partie 1.avi",
		"Loose.Change.Final.Cut.2007.VOSTFR.DVDRIP.avi/Partie 2.avi",
	)
	id := newLibrary(t, pool, "movies", root)
	s := New(pool, fakeProber{}, nil, 0)
	ctx := context.Background()

	if err := s.ScanLibrary(ctx, id); err != nil {
		t.Fatal(err)
	}
	if st := status(s, id); st.Added != 2 {
		t.Fatalf("parts must not count as added movies: %+v", st)
	}
	if c := counts(t, pool, id); c["movie"] != 2 || c["part"] != 4 {
		t.Fatalf("expected 2 movies with 2 parts each, got %v", c)
	}

	var direct int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM media_files f JOIN items i ON i.id = f.item_id WHERE i.kind = 'movie'`).Scan(&direct)
	if err != nil || direct != 1 {
		t.Fatalf("the single-file version must stay attached to the movie itself, got %d (%v)", direct, err)
	}

	// Parts whose files are purged disappear, and so does a movie left without files or parts.
	if _, err := pool.Exec(ctx, `DELETE FROM media_files WHERE path LIKE '%Loose.Change%'`); err != nil {
		t.Fatal(err)
	}
	if err := s.cleanup(ctx, id); err != nil {
		t.Fatal(err)
	}
	if c := counts(t, pool, id); c["movie"] != 1 || c["part"] != 2 {
		t.Fatalf("orphan parts and movie must be removed, got %v", c)
	}
}

func TestEnqueueDeduplicates(t *testing.T) {
	s := New(nil, fakeProber{}, nil, 0)
	id := uuid.New()
	s.Enqueue(id)
	s.Enqueue(id)
	if len(s.queue) != 1 {
		t.Fatalf("expected a single queued scan, got %d", len(s.queue))
	}
}
