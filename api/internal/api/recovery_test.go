package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	"toto/api/internal/store"
)

func TestResetPassword(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	admin := setupAdmin(t, env.h)

	if _, err := ResetPassword(ctx, env.srv.Pool, "admin", "short"); err == nil {
		t.Fatal("a too short password must be refused")
	}
	if _, err := ResetPassword(ctx, env.srv.Pool, "nobody", "newpassword"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("unknown user: expected ErrUserNotFound, got %v", err)
	}

	// The username matches case-insensitively, like the login.
	if _, err := ResetPassword(ctx, env.srv.Pool, "ADMIN", "newpassword"); err != nil {
		t.Fatal(err)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "admin", "password": "password123"}); r.status != http.StatusUnauthorized {
		t.Fatalf("old password: expected 401, got %d", r.status)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "admin", "password": "newpassword"}); r.status != http.StatusOK {
		t.Fatalf("new password: %d %v", r.status, r.body)
	}
	if r := do(t, env.h, http.MethodPost, "/api/v1/auth/refresh", "", map[string]any{"refreshToken": admin.body["refreshToken"]}); r.status != http.StatusUnauthorized {
		t.Fatalf("a reset must close existing sessions, got %d", r.status)
	}
}

func TestResetPasswordRefusesRemoteUsers(t *testing.T) {
	env, peer, _ := pairedEnv(t)
	ctx := context.Background()
	if err := store.New(env.srv.Pool).UpsertRemoteUser(ctx, store.UpsertRemoteUserParams{ID: uuid.New(), Username: "remote", OriginPeerID: &peer.id}); err != nil {
		t.Fatal(err)
	}
	if _, err := ResetPassword(ctx, env.srv.Pool, "remote", "newpassword"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("remote user: expected ErrUserNotFound, got %v", err)
	}
	admins, err := LocalAdminUsernames(ctx, env.srv.Pool)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(admins, []string{"admin"}) {
		t.Fatalf("local admins: %v", admins)
	}
}
