package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sandlock/k8s-agent-platform/internal/auth"
)

type userRow struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	IsAdmin   bool      `json:"isAdmin"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(),
		`SELECT id, username, is_admin, created_at FROM users ORDER BY created_at`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var users []userRow
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.ID, &u.Username, &u.IsAdmin, &u.CreatedAt); err == nil {
			users = append(users, u)
		}
	}
	writeJSON(w, http.StatusOK, users)
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"isAdmin"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
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
		`INSERT INTO users(username, password_hash, is_admin, must_change_password)
		 VALUES($1,$2,$3,true) RETURNING id`,
		req.Username, hash, req.IsAdmin,
	).Scan(&id)
	if err != nil {
		http.Error(w, "username already taken", http.StatusConflict)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"userId": id})
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "id")
	callerID := userIDFromCtx(r.Context())
	if targetID == callerID {
		http.Error(w, "cannot delete yourself", http.StatusBadRequest)
		return
	}
	_, err := s.db.Exec(r.Context(), `DELETE FROM users WHERE id=$1`, targetID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
