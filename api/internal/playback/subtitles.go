package playback

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const subtitleTimeout = 10 * time.Minute

// Subtitles extracts text subtitle tracks to WebVTT once and keeps them in the cache. Extracting a track
// from an MKV reads the whole file, which takes a while on a network share, hence the cache.
type Subtitles struct {
	ffmpeg string
	dir    string

	mu      sync.Mutex
	pending map[string]chan struct{}
	errs    map[string]error
}

func NewSubtitles(ffmpeg, cacheDir string) (*Subtitles, error) {
	dir := filepath.Join(cacheDir, "subtitles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Subtitles{ffmpeg: ffmpeg, dir: dir, pending: map[string]chan struct{}{}, errs: map[string]error{}}, nil
}

// The file size is part of the key so that a replaced file is extracted again.
func (s *Subtitles) path(fileID uuid.UUID, size int64, index int) string {
	return filepath.Join(s.dir, fmt.Sprintf("%s-%d-%d.vtt", fileID, size, index))
}

// Prepare starts the extraction in the background if the track is not cached yet.
func (s *Subtitles) Prepare(fileID uuid.UUID, size int64, source string, index int) {
	out := s.path(fileID, size, index)
	if _, err := os.Stat(out); err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, running := s.pending[out]; running {
		return
	}
	done := make(chan struct{})
	s.pending[out] = done
	delete(s.errs, out)
	go func() {
		err := s.extract(source, index, out)
		s.mu.Lock()
		delete(s.pending, out)
		if err != nil {
			s.errs[out] = err
		}
		s.mu.Unlock()
		close(done)
	}()
}

// Cached returns the extracted track without waiting.
func (s *Subtitles) Cached(fileID uuid.UUID, size int64, index int) (string, bool) {
	out := s.path(fileID, size, index)
	_, err := os.Stat(out)
	return out, err == nil
}

// Wait returns the cached WebVTT path, waiting at most until ctx is done.
func (s *Subtitles) Wait(ctx context.Context, fileID uuid.UUID, size int64, source string, index int) (string, error) {
	s.Prepare(fileID, size, source, index)
	out := s.path(fileID, size, index)
	s.mu.Lock()
	done, running := s.pending[out]
	err := s.errs[out]
	s.mu.Unlock()
	if running {
		select {
		case <-done:
		case <-ctx.Done():
			return "", ErrNotReady
		}
		s.mu.Lock()
		err = s.errs[out]
		s.mu.Unlock()
	}
	if err != nil {
		return "", err
	}
	return out, nil
}

func (s *Subtitles) extract(source string, index int, out string) error {
	ctx, cancel := context.WithTimeout(context.Background(), subtitleTimeout)
	defer cancel()
	tmp := out + ".tmp"
	cmd := exec.CommandContext(ctx, s.ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", source, "-map", fmt.Sprintf("0:%d", index), "-c:s", "webvtt", "-f", "webvtt", tmp)
	if output, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("extract subtitle %d: %v: %s", index, err, strings.TrimSpace(string(output)))
	}
	return os.Rename(tmp, out)
}

var cueTimingRe = regexp.MustCompile(`^((?:\d+:)?\d{2}:\d{2}\.\d{3})\s+-->\s+((?:\d+:)?\d{2}:\d{2}\.\d{3})(.*)$`)

// ShiftVTT moves every cue back by offset seconds, so cues line up with a stream that starts at offset.
// Cues that end before the start are dropped.
func ShiftVTT(r io.Reader, w io.Writer, offset float64) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	bw := bufio.NewWriter(w)
	skipping := false
	for sc.Scan() {
		line := sc.Text()
		if m := cueTimingRe.FindStringSubmatch(line); m != nil {
			start, end := parseVTTTime(m[1])-offset, parseVTTTime(m[2])-offset
			skipping = end <= 0
			if skipping {
				continue
			}
			line = formatVTTTime(max(start, 0)) + " --> " + formatVTTTime(end) + m[3]
		} else if skipping {
			if strings.TrimSpace(line) == "" {
				skipping = false
			}
			continue
		}
		if _, err := bw.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return bw.Flush()
}

func parseVTTTime(s string) float64 {
	parts := strings.Split(s, ":")
	total := 0.0
	for _, p := range parts {
		v, _ := strconv.ParseFloat(p, 64)
		total = total*60 + v
	}
	return total
}

func formatVTTTime(t float64) string {
	ms := int64(t*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
