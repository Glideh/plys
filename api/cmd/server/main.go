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

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.New(pool, auth.NewIssuer(secret, accessTokenTTL), cfg.CookieSecure, roots).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "addr", cfg.HTTPAddr)
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
