package api

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestForwardedClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.18.0.0/16"), netip.MustParsePrefix("127.0.0.1/32")}
	cases := []struct {
		name, remote string
		xff          []string
		want         string
	}{
		{"direct client ignores the header", "203.0.113.7:5000", []string{"198.51.100.1"}, "203.0.113.7"},
		{"trusted proxy without header", "172.18.0.3:5000", nil, "172.18.0.3"},
		{"trusted proxy", "172.18.0.3:5000", []string{"203.0.113.7"}, "203.0.113.7"},
		{"spoofed left hops are ignored", "172.18.0.3:5000", []string{"10.0.0.1, 198.51.100.1, 203.0.113.7"}, "203.0.113.7"},
		{"chain of trusted proxies", "172.18.0.3:5000", []string{"203.0.113.7, 172.18.0.1"}, "203.0.113.7"},
		{"several headers", "172.18.0.3:5000", []string{"198.51.100.1", "203.0.113.7, 172.18.0.1"}, "203.0.113.7"},
		{"garbage stops at the last trusted hop", "172.18.0.3:5000", []string{"203.0.113.7, nope"}, "172.18.0.3"},
		{"all hops trusted", "172.18.0.3:5000", []string{"127.0.0.1"}, "127.0.0.1"},
		{"ipv4-mapped socket address", "[::ffff:172.18.0.3]:5000", []string{"203.0.113.7"}, "203.0.113.7"},
		{"ipv6 client", "172.18.0.3:5000", []string{"2001:db8::1"}, "2001:db8::1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = c.remote
			for _, v := range c.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := forwardedClientIP(r, trusted); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestForwardedClientIPTrustsNothingByDefault(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got := forwardedClientIP(r, nil); got != "127.0.0.1" {
		t.Fatalf("got %q", got)
	}
}
