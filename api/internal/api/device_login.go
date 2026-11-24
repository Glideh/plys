package api

import (
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

// A device login lets a TV sign in without typing a password: the TV shows a short code and a QR code, a signed-in
// phone approves it, and the TV, polling with a secret device code only it knows, then receives its own session.
const (
	deviceLoginTTL = 10 * time.Minute
	// Seconds the TV waits between two polls.
	deviceLoginInterval = 5
	// Consonants only, without look-alikes: the code cannot spell words and is easy to read on a TV across the room.
	userCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"
	userCodeLength   = 8
	// The phone page, reached by the QR code; the user code is not a secret, it is shown on the TV.
	deviceLoginPath = "/link?code="
	// Device logins started per client address and minute, and codes a user may check per minute.
	deviceLoginStartLimit = 10
	deviceLoginCheckLimit = 10
	maxDeviceCodeLen      = 128
	userCodeAttempts      = 5
)

var errDeviceLoginPending = errors.New("device login pending")

type deviceLoginStartRequest struct {
	DeviceName string `json:"deviceName"`
}

type deviceLoginStartResponse struct {
	DeviceCode string `json:"deviceCode"`
	UserCode   string `json:"userCode"`
	// VerificationURL opens the approval page on a phone; empty when the server has no public URL, in which case the
	// TV builds it from the address it signed in with.
	VerificationURL string    `json:"verificationUrl"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Interval        int       `json:"interval"`
}

func newUserCode() (string, error) {
	var b strings.Builder
	for i := range userCodeLength {
		if i == userCodeLength/2 {
			b.WriteByte('-')
		}
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(userCodeAlphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(userCodeAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// normalizeUserCode accepts a code typed in any case, with or without its dash or spaces.
func normalizeUserCode(s string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if r == '-' || r == ' ' {
			continue
		}
		if !strings.ContainsRune(userCodeAlphabet, r) || b.Len() == userCodeLength {
			return "", false
		}
		b.WriteRune(r)
	}
	if b.Len() != userCodeLength {
		return "", false
	}
	code := b.String()
	return code[:userCodeLength/2] + "-" + code[userCodeLength/2:], true
}

func deviceLoginInvalid(w http.ResponseWriter) {
	httpx.Error(w, http.StatusNotFound, "device_login_invalid", "this sign-in request is invalid, expired or already used")
}

// startDeviceLogin is called by the TV, without any account: it receives the codes to show and the one to poll with.
func (s *Server) startDeviceLogin(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var req deviceLoginStartRequest
	if err := httpx.Decode(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	opts := sessionOptions{DeviceName: req.DeviceName}
	if err := opts.normalize(); err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	ip := clientIP(r)
	if !s.deviceLoginIPs.allow(ip) {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many sign-in requests")
		return
	}
	ctx := r.Context()
	if _, err := s.q.DeleteExpiredDeviceLogins(ctx); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	publicURL, err := s.Settings.ServerPublicURL(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	deviceCode, deviceCodeHash, err := auth.NewRefreshToken()
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	var login store.DeviceLogin
	// A user code is short: on the rare collision with a pending one, draw another.
	for attempt := 0; ; attempt++ {
		userCode, err := newUserCode()
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		login, err = s.q.CreateDeviceLogin(ctx, store.CreateDeviceLoginParams{
			DeviceCodeHash: deviceCodeHash,
			UserCode:       userCode,
			DeviceName:     opts.DeviceName,
			ExpiresAt:      time.Now().Add(deviceLoginTTL),
		})
		if err == nil {
			break
		}
		if !isUniqueViolation(err) || attempt+1 >= userCodeAttempts {
			httpx.InternalError(w, r, err)
			return
		}
	}
	resp := deviceLoginStartResponse{
		DeviceCode: deviceCode,
		UserCode:   login.UserCode,
		ExpiresAt:  login.ExpiresAt,
		Interval:   deviceLoginInterval,
	}
	if publicURL != "" {
		resp.VerificationURL = publicURL + deviceLoginPath + login.UserCode
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

type deviceLoginTokenRequest struct {
	DeviceCode string `json:"deviceCode"`
}

// deviceLoginToken is polled by the TV: 409 while the phone has not approved, then the session, exactly once.
func (s *Server) deviceLoginToken(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var req deviceLoginTokenRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	if req.DeviceCode == "" || len(req.DeviceCode) > maxDeviceCodeLen {
		deviceLoginInvalid(w)
		return
	}
	ctx := r.Context()
	var sess session
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		login, err := q.GetDeviceLoginByDeviceCodeForUpdate(ctx, auth.HashRefreshToken(req.DeviceCode))
		if err != nil {
			return err
		}
		if login.UserID == nil {
			return errDeviceLoginPending
		}
		user, err := q.GetUserByID(ctx, *login.UserID)
		if err != nil {
			return err
		}
		if err := q.DeleteDeviceLogin(ctx, login.ID); err != nil {
			return err
		}
		sess, err = s.createSession(ctx, q, user, uuid.New(), login.DeviceName)
		return err
	})
	switch {
	case err == nil:
		slog.InfoContext(ctx, "device signed in from a phone", "user_id", sess.user.ID, "client_ip", clientIP(r))
		s.writeSession(w, http.StatusOK, sess, deliveryBody)
	case errors.Is(err, errDeviceLoginPending):
		httpx.Error(w, http.StatusConflict, "authorization_pending", "the sign-in has not been approved yet")
	case errors.Is(err, pgx.ErrNoRows):
		deviceLoginInvalid(w)
	default:
		httpx.InternalError(w, r, err)
	}
}

type deviceLoginCodeRequest struct {
	UserCode string `json:"userCode"`
}

type deviceLoginInfo struct {
	UserCode   string    `json:"userCode"`
	DeviceName string    `json:"deviceName"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

// readUserCode decodes the code a signed-in user typed or scanned, within the per-user limit.
// It reports false once it has answered.
func (s *Server) readUserCode(w http.ResponseWriter, r *http.Request) (string, bool) {
	noStore(w)
	var req deviceLoginCodeRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return "", false
	}
	p, _ := auth.PrincipalFrom(r.Context())
	if wait := s.deviceLoginUsers.allow(p.UserID.String()); wait > 0 {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many attempts")
		return "", false
	}
	code, ok := normalizeUserCode(req.UserCode)
	if !ok {
		deviceLoginInvalid(w)
		return "", false
	}
	return code, true
}

// lookupDeviceLogin shows the phone which device asks to sign in, before the user approves it.
func (s *Server) lookupDeviceLogin(w http.ResponseWriter, r *http.Request) {
	code, ok := s.readUserCode(w, r)
	if !ok {
		return
	}
	login, err := s.q.GetPendingDeviceLogin(r.Context(), code)
	if errors.Is(err, pgx.ErrNoRows) {
		deviceLoginInvalid(w)
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, deviceLoginInfo{UserCode: login.UserCode, DeviceName: login.DeviceName, ExpiresAt: login.ExpiresAt})
}

// approveDeviceLogin signs the waiting device in as the caller.
func (s *Server) approveDeviceLogin(w http.ResponseWriter, r *http.Request) {
	code, ok := s.readUserCode(w, r)
	if !ok {
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())
	n, err := s.q.ApproveDeviceLogin(r.Context(), store.ApproveDeviceLoginParams{UserID: &p.UserID, UserCode: code})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if n == 0 {
		deviceLoginInvalid(w)
		return
	}
	s.deviceLoginUsers.reset(p.UserID.String())
	slog.InfoContext(r.Context(), "device sign-in approved", "user_id", p.UserID, "client_ip", clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}

// denyDeviceLogin cancels the request: the TV is told it is no longer valid.
func (s *Server) denyDeviceLogin(w http.ResponseWriter, r *http.Request) {
	code, ok := s.readUserCode(w, r)
	if !ok {
		return
	}
	n, err := s.q.DenyDeviceLogin(r.Context(), code)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if n == 0 {
		deviceLoginInvalid(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
