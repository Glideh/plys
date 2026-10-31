package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"toto/api/db/migrations"
)

// The database container may still be starting when the API does, so connecting is retried for a while.
var connectTimeout = 30 * time.Second

const connectRetryEvery = time.Second

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	deadline := time.Now().Add(connectTimeout)
	for attempt := 1; ; attempt++ {
		err = pool.Ping(ctx)
		if err == nil {
			return pool, nil
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			pool.Close()
			return nil, fmt.Errorf("ping database: %w", err)
		}
		if attempt == 1 {
			slog.Info("waiting for the database", "err", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(connectRetryEvery):
		}
	}
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(pool), migrations.FS)
	if err != nil {
		return fmt.Errorf("migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
