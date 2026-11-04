package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"toto/api/internal/httpx"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type Principal struct {
	UserID uuid.UUID
	Role   string
}

type principalKey struct{}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// WithPrincipal returns a context carrying p, for principals authenticated by another mechanism.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func (i *Issuer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || raw == "" {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		claims, err := i.Parse(raw)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid token subject")
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, Principal{UserID: userID, Role: claims.Role})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := PrincipalFrom(r.Context()); !ok || p.Role != RoleAdmin {
			httpx.Error(w, http.StatusForbidden, "forbidden", "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
