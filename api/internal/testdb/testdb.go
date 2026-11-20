// Package testdb gives each test package its own PostgreSQL schema so packages can run in parallel.
package testdb

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/database"
)

// New recreates the schema, applies migrations and returns a pool bound to it.
// It skips the test when TEST_DATABASE_URL is unset, unless TESTDB_REQUIRED is set (CI), where it fails.
func New(t *testing.T, schema string) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		if os.Getenv("TESTDB_REQUIRED") != "" {
			t.Fatal("TEST_DATABASE_URL not set while TESTDB_REQUIRED is")
		}
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	ident := pgx.Identifier{schema}.Sanitize()
	_, err = admin.Exec(ctx, "DROP SCHEMA IF EXISTS "+ident+" CASCADE; CREATE SCHEMA "+ident)
	if err == nil {
		err = createExtensions(ctx, admin)
	}
	admin.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// createExtensions installs the database-wide extensions the migrations need before they run.
// Test packages migrate in parallel, and concurrent CREATE EXTENSION IF NOT EXISTS calls can fail
// on a fresh database, so they are serialized with an advisory lock.
func createExtensions(ctx context.Context, conn *pgx.Conn) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('plys-testdb-extensions'))"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS unaccent SCHEMA public"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
