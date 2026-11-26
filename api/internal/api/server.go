package api

import (
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/mediafs"
	"toto/api/internal/metadata"
	"toto/api/internal/opensubtitles"
	"toto/api/internal/playback"
	"toto/api/internal/release"
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
	Playback     *playback.Manager
	Subtitles    *playback.Subtitles
	// OpenSubtitles searches subtitles online; nil disables the search.
	OpenSubtitles *opensubtitles.Client
	// FederationRequireHTTPS refuses peers served over plain HTTP.
	FederationRequireHTTPS bool
	// HTTPClient reaches peers; nil uses a shared client with a timeout and a pool of connections per peer.
	HTTPClient *http.Client
	// PeerUserSyncInterval is the minimum delay between two automatic user syncs of a peer; zero uses one minute.
	PeerUserSyncInterval time.Duration
	// PeerVersionTTL is how long a checked peer version is trusted; zero uses five minutes, a negative value always asks the peer.
	PeerVersionTTL time.Duration
	// PeerCacheTTL overrides how long peer catalog, filter and home answers are reused; zero keeps the defaults,
	// a negative value disables the cache.
	PeerCacheTTL time.Duration
	// TrustedProxies may set X-Forwarded-For; the client address is the right-most hop outside them.
	TrustedProxies []netip.Prefix
	// Releases reports a newer published release of Plys; nil disables the update indicator.
	Releases *release.Checker
}

type Server struct {
	Deps
	q          *store.Queries
	handshakes *fixedWindowLimiter
	loginIPs   *backoffLimiter
	loginUsers *backoffLimiter
	// Public invitation endpoints, counted per client address.
	invitationIPs *backoffLimiter
	// Phone sign-in: requests started per client address, codes checked per signed-in user.
	deviceLoginIPs   *fixedWindowLimiter
	deviceLoginUsers *backoffLimiter
	// Recent peer answers, so that paging through an aggregated listing does not ask every peer again.
	peerCatalogs *peerCache[[]sourcedCard]
	peerFilters  *peerCache[catalogOptionsResponse]
	peerHomes    *peerCache[homeResponse]
}

func New(d Deps) *Server {
	if d.HTTPClient == nil {
		d.HTTPClient = newPeerHTTPClient()
	}
	return &Server{
		Deps:             d,
		q:                store.New(d.Pool),
		handshakes:       newFixedWindowLimiter(handshakeLimit, handshakeWindow),
		loginIPs:         newBackoffLimiter(loginIPLimit, loginWindow, loginMaxBlock),
		loginUsers:       newBackoffLimiter(loginUserLimit, loginWindow, loginMaxBlock),
		invitationIPs:    newBackoffLimiter(invitationIPLimit, loginWindow, loginMaxBlock),
		deviceLoginIPs:   newFixedWindowLimiter(deviceLoginStartLimit, time.Minute),
		deviceLoginUsers: newBackoffLimiter(deviceLoginCheckLimit, loginWindow, loginMaxBlock),
		peerCatalogs:     newPeerCache[[]sourcedCard](peerCatalogCacheTTL, d.PeerCacheTTL, peerCatalogCacheSize),
		peerFilters:      newPeerCache[catalogOptionsResponse](peerFilterCacheTTL, d.PeerCacheTTL, peerFilterCacheSize),
		peerHomes:        newPeerCache[homeResponse](peerHomeCacheTTL, d.PeerCacheTTL, peerHomeCacheSize),
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	// JSON answers are compressed: catalog listings, between peers in particular, are large and repetitive.
	r.Use(middleware.RequestID, s.resolveClientIP, requestLogger, middleware.Recoverer, middleware.Compress(5, "application/json"))

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
		r.Post("/auth/device", s.startDeviceLogin)
		r.Post("/auth/device/token", s.deviceLoginToken)

		// The token travels in the body, never in the path, so that it stays out of access logs.
		r.Post("/invitations/lookup", s.lookupInvitation)
		r.Post("/invitations/accept", s.acceptInvitation)

		// Public so that <img> tags work without a bearer token; only referenced images are served.
		r.Get("/images/tmdb/{size}/{file}", s.tmdbImage)
		r.Get("/images/fankai/{file}", s.fanKaiImage)
		r.Get("/images/local/{file}", s.localImage)
		r.Get("/images/avatars/{id}", s.userAvatar)
		r.Post("/federation/handshake", s.handshake)
		r.Get("/federation/users", s.peerUsers)
		r.Get("/federation/home", s.peerHome)
		r.Get("/federation/catalog/kinds/{kind}/items", s.peerKindItems)
		r.Get("/federation/catalog/kinds/{kind}/filters", s.peerCatalogFilterOptions)
		r.Get("/federation/catalog/titles/{kind}/{tmdbId}", s.peerTitleIDs)
		r.Get("/federation/watchlist", s.peerWatchlist)
		r.Group(func(r chi.Router) {
			r.Use(s.peerCORS)
			r.Get("/stream/{sid}/subtitles/{track}", s.streamSubtitle)
			r.Get("/stream/{sid}/{file}", s.streamFile)
			r.Options("/stream/{sid}/subtitles/{track}", noContent)
			r.Options("/stream/{sid}/{file}", noContent)

			r.Options("/items/{id}", noContent)
			r.Options("/items/{id}/progress", noContent)
			r.Options("/items/{id}/watched", noContent)
			r.Options("/items/{id}/watchlist", noContent)
			r.Options("/items/{id}/subtitles", noContent)
			r.Options("/items/{id}/subtitles/search", noContent)
			r.Options("/playback/sessions", noContent)
			r.Options("/playback/sessions/{id}", noContent)
			r.Group(func(r chi.Router) {
				r.Use(s.federatedOrLocal)
				r.Get("/items/{id}", s.getItem)
				r.Put("/items/{id}/progress", s.putProgress)
				r.Post("/items/{id}/watched", s.setWatched(true))
				r.Delete("/items/{id}/watched", s.setWatched(false))
				r.Post("/items/{id}/watchlist", s.setWatchlisted(true))
				r.Delete("/items/{id}/watchlist", s.setWatchlisted(false))
				r.Get("/items/{id}/subtitles/search", s.searchSubtitles)
				r.Post("/items/{id}/subtitles", s.downloadSubtitle)
				r.Post("/playback/sessions", s.startPlayback)
				r.Delete("/playback/sessions/{id}", s.stopPlayback)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(s.Tokens.Middleware)
			r.Get("/me", s.me)
			r.Patch("/me", s.updateMe)
			r.Put("/me/avatar", s.putAvatar)
			r.Delete("/me/avatar", s.deleteAvatar)
			r.Post("/me/federation-token", s.federationToken)
			r.Post("/device-logins/lookup", s.lookupDeviceLogin)
			r.Post("/device-logins/approve", s.approveDeviceLogin)
			r.Post("/device-logins/deny", s.denyDeviceLogin)
			r.Get("/libraries", s.listLibraries)
			r.Get("/libraries/{id}", s.getLibrary)
			r.Get("/libraries/{id}/items", s.listLibraryItems)
			r.Get("/kinds/{kind}/items", s.listKindItems)
			r.Get("/kinds/{kind}/filters", s.listCatalogFilterOptions)
			r.Get("/kinds/{kind}/titles/{tmdbId}/sources", s.listTitleSources)
			r.Delete("/kinds/{kind}/titles/{tmdbId}/watchlist", s.deleteTitleFromWatchlist)
			r.Get("/search", s.search)
			r.Get("/me/continue-watching", s.continueWatching)
			r.Get("/me/next-up", s.nextUp)
			r.Get("/me/home", s.home)
			r.Get("/me/watchlist", s.watchlist)

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Get("/users", s.listUsers)
				r.Post("/users", s.createUser)
				r.Patch("/users/{id}", s.updateUser)
				r.Delete("/users/{id}", s.deleteUser)
				r.Put("/users/{id}/alias", s.setUserAlias)
				r.Get("/invitations", s.listInvitations)
				r.Post("/invitations", s.createInvitation)
				r.Delete("/invitations/{id}", s.deleteInvitation)

				r.Post("/libraries", s.createLibrary)
				r.Patch("/libraries/{id}", s.updateLibrary)
				r.Delete("/libraries/{id}", s.deleteLibrary)
				r.Post("/libraries/{id}/scan", s.scanLibrary)

				r.Get("/admin/scans", s.listScans)
				r.Get("/admin/playback/sessions", s.listPlaybackSessions)
				r.Get("/admin/fs/browse", s.browseFS)
				r.Get("/admin/settings", s.getSettings)
				r.Get("/admin/federation/pairing-code", s.pairingCodeHandler)
				r.Get("/admin/federation/peers", s.listPeers)
				r.Post("/admin/federation/peers", s.addPeer)
				r.Post("/admin/federation/peers/{id}/sync", s.syncPeer)
				r.Patch("/admin/federation/peers/{id}", s.updatePeer)
				r.Put("/admin/federation/peers/{id}/paused", s.pausePeer)
				r.Put("/admin/federation/peers/{id}/alias", s.setPeerAlias)
				r.Delete("/admin/federation/peers/{id}", s.deletePeer)
				r.Patch("/admin/settings", s.updateSettings)
				r.Get("/admin/system/transcoding", s.transcodingInfo)

				r.Get("/items/{id}/match/search", s.searchMatch)
				r.Get("/items/{id}/match/fankai/search", s.searchFanKaiMatch)
				r.Post("/items/{id}/match/fankai", s.setFanKaiMatch)
				r.Post("/items/{id}/match", s.setMatch)
				r.Post("/items/{id}/refresh", s.refreshItem)
				r.Patch("/items/{id}/metadata", s.editItem)
				r.Get("/items/{id}/collection", s.getCollection)
				r.Put("/items/{id}/collection", s.setCollection)
				r.Get("/items/{id}/collection/search", s.searchCollectionCandidates)
				r.Delete("/items/{id}/metadata", s.revertItemEdits)
				r.Put("/items/{id}/poster", s.putPoster)
				r.Delete("/items/{id}/subtitles/{subtitleId}", s.deleteSubtitle)
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
			"client_ip", clientIP(r),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}
