package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"toto/api/internal/store"
)

const (
	// peerOfflineThreshold is the number of consecutive failures after which a peer is considered offline.
	peerOfflineThreshold = 2
	// peerFailureSpacing groups the failures of concurrent requests: they are the same outage, not consecutive ones.
	peerFailureSpacing = 2 * time.Second
	// peerSeenRefresh throttles the last_seen_at writes of a healthy peer.
	peerSeenRefresh = time.Minute
	// defaultPeerVersionTTL is how long a checked peer version is trusted before asking the peer again.
	defaultPeerVersionTTL = 5 * time.Minute
	// peerMonitorTick is how often the monitor looks for peers due for a probe.
	peerMonitorTick = 10 * time.Second
)

var (
	// errPeerDown marks a call that got no usable answer: no connection, a timeout or a 5xx status.
	errPeerDown = errors.New("peer did not answer")
	// errPeerSkipped marks a peer that was not called because it is paused or offline.
	errPeerSkipped = errors.New("peer is paused or offline")
	// errPeerUnexpected marks a 400 or 404 answer, which an upgraded peer may give before its new version is known.
	errPeerUnexpected = errors.New("peer gave an unexpected answer")
)

// peerSkipped reports whether outgoing calls leave the peer out: an admin paused it, or it stopped answering.
func peerSkipped(p store.Peer) bool {
	return p.Paused || p.OfflineSince != nil
}

// peerBackoff is the delay before probing a failing peer again: 30 s, then 1 min, then every 5 min.
func peerBackoff(failures int32) time.Duration {
	switch {
	case failures <= peerOfflineThreshold:
		return 30 * time.Second
	case failures == peerOfflineThreshold+1:
		return time.Minute
	default:
		return 5 * time.Minute
	}
}

func (s *Server) peerVersionTTL() time.Duration {
	if s.PeerVersionTTL == 0 {
		return defaultPeerVersionTTL
	}
	return s.PeerVersionTTL
}

// callPeer runs one outgoing exchange with a peer under the aggregation budget, and records whether the peer answered.
// Paused and offline peers are not called at all.
func (s *Server) callPeer(ctx context.Context, p store.Peer, call func(context.Context) error) error {
	if peerSkipped(p) {
		return fmt.Errorf("%w: %w", errPeerUnreachable, errPeerSkipped)
	}
	callCtx, cancel := context.WithTimeout(ctx, peerCatalogTimeout)
	defer cancel()
	err := func() error {
		if _, err := s.checkPeerVersion(callCtx, p); err != nil {
			return err
		}
		return call(callCtx)
	}()
	if err != nil && ctx.Err() == nil && errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		// The aggregation budget ran out, not the peer: a slow but answering peer must not be marked as failing.
		// A peer that does not answer at all still fails each request on the client timeout, shorter than the budget.
		return fmt.Errorf("%w: aggregation budget exceeded: %v", errPeerUnreachable, err)
	}
	s.recordPeerResult(ctx, p, err)
	if errors.Is(err, errPeerUnexpected) && ctx.Err() == nil {
		// The peer may have changed version since the cached check: read it again for the next requests.
		go func() {
			if _, err := s.refreshPeerVersion(context.WithoutCancel(ctx), p); err != nil {
				slog.Debug("peer version refresh failed", "peer", p.ID, "err", err)
			}
		}()
	}
	return err
}

// recordPeerResult updates the health of a peer after a call. Any answer, even a refusal, proves the peer is up.
func (s *Server) recordPeerResult(ctx context.Context, p store.Peer, err error) {
	if ctx.Err() != nil {
		// The caller left: the outcome says nothing about the peer.
		return
	}
	if errors.Is(err, errPeerDown) {
		s.markPeerFailed(ctx, p)
		return
	}
	if err == nil || errors.Is(err, errPeerUnreachable) || errors.Is(err, errPeerIncompatible) || errors.Is(err, errPeerRejected) {
		s.markPeerSeen(ctx, p)
	}
}

// markPeerSeen records that the peer answered. A healthy peer is written at most once per peerSeenRefresh.
func (s *Server) markPeerSeen(ctx context.Context, p store.Peer) {
	if p.ConsecutiveFailures == 0 && p.OfflineSince == nil && p.LastSeenAt != nil && time.Since(*p.LastSeenAt) < peerSeenRefresh {
		return
	}
	if err := s.q.RecordPeerSuccess(context.WithoutCancel(ctx), p.ID); err != nil {
		slog.Error("record peer success", "peer", p.ID, "err", err)
		return
	}
	if p.OfflineSince != nil {
		slog.Info("peer back online", "peer", p.ID, "name", p.Name)
	}
}

func (s *Server) markPeerFailed(ctx context.Context, p store.Peer) {
	now := time.Now()
	cutoff := now.Add(-peerFailureSpacing)
	updated, err := s.q.RecordPeerFailure(context.WithoutCancel(ctx), store.RecordPeerFailureParams{
		ID: p.ID, Now: &now, Threshold: peerOfflineThreshold, SpacingCutoff: &cutoff,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		slog.Error("record peer failure", "peer", p.ID, "err", err)
		return
	}
	if p.OfflineSince == nil && updated.OfflineSince != nil {
		slog.Warn("peer offline, skipped until it answers again", "peer", p.ID, "name", p.Name)
	}
}

// fanOutPeers calls every peer that is not paused in parallel. It returns the results of the peers that answered
// and the names of the others, offline ones included, so that clients can say which servers are missing.
func fanOutPeers[T any](ctx context.Context, s *Server, what string, call func(context.Context, store.Peer) (T, error)) ([]T, []string, error) {
	peers, err := s.q.ListPeers(ctx)
	if err != nil {
		return nil, nil, err
	}
	active := make([]store.Peer, 0, len(peers))
	for _, p := range peers {
		if !p.Paused {
			active = append(active, p)
		}
	}
	results := make([]T, len(active))
	errs := make([]error, len(active))
	var wg sync.WaitGroup
	for i, p := range active {
		wg.Go(func() { results[i], errs[i] = call(ctx, p) })
	}
	wg.Wait()
	out := make([]T, 0, len(active))
	unavailable := []string{}
	for i, p := range active {
		if errs[i] != nil {
			unavailable = append(unavailable, peerDisplayName(p))
			if !errors.Is(errs[i], errPeerSkipped) {
				slog.Warn("federated "+what+" unavailable", "peer", p.ID, "err", errs[i])
			}
			continue
		}
		out = append(out, results[i])
	}
	return out, unavailable, nil
}

// RunPeerMonitor probes the peers until ctx ends: offline ones with a backoff, so that they come back
// without a user paying for a timeout, and healthy ones to keep their version fresh.
func (s *Server) RunPeerMonitor(ctx context.Context) {
	ticker := time.NewTicker(peerMonitorTick)
	defer ticker.Stop()
	for {
		s.probeDuePeers(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) probeDuePeers(ctx context.Context) {
	peers, err := s.q.ListPeers(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("list peers for probing", "err", err)
		}
		return
	}
	now := time.Now()
	var wg sync.WaitGroup
	for _, p := range peers {
		if peerProbeDue(p, now, s.peerVersionTTL()) {
			wg.Go(func() {
				_, err := s.refreshPeerVersion(ctx, p)
				s.recordPeerResult(ctx, p, err)
			})
		}
	}
	wg.Wait()
}

// peerProbeDue tells whether the monitor should ask a peer for its version now.
func peerProbeDue(p store.Peer, now time.Time, versionTTL time.Duration) bool {
	if p.Paused {
		return false
	}
	if p.ConsecutiveFailures > 0 && p.LastFailureAt != nil {
		return !now.Before(p.LastFailureAt.Add(peerBackoff(p.ConsecutiveFailures)))
	}
	return p.VersionCheckedAt == nil || now.Sub(*p.VersionCheckedAt) >= versionTTL
}
