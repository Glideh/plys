package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/mediafs"
	"toto/api/internal/metadata"
	"toto/api/internal/scanner"
	"toto/api/internal/settings"
	"toto/api/internal/store"
)

type Deps struct {
	Pool         *pgxpool.Pool
	Tokens       *auth.Issuer
	CookieSecure bool
	Roots        *mediafs.Roots
	Scanner      *scanner.Scanner
	Metadata     *metadata.Service
	Images       *metadata.ImageCache
	Settings     *settings.Store
	Identity     settings.Identity
}

type Server struct {
	Deps
	q *store.Queries
}

func New(d Deps) *Server {
	return &Server{Deps: d, q: store.New(d.Pool)}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, requestLogger, middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Pool.Ping(r.Context()); err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "db_unavailable", "database unreachable")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/server", s.serverInfo)
		r.Get("/setup/status", s.setupStatus)
		r.Post("/setup/admin", s.setupAdmin)

		r.Post("/auth/login", s.login)
		r.Post("/auth/refresh", s.refresh)
		r.Post("/auth/logout", s.logout)

		// Public so that <img> tags work without a bearer token; only referenced images are served.
		r.Get("/images/tmdb/{size}/{file}", s.tmdbImage)
		r.Get("/images/local/{file}", s.localImage)

		r.Group(func(r chi.Router) {
			r.Use(s.Tokens.Middleware)
			r.Get("/me", s.me)
			r.Get("/libraries", s.listLibraries)
			r.Get("/libraries/{id}", s.getLibrary)

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Get("/users", s.listUsers)
				r.Post("/users", s.createUser)
				r.Patch("/users/{id}", s.updateUser)
				r.Delete("/users/{id}", s.deleteUser)

				r.Post("/libraries", s.createLibrary)
				r.Patch("/libraries/{id}", s.updateLibrary)
				r.Delete("/libraries/{id}", s.deleteLibrary)
				r.Post("/libraries/{id}/scan", s.scanLibrary)

				r.Get("/admin/scans", s.listScans)
				r.Get("/admin/fs/browse", s.browseFS)
				r.Get("/admin/settings", s.getSettings)
				r.Patch("/admin/settings", s.updateSettings)

				r.Get("/items/{id}/match/search", s.searchMatch)
				r.Post("/items/{id}/match", s.setMatch)
				r.Post("/items/{id}/refresh", s.refreshItem)
			})
		})
	})
	return r
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		slog.DebugContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration", time.Since(start),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}
