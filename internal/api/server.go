// Package api wires together the HTTP router for the Sandlock control plane.
package api

import (
	"io/fs"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	webui "github.com/sandlock/k8s-agent-platform/web"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Server holds shared dependencies for all HTTP handlers.
type Server struct {
	k8s       client.Client
	sandboxNS string
	db        *pgxpool.Pool // nil when DATABASE_URL is not set

	// in-memory fallback when DB is unavailable (dev mode)
	mu       sync.RWMutex
	memStore map[string]*sandboxRecord
}

// NewServer creates a Server. db may be nil for no-auth dev mode.
func NewServer(k8s client.Client, sandboxNS string, db *pgxpool.Pool) *Server {
	return &Server{
		k8s:       k8s,
		sandboxNS: sandboxNS,
		db:        db,
		memStore:  make(map[string]*sandboxRecord),
	}
}

// Handler builds and returns the chi router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// Health probes.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	// Web dashboard — serve embedded web/dist, fallback to index.html for SPA routing.
	distFS, _ := fs.Sub(webui.Dist, "dist")
	fileServer := http.FileServer(http.FS(distFS))
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		// Serve the file if it exists, otherwise fall back to index.html.
		if _, err := distFS.Open(req.URL.Path[1:]); err != nil {
			req.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, req)
	})

	// Auth endpoints — no session required.
	r.Post("/v1/auth/register", s.register)
	r.Post("/v1/auth/login", s.login)
	r.Post("/v1/auth/logout", s.logout)

	// Authenticated endpoints.
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Put("/v1/auth/password", s.changePassword)

		r.Post("/v1/keys", s.storeKey)
		r.Delete("/v1/keys", s.deleteKey)

		r.Post("/v1/sandboxes", s.createSandbox)
		r.Get("/v1/sandboxes", s.listSandboxes)
		r.Get("/v1/sandboxes/{id}", s.getSandbox)
		r.Delete("/v1/sandboxes/{id}", s.stopSandbox)
		r.Get("/v1/sandboxes/{id}/terminal", s.terminalProxy)
		r.Get("/v1/sandboxes/{id}/tunnel", s.tunnelProxy)
	})

	// Admin-only user management.
	r.Group(func(r chi.Router) {
		r.Use(s.requireAdmin)
		r.Get("/v1/users", s.listUsers)
		r.Post("/v1/users", s.createUser)
		r.Delete("/v1/users/{id}", s.deleteUser)
	})

	return r
}
