package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

var errAlreadyInitialized = errors.New("already initialized")

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	n, err := s.q.CountUsers(r.Context())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"needsSetup": n == 0})
}

type setupAdminRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Locale   string `json:"locale"`
	sessionOptions
}

func (s *Server) setupAdmin(w http.ResponseWriter, r *http.Request) {
	var req setupAdminRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	locale, err := normalizeLocale(req.Locale)
	if err == nil {
		err = errors.Join(validateUsername(req.Username), validatePassword(req.Password), req.normalize())
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

	ctx := r.Context()
	var sess session
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.LockUsers(ctx); err != nil {
			return err
		}
		n, err := q.CountUsers(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			return errAlreadyInitialized
		}
		user, err := q.CreateUser(ctx, store.CreateUserParams{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         auth.RoleAdmin,
			Locale:       locale,
		})
		if err != nil {
			return err
		}
		sess, err = s.createSession(ctx, q, user, uuid.New(), req.DeviceName)
		return err
	})
	if errors.Is(err, errAlreadyInitialized) {
		httpx.Error(w, http.StatusConflict, "already_initialized", "setup has already been completed")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	s.writeSession(w, http.StatusCreated, sess, req.TokenDelivery)
}
