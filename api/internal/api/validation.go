package api

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"toto/api/internal/auth"
	"toto/api/internal/store"
)

var (
	usernamePattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)
	supportedLocales = []string{"fr", "en"}
)

const (
	minPasswordLen = 8
	// Bounded to keep argon2 cost predictable on hostile input.
	maxPasswordLen = 256
)

type userDTO struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Locale    string    `json:"locale"`
	CreatedAt time.Time `json:"createdAt"`
}

func toUserDTO(u store.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Role: u.Role, Locale: u.Locale, CreatedAt: u.CreatedAt}
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

// normalizeLocale returns the default locale for an empty value.
func normalizeLocale(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return supportedLocales[0], nil
	}
	if !slices.Contains(supportedLocales, s) {
		return "", errors.New("locale must be one of: fr, en")
	}
	return s, nil
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
