package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sandlock/k8s-agent-platform/internal/auth"
	proto "github.com/sandlock/k8s-agent-platform/internal/supervisorproto"
)

type mcpServerRecord struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	URL              string            `json:"url,omitempty"`
	Command          string            `json:"command,omitempty"`
	Args             []string          `json:"args,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
	SecretEnvKeys    []string          `json:"secretEnvKeys,omitempty"`
	SecretHeaderKeys []string          `json:"secretHeaderKeys,omitempty"`
	DisplayName      string            `json:"displayName,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

type upsertMCPServerRequest struct {
	Type          string            `json:"type"`
	URL           string            `json:"url,omitempty"`
	Command       string            `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	SecretEnv     map[string]string `json:"secretEnv,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	SecretHeaders map[string]string `json:"secretHeaders,omitempty"`
	DisplayName   string            `json:"displayName,omitempty"`
}

func (s *Server) listMCPServers(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, http.StatusOK, []mcpServerRecord{})
		return
	}
	userID := userIDFromCtx(r.Context())
	rows, err := s.db.Query(r.Context(),
		`SELECT id, name, server_type, COALESCE(url,''), COALESCE(command,''),
		        COALESCE(args,'[]'::jsonb), COALESCE(env_vars,'{}'::jsonb),
		        COALESCE(headers,'{}'::jsonb),
		        secret_env_ciphertext, secret_headers_ciphertext,
		        display_name, created_at, updated_at
		 FROM user_mcp_servers WHERE user_id=$1 ORDER BY name`,
		userID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []mcpServerRecord
	for rows.Next() {
		var rec mcpServerRecord
		var argsJSON, envJSON, headersJSON []byte
		var secretEnvCT, secretHeadersCT []byte
		if err := rows.Scan(
			&rec.ID, &rec.Name, &rec.Type, &rec.URL, &rec.Command,
			&argsJSON, &envJSON, &headersJSON,
			&secretEnvCT, &secretHeadersCT,
			&rec.DisplayName, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			continue
		}
		json.Unmarshal(argsJSON, &rec.Args)
		json.Unmarshal(envJSON, &rec.Env)
		json.Unmarshal(headersJSON, &rec.Headers)
		rec.SecretEnvKeys = secretKeyNames(secretEnvCT)
		rec.SecretHeaderKeys = secretKeyNames(secretHeadersCT)
		list = append(list, rec)
	}
	if list == nil {
		list = []mcpServerRecord{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) putMCPServer(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "mcp server storage requires DATABASE_URL", http.StatusNotImplemented)
		return
	}
	name := chi.URLParam(r, "name")
	if !skillNameRE.MatchString(name) {
		http.Error(w, "invalid name: must match ^[a-z0-9][a-z0-9_-]{0,63}$", http.StatusBadRequest)
		return
	}
	var req upsertMCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch req.Type {
	case "http", "sse":
		if !strings.HasPrefix(req.URL, "http") {
			http.Error(w, "url required and must begin with http", http.StatusBadRequest)
			return
		}
	case "stdio":
		if req.Command == "" {
			http.Error(w, "command required for stdio type", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "type must be http, sse, or stdio", http.StatusBadRequest)
		return
	}

	userID := userIDFromCtx(r.Context())

	// Preserve existing ciphertext when secrets are not re-submitted.
	var existingSecretEnvCT, existingSecretHeadersCT []byte
	s.db.QueryRow(r.Context(),
		`SELECT secret_env_ciphertext, secret_headers_ciphertext
		 FROM user_mcp_servers WHERE user_id=$1 AND name=$2`,
		userID, name,
	).Scan(&existingSecretEnvCT, &existingSecretHeadersCT)

	secretEnvCT := existingSecretEnvCT
	if len(req.SecretEnv) > 0 {
		b, err := json.Marshal(req.SecretEnv)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		secretEnvCT, err = auth.EncryptBytes(b)
		if err != nil {
			http.Error(w, "encryption failed — is MASTER_KEY set?", http.StatusInternalServerError)
			return
		}
	}

	secretHeadersCT := existingSecretHeadersCT
	if len(req.SecretHeaders) > 0 {
		b, err := json.Marshal(req.SecretHeaders)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		secretHeadersCT, err = auth.EncryptBytes(b)
		if err != nil {
			http.Error(w, "encryption failed — is MASTER_KEY set?", http.StatusInternalServerError)
			return
		}
	}

	argsJSON, _ := json.Marshal(req.Args)
	envJSON, _ := json.Marshal(req.Env)
	headersJSON, _ := json.Marshal(req.Headers)

	var id string
	err := s.db.QueryRow(r.Context(),
		`INSERT INTO user_mcp_servers
		    (user_id, name, server_type, url, command, args, env_vars,
		     secret_env_ciphertext, headers, secret_headers_ciphertext, display_name)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 ON CONFLICT (user_id, name) DO UPDATE SET
		    server_type               = EXCLUDED.server_type,
		    url                       = EXCLUDED.url,
		    command                   = EXCLUDED.command,
		    args                      = EXCLUDED.args,
		    env_vars                  = EXCLUDED.env_vars,
		    secret_env_ciphertext     = EXCLUDED.secret_env_ciphertext,
		    headers                   = EXCLUDED.headers,
		    secret_headers_ciphertext = EXCLUDED.secret_headers_ciphertext,
		    display_name              = EXCLUDED.display_name,
		    updated_at                = NOW()
		 RETURNING id`,
		userID, name, req.Type, req.URL, req.Command,
		argsJSON, envJSON, secretEnvCT, headersJSON, secretHeadersCT, req.DisplayName,
	).Scan(&id)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "name": name})
}

func (s *Server) deleteMCPServer(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	name := chi.URLParam(r, "name")
	userID := userIDFromCtx(r.Context())
	s.db.Exec(r.Context(),
		`DELETE FROM user_mcp_servers WHERE user_id=$1 AND name=$2`,
		userID, name)
	w.WriteHeader(http.StatusNoContent)
}

// mcpServersForUser returns the user's MCP servers with secrets decrypted and
// merged into Env/Headers. The returned structs must not be logged or persisted.
func (s *Server) mcpServersForUser(ctx context.Context, userID string) ([]proto.MCPServer, error) {
	rows, err := s.db.Query(ctx,
		`SELECT name, server_type, COALESCE(url,''), COALESCE(command,''),
		        COALESCE(args,'[]'::jsonb), COALESCE(env_vars,'{}'::jsonb),
		        secret_env_ciphertext, COALESCE(headers,'{}'::jsonb),
		        secret_headers_ciphertext
		 FROM user_mcp_servers WHERE user_id=$1 ORDER BY name`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []proto.MCPServer
	for rows.Next() {
		var srv proto.MCPServer
		var argsJSON, envJSON, headersJSON []byte
		var secretEnvCT, secretHeadersCT []byte
		if err := rows.Scan(
			&srv.Name, &srv.Type, &srv.URL, &srv.Command,
			&argsJSON, &envJSON, &secretEnvCT, &headersJSON, &secretHeadersCT,
		); err != nil {
			continue
		}
		json.Unmarshal(argsJSON, &srv.Args)

		env := map[string]string{}
		json.Unmarshal(envJSON, &env)
		if len(secretEnvCT) > 0 {
			if plain, err := auth.DecryptBytes(secretEnvCT); err == nil {
				var secrets map[string]string
				if json.Unmarshal(plain, &secrets) == nil {
					for k, v := range secrets {
						env[k] = v
					}
				}
			}
		}
		if len(env) > 0 {
			srv.Env = env
		}

		headers := map[string]string{}
		json.Unmarshal(headersJSON, &headers)
		if len(secretHeadersCT) > 0 {
			if plain, err := auth.DecryptBytes(secretHeadersCT); err == nil {
				var secrets map[string]string
				if json.Unmarshal(plain, &secrets) == nil {
					for k, v := range secrets {
						headers[k] = v
					}
				}
			}
		}
		if len(headers) > 0 {
			srv.Headers = headers
		}

		servers = append(servers, srv)
	}
	return servers, nil
}

// secretKeyNames decrypts the ciphertext and returns only the map keys.
// Used to show users which secret keys exist without revealing values.
func secretKeyNames(ciphertext []byte) []string {
	if len(ciphertext) == 0 {
		return nil
	}
	plain, err := auth.DecryptBytes(ciphertext)
	if err != nil {
		return nil
	}
	var m map[string]string
	if json.Unmarshal(plain, &m) != nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
