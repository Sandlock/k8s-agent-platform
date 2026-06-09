package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sandlock/k8s-agent-platform/internal/auth"
)

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "auth not available without DATABASE_URL", http.StatusNotImplemented)
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var id string
	err = s.db.QueryRow(r.Context(),
		`INSERT INTO users(username, password_hash) VALUES($1,$2) RETURNING id`,
		req.Username, hash,
	).Scan(&id)
	if err != nil {
		http.Error(w, "username already taken", http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"userId": id})
}

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
	err := s.db.QueryRow(r.Context(),
		`SELECT id, password_hash FROM users WHERE username=$1`, req.Username,
	).Scan(&userID, &storedHash)
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
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "expiresAt": expires.Format(time.RFC3339)})
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
