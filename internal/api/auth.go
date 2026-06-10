package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sandlock/k8s-agent-platform/internal/auth"
)


type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "auth not available without DATABASE_URL", http.StatusNotImplemented)
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var userID, storedHash string
	var isAdmin, mustChange bool
	err := s.db.QueryRow(r.Context(),
		`SELECT id, password_hash, is_admin, must_change_password FROM users WHERE username=$1`,
		req.Username,
	).Scan(&userID, &storedHash, &isAdmin, &mustChange)
	if err != nil || !auth.CheckPassword(req.Password, storedHash) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, tokenHash, err := auth.NewToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	expires := time.Now().Add(24 * time.Hour)
	_, err = s.db.Exec(r.Context(),
		`INSERT INTO sessions(user_id, token_hash, expires_at) VALUES($1,$2,$3)`,
		userID, tokenHash, expires,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":              token,
		"expiresAt":          expires.Format(time.RFC3339),
		"isAdmin":            isAdmin,
		"mustChangePassword": mustChange,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	token := bearerToken(r)
	if token == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	s.db.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash=$1`, auth.HashToken(token))
	w.WriteHeader(http.StatusNoContent)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "auth not available without DATABASE_URL", http.StatusNotImplemented)
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewPassword == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	userID := userIDFromCtx(r.Context())

	var storedHash string
	if err := s.db.QueryRow(r.Context(),
		`SELECT password_hash FROM users WHERE id=$1`, userID,
	).Scan(&storedHash); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !auth.CheckPassword(req.CurrentPassword, storedHash) {
		http.Error(w, "current password incorrect", http.StatusUnauthorized)
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, err = s.db.Exec(r.Context(),
		`UPDATE users SET password_hash=$1, must_change_password=false WHERE id=$2`,
		newHash, userID,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
