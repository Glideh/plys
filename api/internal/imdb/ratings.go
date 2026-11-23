// Package imdb keeps the IMDb public ratings of the items up to date from IMDb's free daily dataset
// (title.ratings.tsv.gz, no API key needed), joined on the items' IMDb IDs.
package imdb

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"toto/api/internal/store"
)

const (
	// DatasetURL is IMDb's daily ratings dataset, free for non-commercial use.
	DatasetURL = "https://datasets.imdbws.com/title.ratings.tsv.gz"
	// datasetMaxAge is how long a downloaded dataset is used before it is fetched again; IMDb updates it daily.
	datasetMaxAge = 24 * time.Hour
	// checkInterval is how often the job looks for a stale dataset or items it has not rated yet.
	checkInterval   = time.Hour
	downloadTimeout = 10 * time.Minute
)

// Ratings downloads the dataset into the cache folder and applies it to the items.
type Ratings struct {
	q      *store.Queries
	url    string
	path   string
	client *http.Client
	now    func() time.Time

	// applied holds the IMDb IDs the last application of the dataset covered, so that new IDs are rated without
	// waiting for the next download.
	applied map[string]struct{}
}

// New returns the job; url is the dataset address (DatasetURL outside tests).
func New(q *store.Queries, cacheDir, url string) *Ratings {
	return &Ratings{
		q: q, url: url, path: filepath.Join(cacheDir, "imdb", "title.ratings.tsv.gz"),
		client: &http.Client{Timeout: downloadTimeout}, now: time.Now,
	}
}

// Run syncs the ratings now and then every checkInterval until ctx is done.
func (r *Ratings) Run(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		if err := r.Sync(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("imdb ratings sync", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Sync downloads the dataset when it is missing or older than a day, then applies it when it changed or when items
// carry IMDb IDs it was not applied to yet.
func (r *Ratings) Sync(ctx context.Context) error {
	ids, err := r.q.ListItemIMDbIDs(ctx)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	fresh := false
	info, err := os.Stat(r.path)
	if err != nil || r.now().Sub(info.ModTime()) > datasetMaxAge {
		if err := r.download(ctx); err != nil {
			// A stale copy still rates the new items until the download succeeds.
			if info == nil {
				return err
			}
			slog.Warn("imdb ratings download", "err", err)
		} else {
			fresh = true
		}
	}
	if !fresh && r.covers(ids) {
		return nil
	}
	return r.apply(ctx, ids)
}

func (r *Ratings) covers(ids []string) bool {
	if r.applied == nil {
		return false
	}
	for _, id := range ids {
		if _, ok := r.applied[id]; !ok {
			return false
		}
	}
	return true
}

// download replaces the cached dataset, through a temporary file so that a failed download keeps the previous one.
func (r *Ratings) download(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", r.url, resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(r.path), "ratings-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// A truncated or corrupt archive must not replace a good one.
	if _, err := readDataset(tmp.Name(), nil); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), r.path)
}

func (r *Ratings) apply(ctx context.Context, ids []string) error {
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	rows, err := readDataset(r.path, wanted)
	if err != nil {
		return err
	}
	params := store.SetIMDbRatingsParams{
		ImdbIds: make([]string, 0, len(rows)), Ratings: make([]float32, 0, len(rows)), Votes: make([]int32, 0, len(rows)),
	}
	for id, v := range rows {
		params.ImdbIds = append(params.ImdbIds, id)
		params.Ratings = append(params.Ratings, v.rating)
		params.Votes = append(params.Votes, v.votes)
	}
	n, err := r.q.SetIMDbRatings(ctx, params)
	if err != nil {
		return err
	}
	r.applied = wanted
	slog.Info("imdb ratings applied", "rated", len(rows), "ids", len(ids), "updated", n)
	return nil
}

type rating struct {
	rating float32
	votes  int32
}

// readDataset reads the tconst, averageRating and numVotes columns of the gzipped TSV, keeping only the wanted IDs
// (every line is still checked when wanted is nil).
func readDataset(path string, wanted map[string]struct{}) (map[string]rating, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("imdb dataset: %w", err)
	}
	defer gz.Close()
	out := map[string]rating{}
	sc := bufio.NewScanner(gz)
	header := true
	for sc.Scan() {
		line := sc.Text()
		if header {
			header = false
			if !strings.HasPrefix(line, "tconst\t") {
				return nil, fmt.Errorf("imdb dataset: unexpected header %q", line)
			}
			continue
		}
		id, rest, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		if wanted != nil {
			if _, ok := wanted[id]; !ok {
				continue
			}
		}
		avg, votes, ok := strings.Cut(rest, "\t")
		if !ok {
			continue
		}
		a, err := strconv.ParseFloat(avg, 32)
		if err != nil {
			continue
		}
		n, err := strconv.ParseInt(votes, 10, 32)
		if err != nil {
			continue
		}
		if wanted != nil {
			out[id] = rating{rating: float32(a), votes: int32(n)}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("imdb dataset: %w", err)
	}
	if header {
		return nil, fmt.Errorf("imdb dataset: empty")
	}
	return out, nil
}
