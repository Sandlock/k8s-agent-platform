// Package api wires together the HTTP router for the Sandlock control plane.
package api

import (
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sandlock/k8s-agent-platform/internal/pool"
	"github.com/sandlock/k8s-agent-platform/internal/provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Server holds shared dependencies for all HTTP handlers.
type Server struct {
	k8s      client.Client
	prov     provider.Provider
	poolMgr  *pool.Manager
	db       *pgxpool.Pool // nil when DATABASE_URL is not set

	// in-memory fallback when DB is unavailable (M1 dev mode)
	mu        sync.RWMutex
	memStore  map[string]*sandboxRecord
}

// NewServer creates a Server. db may be nil for no-auth dev mode.
func NewServer(k8s client.Client, prov provider.Provider, poolMgr *pool.Manager, db *pgxpool.Pool) *Server {
	return &Server{
		k8s:      k8s,
		prov:     prov,
		poolMgr:  poolMgr,
		db:       db,
		memStore: make(map[string]*sandboxRecord),
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

	// Auth endpoints — no session required.
	r.Post("/v1/auth/register", s.register)
	r.Post("/v1/auth/login", s.login)
	r.Post("/v1/auth/logout", s.logout)

	// BYOK key endpoints.
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Post("/v1/keys", s.storeKey)
		r.Delete("/v1/keys", s.deleteKey)
	})

	// Sandbox endpoints.
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Post("/v1/sandboxes", s.createSandbox)
		r.Get("/v1/sandboxes", s.listSandboxes)
		r.Get("/v1/sandboxes/{id}", s.getSandbox)
		r.Delete("/v1/sandboxes/{id}", s.stopSandbox)
		r.Get("/v1/sandboxes/{id}/terminal", s.terminalProxy)
		r.Get("/v1/sandboxes/{id}/tunnel", s.tunnelProxy)
	})

	return r
}
