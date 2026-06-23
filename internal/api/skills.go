package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
)

var skillNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

type skillRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type upsertSkillRequest struct {
	Content string `json:"content"`
}

func (s *Server) listSkills(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, []skillRecord{})
		return
	}
	userID := userIDFromCtx(r.Context())
	rows, err := s.db.Query(r.Context(),
		`SELECT id, name, content, created_at FROM user_skills WHERE user_id=$1 ORDER BY name`,
		userID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []skillRecord
	for rows.Next() {
		var rec skillRecord
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Content, &rec.CreatedAt); err == nil {
			list = append(list, rec)
		}
	}
	if list == nil {
		list = []skillRecord{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) putSkill(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "skill storage requires DATABASE_URL", http.StatusNotImplemented)
		return
	}
	name := chi.URLParam(r, "name")
	if !skillNameRE.MatchString(name) {
		http.Error(w, "invalid skill name: must match ^[a-z0-9][a-z0-9_-]{0,63}$", http.StatusBadRequest)
		return
	}
	var req upsertSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		http.Error(w, "content required", http.StatusBadRequest)
		return
	}
	userID := userIDFromCtx(r.Context())
	var id string
	err := s.db.QueryRow(r.Context(),
		`INSERT INTO user_skills(user_id, name, content)
		 VALUES($1, $2, $3)
		 ON CONFLICT (user_id, name)
		 DO UPDATE SET content=EXCLUDED.content
		 RETURNING id`,
		userID, name, req.Content,
	).Scan(&id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "name": name})
}

func (s *Server) deleteSkill(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	name := chi.URLParam(r, "name")
	userID := userIDFromCtx(r.Context())
	s.db.Exec(r.Context(),
		`DELETE FROM user_skills WHERE user_id=$1 AND name=$2`,
		userID, name)
	w.WriteHeader(http.StatusNoContent)
}
