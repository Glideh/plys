// Package release watches the GitHub releases of Plys, so that clients can tell when a newer server is published.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LatestURL is the GitHub endpoint of the latest published release (drafts and pre-releases excluded).
const LatestURL = "https://api.github.com/repos/Glideh/plys/releases/latest"

const checkInterval = 24 * time.Hour

// Update is a published release newer than the running server.
type Update struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// Checker reads the latest release once at startup and then once a day, with its ETag so that an unchanged release
// costs a 304. A failed check keeps the last known release.
type Checker struct {
	url       string
	current   string
	userAgent string
	http      *http.Client

	mu     sync.RWMutex
	etag   string
	latest *Update
}

// New watches the releases published at url for a server running version current.
func New(url, current string) *Checker {
	return &Checker{
		url:       url,
		current:   current,
		userAgent: "Plys-Server/" + current,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Run checks at once, then every day until ctx is done.
func (c *Checker) Run(ctx context.Context) {
	for {
		if err := c.Check(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("release check failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(checkInterval):
		}
	}
}

// Check reads the latest release once.
func (c *Checker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", c.userAgent)
	c.mu.RLock()
	if c.etag != "" {
		req.Header.Set("If-None-Match", c.etag)
	}
	c.mu.RUnlock()
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil
	case http.StatusOK:
	default:
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var body struct {
		TagName    string `json:"tag_name"`
		HTMLURL    string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return err
	}
	var latest *Update
	if !body.Draft && !body.Prerelease && Newer(body.TagName, c.current) {
		latest = &Update{Version: strings.TrimPrefix(body.TagName, "v"), URL: body.HTMLURL}
	}
	c.mu.Lock()
	c.etag = resp.Header.Get("ETag")
	c.latest = latest
	c.mu.Unlock()
	return nil
}

// Available returns the newer release, or nil when the server is up to date, its version is unknown (a development
// build) or no check has succeeded yet. A nil Checker has no update.
func (c *Checker) Available() *Update {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest
}

// Newer tells whether release is a higher x.y.z version than current. Anything else (a development build, a
// pre-release suffix) is never newer.
func Newer(release, current string) bool {
	r, ok := parse(release)
	if !ok {
		return false
	}
	c, ok := parse(current)
	if !ok {
		return false
	}
	for i := range r {
		if r[i] != c[i] {
			return r[i] > c[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || strings.HasPrefix(p, "+") {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
