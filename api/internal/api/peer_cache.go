package api

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"toto/api/internal/store"
)

// Default lifetimes of the cached peer answers. Catalog listings are the expensive ones; the home rails show
// playback progress, so they are kept the shortest.
const (
	peerCatalogCacheTTL = 90 * time.Second
	peerFilterCacheTTL  = 30 * time.Second
	peerHomeCacheTTL    = 15 * time.Second
)

// Bounds on the number of cached answers. A catalog answer can hold every card of a large peer, so few are kept.
const (
	peerCatalogCacheSize = 64
	peerFilterCacheSize  = 512
	peerHomeCacheSize    = 256
)

// peerCache keeps the recent answers of the peers for a short time, and shares one call to a peer between the
// concurrent requests that need the same answer.
type peerCache[T any] struct {
	ttl     time.Duration
	max     int
	mu      sync.Mutex
	entries map[string]peerCacheEntry[T]
	flights flightGroup[T]
}

type peerCacheEntry[T any] struct {
	value   T
	expires time.Time
}

// newPeerCache builds a cache; a negative override disables caching (calls are still shared), a positive one replaces ttl.
func newPeerCache[T any](ttl, override time.Duration, max int) *peerCache[T] {
	if override != 0 {
		ttl = override
	}
	return &peerCache[T]{ttl: ttl, max: max, entries: make(map[string]peerCacheEntry[T])}
}

func (c *peerCache[T]) get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || !time.Now().Before(entry.expires) {
		var zero T
		return zero, false
	}
	return entry.value, true
}

func (c *peerCache[T]) set(key string, value T) {
	if c.ttl <= 0 {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[key]; !ok && len(c.entries) >= c.max {
		// Drop the expired answers, then the ones closest to expiry, until there is room.
		for k, entry := range c.entries {
			if !now.Before(entry.expires) {
				delete(c.entries, k)
			}
		}
		for len(c.entries) >= c.max {
			oldest := ""
			for k, entry := range c.entries {
				if oldest == "" || entry.expires.Before(c.entries[oldest].expires) {
					oldest = k
				}
			}
			delete(c.entries, oldest)
		}
	}
	c.entries[key] = peerCacheEntry[T]{value: value, expires: now.Add(c.ttl)}
}

// forgetPeer drops the cached answers of one peer, for example when the name it is shown under changes.
func (c *peerCache[T]) forgetPeer(id string) {
	prefix := id + "\x00"
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

// cachedPeerCall answers from the cache while it is fresh. Otherwise it calls the peer through callPeer, with the
// same health and version rules, sharing the call between concurrent callers. Only successful answers are cached;
// they must be treated as read-only, since several requests receive the same value.
func cachedPeerCall[T any](ctx context.Context, s *Server, c *peerCache[T], p store.Peer, key string, fetch func(context.Context) (T, error)) (T, error) {
	var zero T
	if peerSkipped(p) {
		return zero, fmt.Errorf("%w: %w", errPeerUnreachable, errPeerSkipped)
	}
	key = p.ID.String() + "\x00" + key
	if value, ok := c.get(key); ok {
		// A cached answer is only served while the peer is still known to be compatible.
		if _, err := s.checkPeerVersion(ctx, p); err == nil {
			return value, nil
		}
	}
	return c.flights.do(ctx, key, func(ctx context.Context) (T, error) {
		var value T
		err := s.callPeer(ctx, p, func(ctx context.Context) error {
			var err error
			value, err = fetch(ctx)
			return err
		})
		if err != nil {
			return zero, err
		}
		c.set(key, value)
		return value, nil
	})
}

// flightGroup runs one call per key at a time and hands its result to every caller waiting for it. The call is
// cancelled only once all of its callers have left, so that one abandoned request does not fail the others.
type flightGroup[T any] struct {
	mu      sync.Mutex
	flights map[string]*flight[T]
}

type flight[T any] struct {
	done    chan struct{}
	value   T
	err     error
	waiters int
	cancel  context.CancelFunc
}

func (g *flightGroup[T]) do(ctx context.Context, key string, call func(context.Context) (T, error)) (T, error) {
	g.mu.Lock()
	if g.flights == nil {
		g.flights = make(map[string]*flight[T])
	}
	f, ok := g.flights[key]
	if !ok {
		callCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		f = &flight[T]{done: make(chan struct{}), cancel: cancel}
		g.flights[key] = f
		go func() {
			f.value, f.err = call(callCtx)
			cancel()
			g.mu.Lock()
			if g.flights[key] == f {
				delete(g.flights, key)
			}
			g.mu.Unlock()
			close(f.done)
		}()
	}
	f.waiters++
	g.mu.Unlock()

	select {
	case <-f.done:
		return f.value, f.err
	case <-ctx.Done():
		g.mu.Lock()
		f.waiters--
		if f.waiters == 0 {
			// Nobody waits any more: stop the call, and let the next caller start a fresh one.
			f.cancel()
			if g.flights[key] == f {
				delete(g.flights, key)
			}
		}
		g.mu.Unlock()
		var zero T
		return zero, ctx.Err()
	}
}
