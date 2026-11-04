package api

import (
	"net"
	"net/http"
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

// clientIP uses the socket address only; forwarded headers are client-controlled.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
