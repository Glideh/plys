package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

var (
	errLastAdmin  = errors.New("last admin")
	errDeleteSelf = errors.New("delete self")
)

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.q.ListUsers(r.Context())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		out = append(out, toUserDTO(u))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Locale   string `json:"locale"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Role == "" {
		req.Role = auth.RoleUser
	}
	locale, err := normalizeLocale(req.Locale)
	if err == nil {
		err = errors.Join(validateUsername(req.Username), validatePassword(req.Password), validateRole(req.Role))
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	user, err := s.q.CreateUser(r.Context(), store.CreateUserParams{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         req.Role,
		Locale:       locale,
	})
	if isUniqueViolation(err) {
		httpx.Error(w, http.StatusConflict, "username_taken", "username already exists")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toUserDTO(user))
}

type updateUserRequest struct {
	Role     *string `json:"role"`
	Locale   *string `json:"locale"`
	Password *string `json:"password"`
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "user")
	if !ok {
		return
	}
	var req updateUserRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	params := store.UpdateUserParams{ID: id, Role: req.Role}
	var errs []error
	if req.Role != nil {
		errs = append(errs, validateRole(*req.Role))
	}
	if req.Locale != nil {
		locale, err := normalizeLocale(*req.Locale)
		errs = append(errs, err)
		params.Locale = &locale
	}
	if req.Password != nil {
		errs = append(errs, validatePassword(*req.Password))
	}
	if err := errors.Join(errs...); err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	if req.Password != nil {
		hash, err := auth.HashPassword(*req.Password)
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		params.PasswordHash = &hash
	}

	ctx := r.Context()
	var user store.User
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.LockUsers(ctx); err != nil {
			return err
		}
		current, err := q.GetUserByID(ctx, id)
		if err != nil {
			return err
		}
		if current.Role == auth.RoleAdmin && req.Role != nil && *req.Role != auth.RoleAdmin {
			if err := ensureAnotherAdmin(ctx, q); err != nil {
				return err
			}
		}
		user, err = q.UpdateUser(ctx, params)
		if err != nil {
			return err
		}
		if req.Password != nil || user.Role != current.Role {
			return q.RevokeUserRefreshTokens(ctx, id)
		}
		return nil
	})
	if s.writeUserMutationError(w, r, err) {
		return
	}
	httpx.JSON(w, http.StatusOK, toUserDTO(user))
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "user")
	if !ok {
		return
	}
	ctx := r.Context()
	if p, _ := auth.PrincipalFrom(ctx); p.UserID == id {
		s.writeUserMutationError(w, r, errDeleteSelf)
		return
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.LockUsers(ctx); err != nil {
			return err
		}
		current, err := q.GetUserByID(ctx, id)
		if err != nil {
			return err
		}
		if current.Role == auth.RoleAdmin {
			if err := ensureAnotherAdmin(ctx, q); err != nil {
				return err
			}
		}
		return q.DeleteUser(ctx, id)
	})
	if s.writeUserMutationError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ensureAnotherAdmin must run inside a transaction holding LockUsers.
func ensureAnotherAdmin(ctx context.Context, q *store.Queries) error {
	n, err := q.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if n <= 1 {
		return errLastAdmin
	}
	return nil
}

// writeUserMutationError reports whether an error response was written.
func (s *Server) writeUserMutationError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, pgx.ErrNoRows):
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
	case errors.Is(err, errLastAdmin):
		httpx.Error(w, http.StatusConflict, "last_admin", "at least one admin must remain")
	case errors.Is(err, errDeleteSelf):
		httpx.Error(w, http.StatusConflict, "cannot_delete_self", "you cannot delete your own account")
	default:
		httpx.InternalError(w, r, err)
	}
	return true
}
