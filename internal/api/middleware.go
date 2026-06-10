package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/sandlock/k8s-agent-platform/internal/auth"
)

type ctxKey string

const (
	ctxUserID  ctxKey = "userID"
	ctxIsAdmin ctxKey = "isAdmin"
)

func userIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}

func isAdminFromCtx(ctx context.Context) bool {
	v, _ := ctx.Value(ctxIsAdmin).(bool)
	return v
}

// requireAuth validates the Bearer session token and injects the user ID and
// admin flag into the context.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.db == nil {
			ctx := context.WithValue(r.Context(), ctxUserID, "dev-user")
			ctx = context.WithValue(ctx, ctxIsAdmin, true)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		token := bearerToken(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		hash := auth.HashToken(token)
		var userID string
		var isAdmin bool
		err := s.db.QueryRow(r.Context(),
			`SELECT s.user_id, u.is_admin
			 FROM sessions s JOIN users u ON u.id = s.user_id
			 WHERE s.token_hash=$1 AND s.expires_at > $2`,
			hash, time.Now(),
		).Scan(&userID, &isAdmin)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, userID)
		ctx = context.WithValue(ctx, ctxIsAdmin, isAdmin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAdmin is like requireAuth but also enforces the is_admin flag.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return s.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAdminFromCtx(r.Context()) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func bearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if after, ok := strings.CutPrefix(h, "Bearer "); ok {
			return strings.TrimSpace(after)
		}
	}
	// WebSocket clients can't set headers; accept token as query param.
	return r.URL.Query().Get("token")
}
