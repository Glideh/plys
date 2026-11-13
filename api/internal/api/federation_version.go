package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"toto/api/internal/store"
)

var errPeerIncompatible = errors.New("peer has an incompatible server version")

var federationVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

func compatiblePeerVersions(local, remote string) bool {
	localParts := federationVersionPattern.FindStringSubmatch(local)
	remoteParts := federationVersionPattern.FindStringSubmatch(remote)
	return localParts != nil && remoteParts != nil && localParts[1] == remoteParts[1]
}

// checkPeerVersion checks that a peer runs a compatible version. A version checked less than
// peerVersionTTL ago is trusted without asking the peer; the monitor keeps it fresh.
func (s *Server) checkPeerVersion(ctx context.Context, peer store.Peer) (store.Peer, error) {
	if peer.Version != nil && peer.VersionCheckedAt != nil && time.Since(*peer.VersionCheckedAt) < s.peerVersionTTL() {
		if !compatiblePeerVersions(Version, *peer.Version) {
			return peer, errPeerIncompatible
		}
		return peer, nil
	}
	return s.refreshPeerVersion(ctx, peer)
}

// refreshPeerVersion asks the peer for its identity and version, and stores the version.
func (s *Server) refreshPeerVersion(ctx context.Context, peer store.Peer) (store.Peer, error) {
	var info struct {
		ID        uuid.UUID `json:"id"`
		PublicKey []byte    `json:"publicKey"`
		Version   string    `json:"version"`
	}
	if err := s.fetchPeerJSON(ctx, http.MethodGet, peer.Url+"/api/v1/server", nil, &info); err != nil {
		return peer, err
	}
	if info.ID != peer.ID || !bytes.Equal(info.PublicKey, peer.PublicKey) {
		return peer, errPeerRejected
	}
	updated, err := s.q.UpdatePeerVersion(ctx, store.UpdatePeerVersionParams{ID: peer.ID, Version: &info.Version})
	if err != nil {
		return peer, err
	}
	if !compatiblePeerVersions(Version, info.Version) {
		return updated, errPeerIncompatible
	}
	return updated, nil
}
