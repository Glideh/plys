package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
)

var (
	errInvalidRefresh = errors.New("invalid refresh token")
	errRefreshReused  = errors.New("refresh token reused")
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	sessionOptions
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if err := req.normalize(); err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	if len(req.Password) > maxPasswordLen {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}

	ctx := r.Context()
	user, err := s.q.GetUserByUsername(ctx, strings.TrimSpace(req.Username))
	if errors.Is(err, pgx.ErrNoRows) {
		auth.BurnPasswordCheck(req.Password)
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}

	sess, err := s.createSession(ctx, s.q, user, uuid.New(), req.DeviceName)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	s.writeSession(w, http.StatusOK, sess, req.TokenDelivery)
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// readRefreshToken prefers the body (mobile) and falls back to the cookie (web).
func readRefreshToken(w http.ResponseWriter, r *http.Request) (token, delivery string, err error) {
	var req refreshRequest
	if err := httpx.Decode(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		return "", "", err
	}
	if req.RefreshToken != "" {
		return req.RefreshToken, deliveryBody, nil
	}
	if c, err := r.Cookie(refreshCookieName); err == nil && c.Value != "" {
		return c.Value, deliveryCookie, nil
	}
	return "", "", errInvalidRefresh
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	token, delivery, err := readRefreshToken(w, r)
	if err != nil {
		s.rejectRefresh(w, err)
		return
	}

	ctx := r.Context()
	var sess session
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		rt, err := q.GetRefreshTokenForUpdate(ctx, auth.HashRefreshToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return errInvalidRefresh
		}
		if err != nil {
			return err
		}
		if rt.RevokedAt != nil {
			// A rotated token being presented again means it leaked: kill the whole chain.
			if err := q.RevokeRefreshFamily(ctx, rt.FamilyID); err != nil {
				return err
			}
			slog.WarnContext(ctx, "refresh token reuse detected", "user_id", rt.UserID, "family_id", rt.FamilyID)
			return nil
		}
		if time.Now().After(rt.ExpiresAt) {
			return errInvalidRefresh
		}
		if err := q.RevokeRefreshToken(ctx, rt.ID); err != nil {
			return err
		}
		user, err := q.GetUserByID(ctx, rt.UserID)
		if err != nil {
			return err
		}
		sess, err = s.createSession(ctx, q, user, rt.FamilyID, rt.DeviceName)
		return err
	})
	if err == nil && sess.accessToken == "" {
		err = errRefreshReused
	}
	if errors.Is(err, errInvalidRefresh) || errors.Is(err, errRefreshReused) {
		s.rejectRefresh(w, err)
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	s.writeSession(w, http.StatusOK, sess, delivery)
}

func (s *Server) rejectRefresh(w http.ResponseWriter, err error) {
	s.clearRefreshCookie(w)
	if errors.Is(err, errInvalidRefresh) || errors.Is(err, errRefreshReused) {
		httpx.Error(w, http.StatusUnauthorized, "invalid_refresh_token", "refresh token is invalid, expired or revoked")
		return
	}
	httpx.Error(w, http.StatusBadRequest, "invalid_body", err.Error())
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token, _, err := readRefreshToken(w, r)
	s.clearRefreshCookie(w)
	if err == nil {
		ctx := r.Context()
		rt, err := s.q.GetRefreshTokenForUpdate(ctx, auth.HashRefreshToken(token))
		if err == nil {
			err = s.q.RevokeRefreshFamily(ctx, rt.FamilyID)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			httpx.InternalError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	user, err := s.q.GetUserByID(r.Context(), p.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "user no longer exists")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toUserDTO(user))
}
