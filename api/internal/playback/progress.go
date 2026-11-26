package playback

import (
	"bytes"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// FFmpeg writes a progress block every statsPeriod; the speed is averaged over speedWindow.
	statsPeriod = time.Second
	speedWindow = 6 * time.Second
	// Without a block for this long, FFmpeg is stalled (on its input, for instance) and the speed unknown.
	speedStale = 3 * statsPeriod
)

var progressArgs = []string{"-progress", "pipe:1", "-stats_period", strconv.FormatFloat(statsPeriod.Seconds(), 'f', -1, 64)}

type progressSample struct {
	at  time.Time
	out time.Duration
}

// progress parses the key=value blocks of FFmpeg's -progress output. FFmpeg's own speed is averaged since the
// start, pauses included, so the speed is measured here from the recent output positions.
type progress struct {
	mu      sync.Mutex
	now     func() time.Time
	partial []byte
	out     time.Duration
	hasOut  bool
	samples []progressSample
}

func newProgress() *progress {
	return &progress{now: time.Now}
}

func (p *progress) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	data := append(p.partial, b...)
	for {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			break
		}
		key, value, _ := strings.Cut(strings.TrimSpace(string(data[:i])), "=")
		data = data[i+1:]
		switch key {
		case "out_time_us":
			if us, err := strconv.ParseInt(value, 10, 64); err == nil && us >= 0 {
				p.out, p.hasOut = time.Duration(us)*time.Microsecond, true
			}
		case "progress":
			if p.hasOut {
				p.record()
			}
		}
	}
	// A line never ends only if the output is not FFmpeg's; do not let it grow.
	if len(data) > 4096 {
		data = data[:0]
	}
	p.partial = append(p.partial[:0], data...)
	return len(b), nil
}

func (p *progress) record() {
	now := p.now()
	p.samples = append(p.samples, progressSample{at: now, out: p.out})
	keep := 0
	for keep < len(p.samples)-2 && now.Sub(p.samples[keep].at) > speedWindow {
		keep++
	}
	p.samples = append(p.samples[:0], p.samples[keep:]...)
}

// reset forgets the samples when FFmpeg is paused or resumed, so the time spent stopped is not measured.
func (p *progress) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.samples = p.samples[:0]
}

// speed is the media time produced per second of real time, false while it is unknown.
func (p *progress) speed() (float64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.samples) < 2 {
		return 0, false
	}
	first, last := p.samples[0], p.samples[len(p.samples)-1]
	elapsed := last.at.Sub(first.at)
	if elapsed < statsPeriod/2 || p.now().Sub(last.at) > speedStale {
		return 0, false
	}
	return max(last.out-first.out, 0).Seconds() / elapsed.Seconds(), true
}

func (p *progress) sampled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.samples) > 0
}
