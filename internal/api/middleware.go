package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/sandlock/k8s-agent-platform/internal/auth"
)

type ctxKey string

const ctxUserID ctxKey = "userID"

func userIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}

// requireAuth validates the Bearer session token and injects the user ID into the context.
// When DB is not configured, it injects a synthetic user ID so M1 dev mode still works.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.db == nil {
			// No-auth dev mode: inject a fixed user ID.
			ctx := context.WithValue(r.Context(), ctxUserID, "dev-user")
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
		err := s.db.QueryRow(r.Context(),
			`SELECT user_id FROM sessions WHERE token_hash=$1 AND expires_at > $2`,
			hash, time.Now(),
		).Scan(&userID)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}
