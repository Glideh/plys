package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

const (
	refreshTTL        = 30 * 24 * time.Hour
	refreshCookieName = "refresh_token"
	refreshCookiePath = "/api/v1/auth"
	maxDeviceNameLen  = 100

	// Web clients keep the refresh token in an HttpOnly cookie; mobile clients receive it in the body.
	deliveryCookie = "cookie"
	deliveryBody   = "body"
)

type sessionOptions struct {
	DeviceName    string `json:"deviceName"`
	TokenDelivery string `json:"tokenDelivery"`
}

func (o *sessionOptions) normalize() error {
	o.DeviceName = strings.TrimSpace(o.DeviceName)
	if len(o.DeviceName) > maxDeviceNameLen {
		return errors.New("deviceName is too long")
	}
	switch o.TokenDelivery {
	case "":
		o.TokenDelivery = deliveryBody
	case deliveryBody, deliveryCookie:
	default:
		return errors.New("tokenDelivery must be 'cookie' or 'body'")
	}
	return nil
}

type session struct {
	user         store.User
	accessToken  string
	expiresAt    time.Time
	refreshToken string
}

type sessionResponse struct {
	AccessToken  string    `json:"accessToken"`
	TokenType    string    `json:"tokenType"`
	ExpiresAt    time.Time `json:"expiresAt"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	User         userDTO   `json:"user"`
}

func (s *Server) createSession(ctx context.Context, q *store.Queries, user store.User, familyID uuid.UUID, deviceName string) (session, error) {
	refresh, hash, err := auth.NewRefreshToken()
	if err != nil {
		return session{}, err
	}
	err = q.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID:     user.ID,
		FamilyID:   familyID,
		TokenHash:  hash,
		DeviceName: deviceName,
		ExpiresAt:  time.Now().Add(refreshTTL),
	})
	if err != nil {
		return session{}, err
	}
	access, exp, err := s.tokens.Issue(user.ID, user.Role)
	if err != nil {
		return session{}, err
	}
	return session{user: user, accessToken: access, expiresAt: exp, refreshToken: refresh}, nil
}

func (s *Server) writeSession(w http.ResponseWriter, status int, sess session, delivery string) {
	resp := sessionResponse{
		AccessToken: sess.accessToken,
		TokenType:   "Bearer",
		ExpiresAt:   sess.expiresAt,
		User:        toUserDTO(sess.user),
	}
	if delivery == deliveryCookie {
		http.SetCookie(w, s.refreshCookie(sess.refreshToken, int(refreshTTL.Seconds())))
	} else {
		resp.RefreshToken = sess.refreshToken
	}
	httpx.JSON(w, status, resp)
}

func (s *Server) refreshCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     refreshCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	}
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, s.refreshCookie("", -1))
}
