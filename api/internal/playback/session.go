package playback

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

const (
	// Sessions are dropped when the player stops requesting the playlist or segments.
	idleTimeout     = 5 * time.Minute
	janitorInterval = 2 * time.Second
	// ffmpeg is paused once it is this many segments ahead of the player, and resumed below the second value.
	pauseAhead     = 45
	resumeAhead    = 25
	waitFile       = 20 * time.Second
	keyframeWindow = 20.0
)

var (
	ErrBusy     = errors.New("too many transcodes running")
	ErrNotFound = errors.New("playback session not found")
	ErrNotReady = errors.New("stream not ready yet")

	streamFileRe = regexp.MustCompile(`^(index\.m3u8|init\.mp4|seg_(\d{5})\.m4s)$`)
)

type Session struct {
	ID     string
	UserID uuid.UUID
	FileID uuid.UUID
	// Start is the source position matching the beginning of the stream.
	Start float64
	Plan  Plan
	// Source details needed to serve the text subtitle tracks listed in TextSubtitles.
	SourcePath    string
	SourceSize    int64
	TextSubtitles []int
	// LiveSubtitleIndex is the text track written alongside the segments, -1 when none.
	LiveSubtitleIndex int
	// ExternalSubtitle is the downloaded subtitle served with the stream, uuid.Nil when none.
	ExternalSubtitle uuid.UUID
	dir              string

	cmd    *exec.Cmd
	done   chan struct{}
	stderr *tailBuffer

	mu            sync.Mutex
	lastAccess    time.Time
	lastRequested int
	paused        bool
}

func (s *Session) Transcoding() bool {
	return !s.Plan.CopyVideo || !s.Plan.CopyAudio
}

type Manager struct {
	ffmpeg, ffprobe string
	Encoder         Encoder
	dir             string
	maxTranscodes   int

	mu       sync.Mutex
	sessions map[string]*Session
}

func NewManager(ffmpeg, ffprobe string, enc Encoder, cacheDir string, maxTranscodes int) (*Manager, error) {
	dir := filepath.Join(cacheDir, "transcode")
	// Leftovers of a previous run cannot be resumed.
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Manager{ffmpeg: ffmpeg, ffprobe: ffprobe, Encoder: enc, dir: dir, maxTranscodes: maxTranscodes, sessions: map[string]*Session{}}, nil
}

type StartRequest struct {
	UserID        uuid.UUID
	FileID        uuid.UUID
	Source        Source
	Size          int64
	TextSubtitles []int
	// ExternalSubtitle is a downloaded subtitle track to serve, uuid.Nil when none.
	ExternalSubtitle uuid.UUID
	Caps             ClientCaps
	Start            float64
}

func (m *Manager) Start(ctx context.Context, req StartRequest) (*Session, error) {
	plan := Decide(req.Source, req.Caps)
	sess := &Session{
		UserID: req.UserID, FileID: req.FileID, Plan: plan, done: make(chan struct{}), stderr: &tailBuffer{max: 4096},
		SourcePath: req.Source.Path, SourceSize: req.Size, TextSubtitles: req.TextSubtitles, LiveSubtitleIndex: -1,
		ExternalSubtitle: req.ExternalSubtitle,
	}
	if req.Source.TextSubtitle != nil {
		sess.LiveSubtitleIndex = req.Source.TextSubtitle.Index
	}

	// Only video transcodes are limited: audio-only conversion is cheap.
	m.mu.Lock()
	if !plan.CopyVideo && m.activeVideoTranscodes() >= m.maxTranscodes {
		m.mu.Unlock()
		return nil, ErrBusy
	}
	m.mu.Unlock()

	start := max(req.Start, 0)
	if req.Source.Duration > 0 {
		start = min(start, max(req.Source.Duration-1, 0))
	}
	// A copied stream starts at the keyframe ffmpeg seeks to, even when the other stream is decoded from the
	// requested time: starting the session on that keyframe keeps clock, audio and subtitles in sync.
	if (plan.CopyVideo || plan.CopyAudio) && start > 0 && req.Source.Video != nil {
		start = m.keyframeBefore(ctx, req.Source.Path, req.Source.Video.Index, start)
	}
	sess.Start = start

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	sess.ID = hex.EncodeToString(id)
	sess.dir = filepath.Join(m.dir, sess.ID)
	if err := os.MkdirAll(sess.dir, 0o755); err != nil {
		return nil, err
	}

	sess.cmd = exec.Command(m.ffmpeg, Args(plan, req.Source, start, m.Encoder, sess.dir)...)
	sess.cmd.Stderr = sess.stderr
	sess.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := sess.cmd.Start(); err != nil {
		os.RemoveAll(sess.dir)
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	go func() {
		err := sess.cmd.Wait()
		if err != nil && !strings.Contains(err.Error(), "signal: killed") {
			slog.Warn("ffmpeg stopped with an error", "session", sess.ID, "err", err, "stderr", sess.stderr.String())
		}
		close(sess.done)
	}()
	sess.lastAccess = time.Now()

	m.mu.Lock()
	m.sessions[sess.ID] = sess
	m.mu.Unlock()
	slog.Info("playback session started", "session", sess.ID, "file", req.FileID, "start", start,
		"copy_video", plan.CopyVideo, "copy_audio", plan.CopyAudio, "tonemap", plan.Tonemap, "burn", plan.BurnSubtitle, "accel", m.Encoder.Accel)
	return sess, nil
}

func (m *Manager) activeVideoTranscodes() int {
	n := 0
	for _, s := range m.sessions {
		if !s.Plan.CopyVideo {
			n++
		}
	}
	return n
}

func (m *Manager) Get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil {
		return nil, ErrNotFound
	}
	return s, nil
}

// Stop kills ffmpeg and deletes the produced segments.
func (m *Manager) Stop(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s != nil {
		s.kill()
		os.RemoveAll(s.dir)
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Stop(id)
	}
}

func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

func (s *Session) kill() {
	if s.cmd.Process == nil {
		return
	}
	// The whole process group is signalled so a paused ffmpeg is killed too.
	_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGCONT)
	_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
	<-s.done
}

// Run stops idle sessions and throttles ffmpeg so it never gets far ahead of the player.
func (m *Manager) Run(ctx context.Context) {
	t := time.NewTicker(janitorInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			m.StopAll()
			return
		case <-t.C:
		}
		m.mu.Lock()
		sessions := make([]*Session, 0, len(m.sessions))
		for _, s := range m.sessions {
			sessions = append(sessions, s)
		}
		m.mu.Unlock()
		for _, s := range sessions {
			s.mu.Lock()
			idle := time.Since(s.lastAccess) > idleTimeout
			s.mu.Unlock()
			if idle {
				slog.Info("playback session idle, stopping", "session", s.ID)
				m.Stop(s.ID)
				continue
			}
			s.throttle()
		}
	}
}

func (s *Session) throttle() {
	select {
	case <-s.done:
		return
	default:
	}
	produced := s.producedSegments()
	s.mu.Lock()
	defer s.mu.Unlock()
	ahead := produced - s.lastRequested
	switch {
	case !s.paused && ahead > pauseAhead:
		if syscall.Kill(-s.cmd.Process.Pid, syscall.SIGSTOP) == nil {
			s.paused = true
		}
	case s.paused && ahead < resumeAhead:
		if syscall.Kill(-s.cmd.Process.Pid, syscall.SIGCONT) == nil {
			s.paused = false
		}
	}
}

func (s *Session) producedSegments() int {
	entries, _ := os.ReadDir(s.dir)
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".m4s") {
			n++
		}
	}
	return n
}

// File returns the path of a playlist or segment, waiting for ffmpeg to produce it.
func (s *Session) File(ctx context.Context, name string) (string, error) {
	m := streamFileRe.FindStringSubmatch(name)
	if m == nil {
		return "", ErrNotFound
	}
	s.mu.Lock()
	s.lastAccess = time.Now()
	if m[2] != "" {
		n, _ := strconv.Atoi(m[2])
		s.lastRequested = n
		// A request past the throttling window (seek inside the produced range) resumes ffmpeg at once.
		if s.paused {
			_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGCONT)
			s.paused = false
		}
	}
	s.mu.Unlock()

	path := filepath.Join(s.dir, name)
	deadline := time.After(waitFile)
	for {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		select {
		case <-s.done:
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
			return "", fmt.Errorf("%w: ffmpeg exited: %s", ErrNotFound, s.stderr.String())
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline:
			return "", ErrNotReady
		case <-time.After(150 * time.Millisecond):
		}
	}
}

// LiveSubtitle returns the WebVTT written so far for the session's text subtitle track, if any.
func (s *Session) LiveSubtitle() (string, bool) {
	path := filepath.Join(s.dir, LiveSubtitleFile)
	_, err := os.Stat(path)
	return path, err == nil
}

// keyframeBefore returns the time of the last video keyframe at or before t.
func (m *Manager) keyframeBefore(ctx context.Context, path string, streamIndex int, t float64) float64 {
	from := max(t-keyframeWindow, 0)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, m.ffprobe, "-v", "error",
		"-select_streams", strconv.Itoa(streamIndex), "-skip_frame", "nokey",
		"-show_entries", "frame=best_effort_timestamp_time", "-of", "csv=p=0",
		"-read_intervals", fmt.Sprintf("%s%%%s", formatSeconds(from), formatSeconds(t+0.5)), path).Output()
	if err != nil {
		return t
	}
	best := -1.0
	for _, line := range strings.Split(string(out), "\n") {
		v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(line, ",")), 64)
		if err == nil && v <= t+0.001 && v > best {
			best = v
		}
	}
	if best < 0 {
		return t
	}
	return best
}

// tailBuffer keeps the end of ffmpeg's error output for diagnostics.
type tailBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
	max int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Write(p)
	if over := b.buf.Len() - b.max; over > 0 {
		b.buf.Next(over)
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(b.buf.String())
}
