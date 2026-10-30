package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/store"
)

const (
	probeWorkers = 4
	// Files that stay missing this long are dropped, along with their watch history.
	missingGrace = 7 * 24 * time.Hour
	queueSize    = 64
)

type State string

const (
	StateQueued  State = "queued"
	StateRunning State = "running"
	StateDone    State = "done"
	StateFailed  State = "failed"
)

type Status struct {
	LibraryID uuid.UUID `json:"libraryId"`
	State     State     `json:"state"`
	// Stage is "files" while walking and probing, then "metadata".
	Stage      string     `json:"stage,omitempty"`
	Total      int        `json:"total"`
	Processed  int        `json:"processed"`
	Added      int        `json:"added"`
	Probed     int        `json:"probed"`
	Unmatched  int        `json:"unmatched"`
	Failed     int        `json:"failed"`
	Missing    int64      `json:"missing"`
	Incomplete bool       `json:"incomplete"`
	MetaTotal  int        `json:"metaTotal"`
	MetaDone   int        `json:"metaDone"`
	MetaError  string     `json:"metaError,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// Enricher fetches metadata once files are catalogued.
type Enricher interface {
	RefreshLibrary(ctx context.Context, lib store.Library, progress func(done, total int)) error
}

// Scanner runs one library scan at a time so a NAS is never hit by parallel walks.
type Scanner struct {
	pool     *pgxpool.Pool
	q        *store.Queries
	prober   Prober
	enricher Enricher
	interval time.Duration

	mu       sync.Mutex
	statuses map[uuid.UUID]*Status
	queue    chan uuid.UUID
}

func New(pool *pgxpool.Pool, prober Prober, enricher Enricher, interval time.Duration) *Scanner {
	return &Scanner{
		pool:     pool,
		q:        store.New(pool),
		prober:   prober,
		enricher: enricher,
		interval: interval,
		statuses: map[uuid.UUID]*Status{},
		queue:    make(chan uuid.UUID, queueSize),
	}
}

// Enqueue schedules a scan unless one is already queued or running for this library.
func (s *Scanner) Enqueue(id uuid.UUID) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st := s.statuses[id]; st != nil && (st.State == StateQueued || st.State == StateRunning) {
		return *st
	}
	st := &Status{LibraryID: id, State: StateQueued}
	select {
	case s.queue <- id:
	default:
		now := time.Now()
		st.State, st.Error, st.FinishedAt = StateFailed, "scan queue is full", &now
	}
	s.statuses[id] = st
	return *st
}

func (s *Scanner) Statuses() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Status, 0, len(s.statuses))
	for _, st := range s.statuses {
		out = append(out, *st)
	}
	return out
}

func (s *Scanner) Forget(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.statuses, id)
}

// Run processes the queue and, when an interval is set, rescans every library periodically.
func (s *Scanner) Run(ctx context.Context) {
	var tick <-chan time.Time
	if s.interval > 0 {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		tick = t.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick:
			s.EnqueueAll(ctx)
		case id := <-s.queue:
			if err := s.ScanLibrary(ctx, id); err != nil && ctx.Err() == nil {
				slog.Error("library scan failed", "library_id", id, "err", err)
			}
		}
	}
}

// EnqueueAll schedules every library, e.g. after the metadata settings changed.
func (s *Scanner) EnqueueAll(ctx context.Context) {
	libs, err := s.q.ListLibraries(ctx)
	if err != nil {
		slog.Error("list libraries for periodic scan", "err", err)
		return
	}
	for _, l := range libs {
		s.Enqueue(l.ID)
	}
}

func (s *Scanner) update(id uuid.UUID, fn func(*Status)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.statuses[id]
	if st == nil {
		st = &Status{LibraryID: id}
		s.statuses[id] = st
	}
	fn(st)
}

// ScanLibrary synchronously reconciles the catalogue of a library with its folders.
func (s *Scanner) ScanLibrary(ctx context.Context, id uuid.UUID) (err error) {
	started := time.Now()
	s.update(id, func(st *Status) {
		*st = Status{LibraryID: id, State: StateRunning, Stage: "files", StartedAt: &started}
	})
	defer func() {
		finished := time.Now()
		s.update(id, func(st *Status) {
			st.FinishedAt = &finished
			st.State = StateDone
			if err != nil {
				st.State, st.Error = StateFailed, err.Error()
			}
		})
		var snapshot Status
		s.update(id, func(st *Status) { snapshot = *st })
		slog.Info("library scan finished", "library_id", id, "duration", time.Since(started),
			"files", snapshot.Total, "probed", snapshot.Probed, "added", snapshot.Added,
			"unmatched", snapshot.Unmatched, "failed", snapshot.Failed, "missing", snapshot.Missing, "incomplete", snapshot.Incomplete)
	}()

	lib, err := s.q.GetLibrary(ctx, id)
	if err != nil {
		return fmt.Errorf("load library: %w", err)
	}
	roots, err := s.q.ListLibraryPaths(ctx, id)
	if err != nil {
		return fmt.Errorf("load library paths: %w", err)
	}
	rows, err := s.q.ListLibraryFiles(ctx, id)
	if err != nil {
		return fmt.Errorf("load library files: %w", err)
	}
	known := make(map[string]store.ListLibraryFilesRow, len(rows))
	for _, r := range rows {
		known[r.Path] = r
	}

	var found []foundFile
	complete := true
	for _, root := range roots {
		files, ok := walk(ctx, root)
		if !ok {
			slog.Warn("library folder unavailable or incomplete, missing files will not be flagged", "library_id", id, "path", root)
			complete = false
		}
		found = append(found, files...)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.update(id, func(st *Status) { st.Total, st.Incomplete = len(found), !complete })

	seen := s.processAll(ctx, lib, found, known)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := s.q.ClearFilesMissing(ctx, seen); err != nil {
		return fmt.Errorf("clear missing flags: %w", err)
	}
	if complete {
		n, err := s.q.MarkFilesMissing(ctx, store.MarkFilesMissingParams{LibraryID: id, Seen: seen})
		if err != nil {
			return fmt.Errorf("flag missing files: %w", err)
		}
		s.update(id, func(st *Status) { st.Missing = n })
	}
	if err := s.cleanup(ctx, id); err != nil {
		return err
	}
	if s.enricher == nil {
		return nil
	}
	s.update(id, func(st *Status) { st.Stage = "metadata" })
	// Metadata failures (TMDB down, invalid key) are reported without failing the file scan.
	if err := s.enricher.RefreshLibrary(ctx, lib, func(done, total int) {
		s.update(id, func(st *Status) { st.MetaDone, st.MetaTotal = done, total })
	}); err != nil && ctx.Err() == nil {
		s.update(id, func(st *Status) { st.MetaError = err.Error() })
	}
	return ctx.Err()
}

func (s *Scanner) cleanup(ctx context.Context, id uuid.UUID) error {
	if _, err := s.q.PurgeMissingFiles(ctx, store.PurgeMissingFilesParams{LibraryID: id, MissingSince: ptr(time.Now().Add(-missingGrace))}); err != nil {
		return fmt.Errorf("purge missing files: %w", err)
	}
	for range 2 {
		if _, err := s.q.DeleteOrphanLeaves(ctx, id); err != nil {
			return fmt.Errorf("delete orphan items: %w", err)
		}
	}
	for range 2 {
		if _, err := s.q.DeleteEmptyContainers(ctx, id); err != nil {
			return fmt.Errorf("delete empty containers: %w", err)
		}
	}
	return s.q.SetLibraryScanned(ctx, id)
}

func (s *Scanner) processAll(ctx context.Context, lib store.Library, found []foundFile, known map[string]store.ListLibraryFilesRow) []uuid.UUID {
	jobs := make(chan foundFile)
	var mu sync.Mutex
	seen := make([]uuid.UUID, 0, len(found))
	var wg sync.WaitGroup
	for range probeWorkers {
		wg.Go(func() {
			for f := range jobs {
				prev, hasPrev := known[f.path]
				var prevPtr *store.ListLibraryFilesRow
				if hasPrev {
					prevPtr = &prev
				}
				fileID, err := s.processFile(ctx, lib, f, prevPtr)
				if err != nil {
					if ctx.Err() == nil {
						slog.Error("process file", "path", f.path, "err", err)
					}
					// A transient database error must not flag a file that is still on disk.
					if hasPrev {
						fileID = prev.ID
					}
				}
				mu.Lock()
				if fileID != uuid.Nil {
					seen = append(seen, fileID)
				}
				mu.Unlock()
				s.update(lib.ID, func(st *Status) { st.Processed++ })
			}
		})
	}
	for _, f := range found {
		if ctx.Err() != nil {
			break
		}
		jobs <- f
	}
	close(jobs)
	wg.Wait()
	return seen
}

func (s *Scanner) processFile(ctx context.Context, lib store.Library, f foundFile, prev *store.ListLibraryFilesRow) (uuid.UUID, error) {
	unchanged := prev != nil && prev.Size == f.size && prev.Mtime.Equal(f.mtime) && prev.ProbedAt != nil
	if unchanged && prev.ItemID != nil {
		return prev.ID, nil
	}

	itemID, err := s.resolveItem(ctx, lib, f.rel)
	if err != nil {
		return uuid.Nil, err
	}
	if itemID == nil {
		s.update(lib.ID, func(st *Status) { st.Unmatched++ })
	}
	version := ParseFileVersion(f.path)
	if unchanged {
		if itemID != nil {
			err = s.q.LinkFileItem(ctx, store.LinkFileItemParams{
				ID: prev.ID, ItemID: itemID, VersionKey: version.Key, PartNumber: optInt(version.Part), Edition: optString(version.Edition),
			})
		}
		return prev.ID, err
	}

	params := store.UpsertProbedFileParams{
		LibraryID: lib.ID, ItemID: itemID, Path: f.path, Size: f.size, Mtime: f.mtime, Streams: []byte("[]"),
		VersionKey: version.Key, PartNumber: optInt(version.Part), Edition: optString(version.Edition),
	}
	res, probeErr := s.prober.Probe(ctx, f.path)
	if probeErr != nil {
		if ctx.Err() != nil {
			return uuid.Nil, ctx.Err()
		}
		params.ProbeError = ptr(probeErr.Error())
		s.update(lib.ID, func(st *Status) { st.Failed++ })
	} else {
		params.DurationSeconds, params.Container, params.Bitrate = &res.Duration, &res.Container, &res.Bitrate
		if params.Streams, err = json.Marshal(res.Streams); err != nil {
			return uuid.Nil, err
		}
	}
	s.update(lib.ID, func(st *Status) { st.Probed++ })
	return s.q.UpsertProbedFile(ctx, params)
}

// resolveItem returns the catalogue item a file belongs to, creating it (and its parents) when needed.
// A nil id means the file name could not be understood.
func (s *Scanner) resolveItem(ctx context.Context, lib store.Library, rel string) (*uuid.UUID, error) {
	upsert := func(p store.UpsertItemParams) (uuid.UUID, error) {
		p.LibraryID = lib.ID
		row, err := s.q.UpsertItem(ctx, p)
		if err == nil && row.Inserted && (p.Kind == "movie" || p.Kind == "episode" || p.Kind == "video") {
			s.update(lib.ID, func(st *Status) { st.Added++ })
		}
		return row.ID, err
	}

	switch lib.Kind {
	case "movies":
		info := ParseMovie(rel)
		if info.Title == "" {
			info.Title = stem(filepath.Base(rel))
		}
		id, err := upsert(store.UpsertItemParams{
			Kind: "movie", MatchKey: fmt.Sprintf("%s|%d", MatchKey(info.Title), info.Year),
			Title: info.Title, SortTitle: SortTitle(info.Title), Year: optInt(info.Year),
		})
		if err != nil {
			return nil, err
		}
		// Parts are listed under the movie like episodes, each with its own progress.
		if part := ParseFileVersion(rel).Part; part > 0 {
			id, err = upsert(store.UpsertItemParams{
				ParentID: &id, Kind: "part", MatchKey: fmt.Sprintf("p%d", part),
				SortTitle: fmt.Sprintf("%04d", part), PartNumber: ptr(int32(part)),
			})
		}
		return &id, err

	case "series":
		info, ok := ParseEpisode(rel)
		if !ok {
			return nil, nil
		}
		seriesID, err := upsert(store.UpsertItemParams{
			Kind: "series", MatchKey: MatchKey(info.SeriesTitle),
			Title: info.SeriesTitle, SortTitle: SortTitle(info.SeriesTitle), Year: optInt(info.Year),
		})
		if err != nil {
			return nil, err
		}
		seasonID, err := upsert(store.UpsertItemParams{
			ParentID: &seriesID, Kind: "season", MatchKey: fmt.Sprintf("s%d", info.Season),
			SortTitle: fmt.Sprintf("%04d", info.Season), SeasonNumber: ptr(int32(info.Season)),
		})
		if err != nil {
			return nil, err
		}
		id, err := upsert(store.UpsertItemParams{
			ParentID: &seasonID, Kind: "episode", MatchKey: fmt.Sprintf("e%d", info.Episode),
			SortTitle: fmt.Sprintf("%04d", info.Episode), SeasonNumber: ptr(int32(info.Season)),
			EpisodeNumber: ptr(int32(info.Episode)), EpisodeNumberEnd: optInt(info.EpisodeEnd),
		})
		return &id, err

	default:
		title := stem(filepath.Base(rel))
		id, err := upsert(store.UpsertItemParams{Kind: "video", MatchKey: rel, Title: title, SortTitle: MatchKey(title)})
		return &id, err
	}
}

type foundFile struct {
	path  string
	rel   string
	size  int64
	mtime time.Time
}

// walk lists video files below root. ok is false when the root is unreachable or empty, or when a
// subfolder could not be read: an unmounted share must never look like a library whose files were deleted.
func walk(ctx context.Context, root string) (files []foundFile, ok bool) {
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) == 0 {
		return nil, false
	}
	ok = true
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			ok = false
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != root && isIgnoredDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || !isVideo(d.Name()) {
			return nil
		}
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		files = append(files, foundFile{
			path:  p,
			rel:   rel,
			size:  info.Size(),
			mtime: info.ModTime().Truncate(time.Microsecond),
		})
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		ok = false
	}
	return files, ok
}

func ptr[T any](v T) *T {
	return &v
}

func optInt(v int) *int32 {
	if v == 0 {
		return nil
	}
	return ptr(int32(v))
}

func optString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
