package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

const (
	invitationTTL = 7 * 24 * time.Hour
	// A wrong PIN counts against the invitation; it is revoked at this many failures (5 tries out of 10,000 codes).
	maxInvitationFailures = 5
	invitationPinDigits   = 4
	// The token travels in the URL fragment: browsers never send it to the server or in a Referer header.
	invitationPath = "/invite#"
	// Bounded so that a hostile body cannot make the server hash megabytes.
	maxInvitationTokenLen = 128
)

var (
	errInvitationInvalid = errors.New("invitation invalid")
	errInvitationPin     = errors.New("invalid pin")
)

type invitationDTO struct {
	ID        uuid.UUID `json:"id"`
	Locale    string    `json:"locale"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func toInvitationDTO(i store.Invitation) invitationDTO {
	return invitationDTO{ID: i.ID, Locale: i.Locale, CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt}
}

// createdInvitationDTO is returned once, at creation: the token and PIN are not stored and cannot be shown again.
type createdInvitationDTO struct {
	invitationDTO
	URL string `json:"url"`
	PIN string `json:"pin"`
}

// invitationPinHash keys the PIN with the token, so that a leaked hash cannot be brute-forced without the link.
func invitationPinHash(token, pin string) []byte {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte(pin))
	return m.Sum(nil)
}

func newInvitationPIN() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(math.Pow10(invitationPinDigits))))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", invitationPinDigits, n.Int64()), nil
}

// noStore keeps invitation answers out of caches and their URLs out of Referer headers.
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (s *Server) listInvitations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if _, err := s.q.DeleteExpiredInvitations(ctx); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	invitations, err := s.q.ListInvitations(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	out := make([]invitationDTO, 0, len(invitations))
	for _, i := range invitations {
		out = append(out, toInvitationDTO(i))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

type createInvitationRequest struct {
	// Locale is the language suggested to the invitee; empty uses the default one.
	Locale string `json:"locale"`
}

func (s *Server) createInvitation(w http.ResponseWriter, r *http.Request) {
	var req createInvitationRequest
	if err := httpx.Decode(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	locale, err := normalizeLocale(req.Locale)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	ctx := r.Context()
	publicURL, err := s.Settings.ServerPublicURL(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if publicURL == "" {
		httpx.Error(w, http.StatusConflict, "public_url_required", "set the server public URL before inviting users")
		return
	}
	token, tokenHash, err := auth.NewRefreshToken()
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	pin, err := newInvitationPIN()
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	p, _ := auth.PrincipalFrom(ctx)
	invitation, err := s.q.CreateInvitation(ctx, store.CreateInvitationParams{
		TokenHash: tokenHash,
		PinHash:   invitationPinHash(token, pin),
		Locale:    locale,
		CreatedBy: &p.UserID,
		ExpiresAt: time.Now().Add(invitationTTL),
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	noStore(w)
	httpx.JSON(w, http.StatusCreated, createdInvitationDTO{
		invitationDTO: toInvitationDTO(invitation),
		URL:           publicURL + invitationPath + token,
		PIN:           pin,
	})
}

func (s *Server) deleteInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "invitation")
	if !ok {
		return
	}
	n, err := s.q.DeleteInvitation(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if n == 0 {
		httpx.Error(w, http.StatusNotFound, "not_found", "invitation not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// throttleInvitation applies the per-address limit shared by the public invitation endpoints.
// It reports whether the request may proceed.
func (s *Server) throttleInvitation(w http.ResponseWriter, r *http.Request) bool {
	ip := clientIP(r)
	wait := s.invitationIPs.allow(ip)
	if wait == 0 {
		return true
	}
	slog.WarnContext(r.Context(), "invitation throttled", "client_ip", ip, "retry_after", wait,
		"request_id", middleware.GetReqID(r.Context()))
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
	httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many attempts")
	return false
}

func invitationInvalid(w http.ResponseWriter) {
	httpx.Error(w, http.StatusNotFound, "invitation_invalid", "this invitation is invalid, expired or already used")
}

type invitationLookupRequest struct {
	Token string `json:"token"`
}

// lookupInvitation tells the invitation page whether its link is still usable, before the form is filled in.
func (s *Server) lookupInvitation(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var req invitationLookupRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if !s.throttleInvitation(w, r) {
		return
	}
	if req.Token == "" || len(req.Token) > maxInvitationTokenLen {
		invitationInvalid(w)
		return
	}
	ctx := r.Context()
	invitation, err := s.q.GetInvitationByToken(ctx, auth.HashRefreshToken(req.Token))
	if errors.Is(err, pgx.ErrNoRows) {
		invitationInvalid(w)
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	serverName, err := s.Settings.ServerName(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"locale":     invitation.Locale,
		"expiresAt":  invitation.ExpiresAt,
		"serverName": serverName,
	})
}

type acceptInvitationRequest struct {
	Token    string `json:"token"`
	PIN      string `json:"pin"`
	Username string `json:"username"`
	Password string `json:"password"`
	Locale   string `json:"locale"`
	sessionOptions
}

// acceptInvitation creates the invitee's account, consumes the invitation and opens a session.
func (s *Server) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var req acceptInvitationRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if !s.throttleInvitation(w, r) {
		return
	}
	if req.Token == "" || len(req.Token) > maxInvitationTokenLen {
		invitationInvalid(w)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.PIN = strings.TrimSpace(req.PIN)
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
	// A wrong PIN must still record the failure, so the transaction commits and the outcome is kept apart.
	var outcome error
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		invitation, err := q.GetInvitationByTokenForUpdate(ctx, auth.HashRefreshToken(req.Token))
		if errors.Is(err, pgx.ErrNoRows) {
			outcome = errInvitationInvalid
			return nil
		}
		if err != nil {
			return err
		}
		if !hmac.Equal(invitation.PinHash, invitationPinHash(req.Token, req.PIN)) {
			outcome = errInvitationPin
			failures, err := q.RecordInvitationFailure(ctx, invitation.ID)
			if err != nil {
				return err
			}
			if failures >= maxInvitationFailures {
				slog.WarnContext(ctx, "invitation revoked after wrong PINs", "invitation_id", invitation.ID, "client_ip", clientIP(r))
				_, err = q.DeleteInvitation(ctx, invitation.ID)
				outcome = errInvitationInvalid
			}
			return err
		}
		user, err := q.CreateUser(ctx, store.CreateUserParams{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         auth.RoleUser,
			Locale:       locale,
		})
		if err != nil {
			return err
		}
		if _, err := q.DeleteInvitation(ctx, invitation.ID); err != nil {
			return err
		}
		sess, err = s.createSession(ctx, q, user, uuid.New(), req.DeviceName)
		return err
	})
	if err == nil {
		err = outcome
	}
	switch {
	case err == nil:
		slog.InfoContext(ctx, "invitation accepted", "username", req.Username, "client_ip", clientIP(r))
		s.writeSession(w, http.StatusCreated, sess, req.TokenDelivery)
	case errors.Is(err, errInvitationInvalid):
		invitationInvalid(w)
	case errors.Is(err, errInvitationPin):
		slog.WarnContext(ctx, "invitation PIN rejected", "client_ip", clientIP(r), "request_id", middleware.GetReqID(ctx))
		httpx.Error(w, http.StatusForbidden, "invalid_pin", "the code does not match this invitation")
	case isUniqueViolation(err):
		httpx.Error(w, http.StatusConflict, "username_taken", "username already exists")
	default:
		httpx.InternalError(w, r, err)
	}
}
