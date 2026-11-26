package playback

import (
	"fmt"
	"testing"
	"time"
)

func TestProgressSpeed(t *testing.T) {
	now := time.Unix(0, 0)
	p := newProgress()
	p.now = func() time.Time { return now }
	block := func(out time.Duration) {
		// Split across writes, as a pipe may deliver it.
		s := fmt.Sprintf("frame=1\nout_time_us=%d\nspeed=9x\nprogress=continue\n", out.Microseconds())
		p.Write([]byte(s[:7]))
		p.Write([]byte(s[7:]))
	}

	if _, ok := p.speed(); ok {
		t.Fatal("speed must be unknown without samples")
	}
	block(0)
	if _, ok := p.speed(); ok {
		t.Fatal("speed must be unknown with one sample")
	}
	for i := 1; i <= 10; i++ {
		now = now.Add(time.Second)
		block(time.Duration(i) * 2 * time.Second)
	}
	if got, ok := p.speed(); !ok || got != 2 {
		t.Fatalf("speed = %v, %v; want 2", got, ok)
	}
	if len(p.samples) > int(speedWindow/statsPeriod)+2 {
		t.Errorf("samples outside the window are kept: %d", len(p.samples))
	}

	// After a pause, only the time FFmpeg runs again counts.
	p.reset()
	now = now.Add(time.Minute)
	block(21 * time.Second)
	now = now.Add(2 * time.Second)
	block(22 * time.Second)
	if got, ok := p.speed(); !ok || got != 0.5 {
		t.Fatalf("speed after resume = %v, %v; want 0.5", got, ok)
	}

	now = now.Add(speedStale + time.Second)
	if _, ok := p.speed(); ok {
		t.Error("speed must be unknown when FFmpeg stalls")
	}

	p.Write([]byte("out_time_us=N/A\nprogress=continue\n"))
	if len(p.samples) != 3 {
		t.Errorf("an invalid position must keep the last one: %d samples", len(p.samples))
	}
}
