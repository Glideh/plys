package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr     string
	DatabaseURL  string
	JWTSecret    string
	CookieSecure bool
	LogLevel     slog.Level
	MediaRoots   []string
	ScanInterval time.Duration
	CacheDir     string
	// HWAccel is auto, qsv, vaapi or none.
	HWAccel       string
	MaxTranscodes int
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
	if c.MaxTranscodes, err = strconv.Atoi(env("MAX_TRANSCODES", "2")); err != nil || c.MaxTranscodes < 1 {
		return c, errors.New("MAX_TRANSCODES must be a positive integer")
	}
	if c.ScanInterval, err = time.ParseDuration(env("SCAN_INTERVAL", "6h")); err != nil {
		return c, fmt.Errorf("SCAN_INTERVAL: %w", err)
	}
	if err := c.LogLevel.UnmarshalText([]byte(env("LOG_LEVEL", "info"))); err != nil {
		return c, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	return c, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
