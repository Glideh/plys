// Package settings stores server-wide configuration in the settings table.
package settings

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/store"
)

const (
	keyTMDB             = "tmdb_api_key"
	keyMetadataLanguage = "metadata_language"
	keyServerIdentity   = "server_identity"
	keyServerName       = "server_name"
	keyServerPublicURL  = "server_public_url"
	keyOpenSubtitles    = "opensubtitles"

	DefaultLanguage = "fr-FR"

	maxServerNameLength = 64
)

var languageRe = regexp.MustCompile(`^[a-z]{2}-[A-Z]{2}$`)

func ValidLanguage(s string) bool {
	return languageRe.MatchString(s)
}

// ValidServerName reports whether a name can be shown to peers and admins.
func ValidServerName(s string) bool {
	return s != "" && utf8.RuneCountInString(s) <= maxServerNameLength
}

// NormalizePublicURL returns the URL without trailing slash, or false when it is not a bare absolute http(s) URL.
func NormalizePublicURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil || (u.Path != "" && u.Path != "/") {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

type Store struct {
	q *store.Queries
}

func New(q *store.Queries) *Store {
	return &Store{q: q}
}

func (s *Store) TMDBKey(ctx context.Context) (string, error) {
	return s.getString(ctx, keyTMDB, "")
}

// SetTMDBKey removes the key when empty.
func (s *Store) SetTMDBKey(ctx context.Context, key string) error {
	if key == "" {
		return s.q.DeleteSetting(ctx, keyTMDB)
	}
	return s.setString(ctx, keyTMDB, key)
}

func (s *Store) MetadataLanguage(ctx context.Context) (string, error) {
	return s.getString(ctx, keyMetadataLanguage, DefaultLanguage)
}

func (s *Store) SetMetadataLanguage(ctx context.Context, lang string) error {
	if !ValidLanguage(lang) {
		return fmt.Errorf("invalid language %q", lang)
	}
	return s.setString(ctx, keyMetadataLanguage, lang)
}

// ServerName is empty until the admin sets it; pairing requires it.
func (s *Store) ServerName(ctx context.Context) (string, error) {
	return s.getString(ctx, keyServerName, "")
}

func (s *Store) SetServerName(ctx context.Context, name string) error {
	return s.setString(ctx, keyServerName, name)
}

func (s *Store) ServerPublicURL(ctx context.Context) (string, error) {
	return s.getString(ctx, keyServerPublicURL, "")
}

// SetServerPublicURL removes the URL when empty.
func (s *Store) SetServerPublicURL(ctx context.Context, raw string) error {
	if raw == "" {
		return s.q.DeleteSetting(ctx, keyServerPublicURL)
	}
	return s.setString(ctx, keyServerPublicURL, raw)
}

// OpenSubtitles holds the opensubtitles.com account used to search and download subtitles. The login is
// optional: it raises the daily download quota of the API key.
type OpenSubtitles struct {
	APIKey   string `json:"apiKey"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// OpenSubtitles returns an empty configuration until the admin sets an API key.
func (s *Store) OpenSubtitles(ctx context.Context) (OpenSubtitles, error) {
	var v OpenSubtitles
	raw, err := s.q.GetSetting(ctx, keyOpenSubtitles)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("decode setting %s: %w", keyOpenSubtitles, err)
	}
	return v, nil
}

// SetOpenSubtitles removes the configuration when the API key is empty.
func (s *Store) SetOpenSubtitles(ctx context.Context, v OpenSubtitles) error {
	if v.APIKey == "" {
		return s.q.DeleteSetting(ctx, keyOpenSubtitles)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.q.UpsertSetting(ctx, store.UpsertSettingParams{Key: keyOpenSubtitles, Value: raw})
}

func (s *Store) getString(ctx context.Context, key, fallback string) (string, error) {
	raw, err := s.q.GetSetting(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return "", err
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("decode setting %s: %w", key, err)
	}
	return v, nil
}

func (s *Store) setString(ctx context.Context, key, value string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.q.UpsertSetting(ctx, store.UpsertSettingParams{Key: key, Value: raw})
}

// Identity identifies this server to federated peers; requests between servers are signed with its key.
type Identity struct {
	ID         uuid.UUID
	PublicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
}

func (i Identity) Sign(msg []byte) []byte {
	return ed25519.Sign(i.privateKey, msg)
}

type storedIdentity struct {
	ID   uuid.UUID `json:"id"`
	Seed []byte    `json:"seed"`
}

// LoadOrCreateIdentity generates the identity on first start; concurrent starts converge on the stored one.
func (s *Store) LoadOrCreateIdentity(ctx context.Context) (Identity, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return Identity{}, err
	}
	raw, err := json.Marshal(storedIdentity{ID: uuid.New(), Seed: seed})
	if err != nil {
		return Identity{}, err
	}
	if err := s.q.InsertSettingIfAbsent(ctx, store.InsertSettingIfAbsentParams{Key: keyServerIdentity, Value: raw}); err != nil {
		return Identity{}, fmt.Errorf("store server identity: %w", err)
	}
	if raw, err = s.q.GetSetting(ctx, keyServerIdentity); err != nil {
		return Identity{}, fmt.Errorf("load server identity: %w", err)
	}
	var stored storedIdentity
	if err := json.Unmarshal(raw, &stored); err != nil || len(stored.Seed) != ed25519.SeedSize {
		return Identity{}, errors.New("corrupt server identity")
	}
	priv := ed25519.NewKeyFromSeed(stored.Seed)
	return Identity{ID: stored.ID, PublicKey: priv.Public().(ed25519.PublicKey), privateKey: priv}, nil
}
