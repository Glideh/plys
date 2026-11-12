package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"toto/api/internal/api"
	"toto/api/internal/auth"
	"toto/api/internal/config"
	"toto/api/internal/database"
	"toto/api/internal/mediafs"
	"toto/api/internal/metadata"
	"toto/api/internal/opensubtitles"
	"toto/api/internal/playback"
	"toto/api/internal/scanner"
	"toto/api/internal/settings"
	"toto/api/internal/store"
)

const accessTokenTTL = 15 * time.Minute

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		return err
	}

	q := store.New(pool)
	secret, err := auth.LoadOrCreateSecret(ctx, q, cfg.JWTSecret)
	if err != nil {
		return err
	}

	go purgeStaleRefreshTokens(ctx, q)

	roots, rootErrs := mediafs.NewRoots(cfg.MediaRoots)
	for _, err := range rootErrs {
		slog.Warn("media root ignored", "err", err)
	}
	slog.Info("media roots", "roots", roots.List())

	st := settings.New(q)
	identity, err := st.LoadOrCreateIdentity(ctx)
	if err != nil {
		return err
	}
	meta := metadata.NewService(pool, st, cfg.CacheDir, "ffmpeg", "")
	images := metadata.NewImageCache(cfg.CacheDir, "")
	meta.SetImages(images)
	go meta.WarmImages(ctx)
	scan := scanner.New(pool, scanner.FFProbe{Binary: "ffprobe"}, meta, cfg.ScanInterval)
	go scan.Run(ctx)
	go meta.BackfillFilters(ctx)

	player, err := playback.NewManager("ffmpeg", "ffprobe", playback.DetectEncoder(ctx, "ffmpeg", cfg.HWAccel), cfg.CacheDir, cfg.MaxTranscodes)
	if err != nil {
		return err
	}
	go player.Run(ctx)
	subtitles, err := playback.NewSubtitles("ffmpeg", cfg.CacheDir)
	if err != nil {
		return err
	}
	osubs := opensubtitles.New("", "Plys v"+api.Version, func(ctx context.Context) (opensubtitles.Credentials, error) {
		v, err := st.OpenSubtitles(ctx)
		return opensubtitles.Credentials{APIKey: v.APIKey, Username: v.Username, Password: v.Password}, err
	})

	handler := api.New(api.Deps{
		Pool:                   pool,
		Tokens:                 auth.NewIssuer(secret, accessTokenTTL),
		CookieSecure:           cfg.CookieSecure,
		Roots:                  roots,
		Scanner:                scan,
		Metadata:               meta,
		Images:                 images,
		Settings:               st,
		Identity:               identity,
		Playback:               player,
		Subtitles:              subtitles,
		OpenSubtitles:          osubs,
		FederationRequireHTTPS: cfg.FederationRequireHTTPS,
		TrustedProxies:         cfg.TrustedProxies,
	}).Routes()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "addr", cfg.HTTPAddr, "trusted_proxies", cfg.TrustedProxies)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func purgeStaleRefreshTokens(ctx context.Context, q *store.Queries) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		n, err := q.DeleteStaleRefreshTokens(ctx)
		if err != nil && ctx.Err() == nil {
			slog.Error("purge refresh tokens", "err", err)
		} else if n > 0 {
			slog.Info("purged refresh tokens", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
