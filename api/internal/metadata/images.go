package metadata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const DefaultImageBaseURL = "https://image.tmdb.org/t/p"

// Sizes offered by TMDB; restricting them keeps the cache bounded.
var ImageSizes = []string{"w92", "w185", "w300", "w342", "w500", "w780", "w1280", "original"}

var imageFileRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}\.(?:jpg|png)$`)

// ImageCache downloads TMDB images once and serves them from disk, so browsers never contact TMDB.
type ImageCache struct {
	dir     string
	baseURL string
	http    *http.Client
}

func NewImageCache(dir, baseURL string) *ImageCache {
	if baseURL == "" {
		baseURL = DefaultImageBaseURL
	}
	return &ImageCache{dir: dir, baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 30 * time.Second}}
}

func ValidImageRequest(size, file string) bool {
	return slices.Contains(ImageSizes, size) && imageFileRe.MatchString(file)
}

// TMDB returns the local path of the image, downloading it on first use.
func (c *ImageCache) TMDB(ctx context.Context, size, file string) (string, error) {
	if !ValidImageRequest(size, file) {
		return "", fmt.Errorf("invalid image %s/%s", size, file)
	}
	path := filepath.Join(c.dir, "tmdb", size, file)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+size+"/"+file, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("TMDB image %s/%s: HTTP %d", size, file, resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, 20<<20)); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmp.Name(), path)
}

// Local returns the path of a thumbnail extracted by ffmpeg.
func (c *ImageCache) Local(file string) string {
	return filepath.Join(c.dir, "local", filepath.Base(file))
}
