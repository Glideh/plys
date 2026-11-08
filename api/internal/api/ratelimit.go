package api

import (
	"sync"
	"time"
)

const (
	handshakeLimit  = 30
	handshakeWindow = time.Minute
	maxTrackedKeys  = 1024
)

// fixedWindowLimiter allows limit hits per key within each window, keeping memory bounded by sweeping expired keys.
type fixedWindowLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]*windowCount
}

type windowCount struct {
	start time.Time
	n     int
}

func newFixedWindowLimiter(limit int, window time.Duration) *fixedWindowLimiter {
	return &fixedWindowLimiter{limit: limit, window: window, hits: map[string]*windowCount{}}
}

func (l *fixedWindowLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.hits) > maxTrackedKeys {
		for k, c := range l.hits {
			if now.Sub(c.start) >= l.window {
				delete(l.hits, k)
			}
		}
	}
	c, ok := l.hits[key]
	if !ok || now.Sub(c.start) >= l.window {
		l.hits[key] = &windowCount{start: now, n: 1}
		return true
	}
	if c.n >= l.limit {
		return false
	}
	c.n++
	return true
}

const (
	loginIPLimit   = 20
	loginUserLimit = 5
	loginWindow    = time.Minute
	loginMaxBlock  = 15 * time.Minute
)

// backoffLimiter allows limit attempts per key within each window. A key over its limit is blocked for one window,
// and each new block doubles that delay up to maxBlock. Strikes are forgotten after maxBlock without a block.
type backoffLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	maxBlock time.Duration
	now      func() time.Time
	entries  map[string]*backoffEntry
}

type backoffEntry struct {
	start        time.Time
	n            int
	strikes      int
	blockedUntil time.Time
}

func newBackoffLimiter(limit int, window, maxBlock time.Duration) *backoffLimiter {
	return &backoffLimiter{limit: limit, window: window, maxBlock: maxBlock, now: time.Now, entries: map[string]*backoffEntry{}}
}

// allow records an attempt for key. It returns zero when the attempt may proceed, otherwise how long to wait.
func (l *backoffLimiter) allow(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.entries) > maxTrackedKeys {
		for k, e := range l.entries {
			if l.idle(e, now) {
				delete(l.entries, k)
			}
		}
	}
	e, ok := l.entries[key]
	if !ok {
		e = &backoffEntry{start: now}
		l.entries[key] = e
	}
	if now.Before(e.blockedUntil) {
		return e.blockedUntil.Sub(now)
	}
	if now.Sub(e.start) >= l.window {
		e.start, e.n = now, 0
		if e.strikes > 0 && now.Sub(e.blockedUntil) >= l.maxBlock {
			e.strikes = 0
		}
	}
	if e.n < l.limit {
		e.n++
		return 0
	}
	e.strikes++
	block := l.maxBlock
	if e.strikes <= 16 {
		block = min(l.window<<(e.strikes-1), l.maxBlock)
	}
	e.blockedUntil = now.Add(block)
	e.start, e.n = e.blockedUntil, 0
	return block
}

// reset forgets key, e.g. after a successful login.
func (l *backoffLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *backoffLimiter) idle(e *backoffEntry, now time.Time) bool {
	return now.Sub(e.start) >= l.window && (e.strikes == 0 || now.Sub(e.blockedUntil) >= l.maxBlock)
}
