package config

import (
	"net/netip"
	"slices"
	"testing"
)

func TestParseTrustedProxies(t *testing.T) {
	local := func() ([]netip.Prefix, error) { return []netip.Prefix{netip.MustParsePrefix("172.18.0.0/16")}, nil }
	cases := map[string][]string{
		"":                                   {"172.18.0.0/16"},
		"none":                               {},
		"10.0.0.0/8, 192.168.1.10":           {"10.0.0.0/8", "192.168.1.10/32"},
		"172.18.0.5/16,fd00::/8,2001:db8::1": {"172.18.0.0/16", "fd00::/8", "2001:db8::1/128"},
	}
	for in, want := range cases {
		got, err := parseTrustedProxies(in, local)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		var s []string
		for _, p := range got {
			s = append(s, p.String())
		}
		if !slices.Equal(s, want) && !(len(s) == 0 && len(want) == 0) {
			t.Fatalf("%q: got %v, want %v", in, s, want)
		}
	}
	for _, bad := range []string{"nope", "10.0.0.0/33"} {
		if _, err := parseTrustedProxies(bad, local); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

func TestLocalNetworksIncludesLoopback(t *testing.T) {
	nets, err := localNetworks()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(nets, func(p netip.Prefix) bool { return p.Contains(netip.MustParseAddr("127.0.0.1")) }) {
		t.Fatalf("loopback missing: %v", nets)
	}
}
