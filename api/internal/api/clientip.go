package api

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

// resolveClientIP stores the client address in the request context for limiters and logs.
func (s *Server) resolveClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := forwardedClientIP(r, s.TrustedProxies)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip)))
	})
}

// clientIP returns the address resolved by resolveClientIP, or the socket address outside the router.
func clientIP(r *http.Request) string {
	if ip, ok := r.Context().Value(clientIPKey{}).(string); ok {
		return ip
	}
	return remoteHost(r.RemoteAddr)
}

// forwardedClientIP trusts X-Forwarded-For only when the socket peer is a trusted proxy. It walks the hops from the
// right and returns the first one that is not a trusted proxy: hops on its left were written by the client.
func forwardedClientIP(r *http.Request, trusted []netip.Prefix) string {
	remote := remoteHost(r.RemoteAddr)
	client, err := netip.ParseAddr(remote)
	if err != nil {
		return remote
	}
	client = client.Unmap()
	if !isTrusted(client, trusted) {
		return client.String()
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0 && isTrusted(client, trusted); i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		client = hop.Unmap()
	}
	return client.String()
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func remoteHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}
