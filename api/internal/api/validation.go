package api

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"toto/api/internal/auth"
	"toto/api/internal/store"
)

var (
	usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)
	// supportedLocales lists the interface languages; the first one is the default.
	supportedLocales = []string{"fr", "en", "pt-BR"}
)

const (
	minPasswordLen = 8
	// Bounded to keep argon2 cost predictable on hostile input.
	maxPasswordLen = 256
)

type userDTO struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	Locale   string    `json:"locale"`
	// Remote marks a user of a paired server: it is listed for library access but managed there.
	Remote bool `json:"remote"`
	// OriginServer names the paired server of a remote user.
	OriginServer string `json:"originServer,omitempty"`
	// AutoplayNext starts the next episode or part when one ends.
	AutoplayNext bool      `json:"autoplayNext"`
	CreatedAt    time.Time `json:"createdAt"`
	// AvatarUpdatedAt is set when an avatar exists; the client uses it as a cache-busting version.
	AvatarUpdatedAt *time.Time `json:"avatarUpdatedAt,omitempty"`
}

func toUserDTO(u store.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Role: u.Role, Locale: u.Locale, Remote: u.OriginPeerID != nil, AutoplayNext: u.AutoplayNext, CreatedAt: u.CreatedAt, AvatarUpdatedAt: u.AvatarUpdatedAt}
}

func validateUsername(s string) error {
	if !usernamePattern.MatchString(s) {
		return errors.New("username must be 3-32 characters: letters, digits, '.', '_' or '-'")
	}
	return nil
}

func validatePassword(s string) error {
	n := utf8.RuneCountInString(s)
	if n < minPasswordLen || len(s) > maxPasswordLen {
		return errors.New("password must be between 8 and 256 characters")
	}
	return nil
}

var errUnsupportedLocale = errors.New("locale must be one of: " + strings.Join(supportedLocales, ", "))

// canonicalLocale matches a locale case-insensitively and returns its canonical spelling (pt-br becomes pt-BR).
func canonicalLocale(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, locale := range supportedLocales {
		if strings.EqualFold(locale, s) {
			return locale, true
		}
	}
	return "", false
}

// normalizeLocale returns the default locale for an empty value.
func normalizeLocale(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return supportedLocales[0], nil
	}
	locale, ok := canonicalLocale(s)
	if !ok {
		return "", errUnsupportedLocale
	}
	return locale, nil
}

func validateRole(s string) error {
	if s != auth.RoleAdmin && s != auth.RoleUser {
		return errors.New("role must be 'admin' or 'user'")
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
