package api

import (
	"encoding/json"
	"net/http"

	"github.com/sandlock/k8s-agent-platform/internal/auth"
)

type storeKeyRequest struct {
	AnthropicKey string `json:"anthropicKey"`
}

// storeKey envelope-encrypts the BYOK key and stores ciphertext in the DB.
// The plaintext key is never persisted.
func (s *Server) storeKey(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "key storage requires DATABASE_URL", http.StatusNotImplemented)
		return
	}
	var req storeKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AnthropicKey == "" {
		http.Error(w, "anthropicKey required", http.StatusBadRequest)
		return
	}

	ciphertext, err := auth.EncryptKey(req.AnthropicKey)
	if err != nil {
		http.Error(w, "encryption failed — is MASTER_KEY set?", http.StatusInternalServerError)
		return
	}

	hint := req.AnthropicKey
	if len(hint) > 4 {
		hint = hint[len(hint)-4:]
	}

	userID := userIDFromCtx(r.Context())
	// Replace any existing key for this user.
	s.db.Exec(r.Context(), `DELETE FROM api_keys WHERE user_id=$1`, userID)
	_, err = s.db.Exec(r.Context(),
		`INSERT INTO api_keys(user_id, ciphertext, key_hint) VALUES($1,$2,$3)`,
		userID, ciphertext, hint,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"hint": "…" + hint})
}

func (s *Server) deleteKey(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	userID := userIDFromCtx(r.Context())
	s.db.Exec(r.Context(), `DELETE FROM api_keys WHERE user_id=$1`, userID)
	w.WriteHeader(http.StatusNoContent)
}

// storedKeyForUser decrypts and returns the user's stored BYOK key, or "" if none.
func (s *Server) storedKeyForUser(r *http.Request, userID string) (string, error) {
	var ciphertext []byte
	err := s.db.QueryRow(r.Context(),
		`SELECT ciphertext FROM api_keys WHERE user_id=$1`, userID,
	).Scan(&ciphertext)
	if err != nil {
		return "", nil // no stored key
	}
	return auth.DecryptKey(ciphertext)
}
