package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
)

type federatedRequestKey struct{}

// isFederatedRequest reports whether the request was authenticated with a peer's federated token.
func isFederatedRequest(ctx context.Context) bool {
	federated, _ := ctx.Value(federatedRequestKey{}).(bool)
	return federated
}

// federatedOrLocal accepts a local session token, or a federated token issued by a paired peer for one of its shadow users.
func (s *Server) federatedOrLocal(next http.Handler) http.Handler {
	local := s.Tokens.Middleware(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			local.ServeHTTP(w, r)
			return
		}
		if _, err := s.Tokens.Parse(token); err == nil {
			local.ServeHTTP(w, r)
			return
		}
		claims, err := s.authenticateFederatedToken(r, token)
		if err != nil {
			// Leaves the rejection to the local middleware, which answers 401.
			local.ServeHTTP(w, r)
			return
		}
		user, err := s.q.GetUserByID(r.Context(), claims.Subject)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (user.OriginPeerID == nil || *user.OriginPeerID != claims.Issuer)) {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid federated token")
			return
		}
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		ctx := auth.WithPrincipal(r.Context(), auth.Principal{UserID: claims.Subject, Role: auth.RoleUser})
		ctx = context.WithValue(ctx, federatedRequestKey{}, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
