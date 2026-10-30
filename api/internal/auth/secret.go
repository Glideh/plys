package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"toto/api/internal/store"
)

const jwtSecretKey = "jwt_secret"

// LoadOrCreateSecret persists a generated secret so sessions survive restarts without manual configuration.
func LoadOrCreateSecret(ctx context.Context, q *store.Queries, override string) ([]byte, error) {
	if override != "" {
		if len(override) < 32 {
			return nil, errors.New("JWT_SECRET must be at least 32 characters")
		}
		return []byte(override), nil
	}
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	generated, err := json.Marshal(base64.StdEncoding.EncodeToString(b))
	if err != nil {
		return nil, err
	}
	if err := q.InsertSettingIfAbsent(ctx, store.InsertSettingIfAbsentParams{Key: jwtSecretKey, Value: generated}); err != nil {
		return nil, fmt.Errorf("store jwt secret: %w", err)
	}
	raw, err := q.GetSetting(ctx, jwtSecretKey)
	if err != nil {
		return nil, fmt.Errorf("load jwt secret: %w", err)
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, fmt.Errorf("decode jwt secret: %w", err)
	}
	return base64.StdEncoding.DecodeString(encoded)
}
