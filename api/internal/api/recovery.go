package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/auth"
	"toto/api/internal/store"
)

var ErrUserNotFound = errors.New("no local account with this username")

// ResetPassword replaces the password of a local account and closes its sessions. It backs the reset-password
// command, for operators who can no longer sign in; accounts of paired servers are managed there and refused.
func ResetPassword(ctx context.Context, pool *pgxpool.Pool, username, password string) (store.User, error) {
	if err := validatePassword(password); err != nil {
		return store.User{}, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return store.User{}, err
	}
	var user store.User
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := store.New(tx)
		if err := q.LockUsers(ctx); err != nil {
			return err
		}
		current, err := q.GetUserByUsername(ctx, username)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		if err != nil {
			return err
		}
		user, err = q.UpdateUser(ctx, store.UpdateUserParams{ID: current.ID, PasswordHash: &hash})
		if err != nil {
			return err
		}
		return q.RevokeUserRefreshTokens(ctx, user.ID)
	})
	return user, err
}

// LocalAdminUsernames helps an operator who forgot the username of the account to recover.
func LocalAdminUsernames(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	users, err := store.New(pool).ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, u := range users {
		if u.Role == auth.RoleAdmin && u.OriginPeerID == nil {
			out = append(out, u.Username)
		}
	}
	return out, nil
}
