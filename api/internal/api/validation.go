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
	// Alias is the label an admin gave the user; DisplayName is the alias, or the username without one.
	Alias       *string `json:"alias"`
	DisplayName string  `json:"displayName"`
	Role        string  `json:"role"`
	Locale      string  `json:"locale"`
	// Remote marks a user of a paired server: it is listed for library access but managed there.
	Remote bool `json:"remote"`
	// OriginServer names the paired server of a remote user.
	OriginServer string `json:"originServer,omitempty"`
	// AutoplayNext starts the next episode when one ends.
	AutoplayNext bool      `json:"autoplayNext"`
	CreatedAt    time.Time `json:"createdAt"`
	// AvatarUpdatedAt is set when an avatar exists; the client uses it as a cache-busting version.
	AvatarUpdatedAt *time.Time `json:"avatarUpdatedAt,omitempty"`
}

func toUserDTO(u store.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Alias: u.Alias, DisplayName: aliasOr(u.Alias, u.Username), Role: u.Role, Locale: u.Locale, Remote: u.OriginPeerID != nil, AutoplayNext: u.AutoplayNext, CreatedAt: u.CreatedAt, AvatarUpdatedAt: u.AvatarUpdatedAt}
}

const maxAliasLen = 64

// aliasOr returns the alias when one is set, so that every client shows it before the original name.
func aliasOr(alias *string, name string) string {
	if alias != nil && *alias != "" {
		return *alias
	}
	return name
}

// peerDisplayName is the name shown to users for a paired server.
func peerDisplayName(p store.Peer) string {
	return aliasOr(p.Alias, p.Name)
}

// normalizeAlias trims the alias; an empty one clears it.
func normalizeAlias(alias *string) (*string, error) {
	if alias == nil {
		return nil, nil
	}
	a := strings.TrimSpace(*alias)
	if a == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(a) > maxAliasLen || strings.ContainsFunc(a, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return nil, errors.New("alias must be at most 64 characters, without control characters")
	}
	return &a, nil
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
