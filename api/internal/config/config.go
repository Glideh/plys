package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	JWTSecret    string
	CookieSecure bool
	// FederationRequireHTTPS refuses peer URLs that are not https. Development may disable it.
	FederationRequireHTTPS bool
	LogLevel               slog.Level
	MediaRoots             []string
	ScanInterval           time.Duration
	CacheDir               string
	// HWAccel is auto, qsv, vaapi or none.
	HWAccel       string
	MaxTranscodes int
	// TrustedProxies may set X-Forwarded-For. Defaults to the networks of this host (the compose network).
	TrustedProxies []netip.Prefix
}

func Load() (Config, error) {
	c := Config{
		HTTPAddr:    env("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		MediaRoots:  filepath.SplitList(env("MEDIA_ROOTS", "/media")),
		CacheDir:    env("CACHE_DIR", "/cache"),
		HWAccel:     env("HW_ACCEL", "auto"),
	}
	switch c.HWAccel {
	case "auto", "qsv", "vaapi", "none":
	default:
		return c, fmt.Errorf("HW_ACCEL must be auto, qsv, vaapi or none, got %q", c.HWAccel)
	}
	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	secure, err := strconv.ParseBool(env("COOKIE_SECURE", "true"))
	if err != nil {
		return c, fmt.Errorf("COOKIE_SECURE: %w", err)
	}
	c.CookieSecure = secure
	federationHTTPS, err := strconv.ParseBool(env("PLYS_FEDERATION_REQUIRE_HTTPS", "true"))
	if err != nil {
		return c, fmt.Errorf("PLYS_FEDERATION_REQUIRE_HTTPS: %w", err)
	}
	c.FederationRequireHTTPS = federationHTTPS
	if c.MaxTranscodes, err = strconv.Atoi(env("MAX_TRANSCODES", "2")); err != nil || c.MaxTranscodes < 1 {
		return c, errors.New("MAX_TRANSCODES must be a positive integer")
	}
	if c.ScanInterval, err = time.ParseDuration(env("SCAN_INTERVAL", "6h")); err != nil {
		return c, fmt.Errorf("SCAN_INTERVAL: %w", err)
	}
	if err := c.LogLevel.UnmarshalText([]byte(env("LOG_LEVEL", "info"))); err != nil {
		return c, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	if c.TrustedProxies, err = parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"), localNetworks); err != nil {
		return c, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	return c, nil
}

// parseTrustedProxies reads a comma-separated list of CIDRs or addresses; "none" trusts no proxy and an empty value
// falls back to the local networks.
func parseTrustedProxies(v string, local func() ([]netip.Prefix, error)) ([]netip.Prefix, error) {
	v = strings.TrimSpace(v)
	switch v {
	case "":
		return local()
	case "none":
		return []netip.Prefix{}, nil
	}
	var out []netip.Prefix
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			addr, err := netip.ParseAddr(part)
			if err != nil {
				return nil, err
			}
			out = append(out, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, err
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// localNetworks lists the loopback ranges and the subnets of the network interfaces, i.e. the compose network in a
// container, where the web front-end forwarding /api lives.
func localNetworks() ([]netip.Prefix, error) {
	out := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() {
			continue
		}
		addr, ok := netip.AddrFromSlice(ipnet.IP)
		if !ok {
			continue
		}
		bits, _ := ipnet.Mask.Size()
		addr = addr.Unmap()
		if addr.Is4() && bits > 32 {
			bits -= 96
		}
		out = append(out, netip.PrefixFrom(addr, bits).Masked())
	}
	return out, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
