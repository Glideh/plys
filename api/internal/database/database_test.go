package database

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestConnectGivesUpWhenTheDatabaseStaysDown(t *testing.T) {
	old := connectTimeout
	connectTimeout = 1500 * time.Millisecond
	t.Cleanup(func() { connectTimeout = old })

	start := time.Now()
	_, err := Connect(context.Background(), "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	if err == nil || !strings.Contains(err.Error(), "ping database") {
		t.Fatalf("expected a ping error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Errorf("must retry before giving up, returned after %v", elapsed)
	}
}

func TestConnectSucceedsWhenTheDatabaseIsUp(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := Connect(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
}
