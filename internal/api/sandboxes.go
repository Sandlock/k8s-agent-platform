package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/sandlock/k8s-agent-platform/internal/auth"
	proto "github.com/sandlock/k8s-agent-platform/internal/supervisorproto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	agentv1alpha1 "sigs.k8s.io/agent-sandbox/api/v1alpha1"
	extensionsv1alpha1 "sigs.k8s.io/agent-sandbox/extensions/api/v1alpha1"
)

// sandboxRecord is the in-memory fallback used when DATABASE_URL is not set.
type sandboxRecord struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Harness     string    `json:"harness"`
	Status      string    `json:"status"`
	ProviderRef string    `json:"providerRef"` // "claim/<ns>/<name>"
	CreatedAt   time.Time `json:"createdAt"`
}

type createSandboxRequest struct {
	Harness      string `json:"harness"`
	AnthropicKey string `json:"anthropicKey,omitempty"`
	UseStoredKey bool   `json:"useStoredKey,omitempty"`
	RepoURL      string `json:"repoUrl,omitempty"`
	GitHubToken  string `json:"githubToken,omitempty"`
	NoResume     bool   `json:"noResume,omitempty"`
}

func (s *Server) createSandbox(w http.ResponseWriter, r *http.Request) {
	var req createSandboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Harness == "" {
		req.Harness = "claude-code"
	}
	req.RepoURL = normalizeRepoURL(req.RepoURL)

	userID := userIDFromCtx(r.Context())

	// Resolve API key — fall back to stored key, then proceed with empty key
	// (Claude Code will prompt for one inside the terminal if needed).
	apiKey := req.AnthropicKey
	if apiKey == "" && s.db != nil {
		apiKey, _ = s.storedKeyForUser(r, userID)
	}
	ctx := r.Context()

	// Create a SandboxClaim and wait for agent-sandbox to adopt a warm pod.
	claimRef, sandboxFQDN, err := s.claimFromPool(ctx, req.Harness, 120*time.Second)
	if err != nil {
		http.Error(w, "failed to provision sandbox: "+err.Error(), http.StatusInternalServerError)
		return
	}

	id := uuid.New().String()

	// Look up prior session snapshot for this (user, repo) pair.
	var sessionSnapshot []byte
	resumed := false
	if s.db != nil && req.RepoURL != "" && !req.NoResume {
		var enc []byte
		if err := s.db.QueryRow(ctx,
			`SELECT snapshot FROM agent_snapshots WHERE user_id=$1 AND repo_url=$2`,
			userID, req.RepoURL,
		).Scan(&enc); err == nil && len(enc) > 0 {
			if dec, err := auth.DecryptBytes(enc); err == nil {
				sessionSnapshot = dec
				resumed = true
			} else {
				log.Printf("createSandbox: decrypt snapshot: %v (ignoring)", err)
			}
		}
	}

	// Send Claim to supervisor — key lives only in this in-memory call.
	supervisorURL := fmt.Sprintf("http://%s:8080/claim", sandboxFQDN)
	if err := claimSupervisor(ctx, supervisorURL, apiKey, req.GitHubToken, req.Harness, req.RepoURL, sessionSnapshot); err != nil {
		s.destroyClaim(ctx, claimRef)
		http.Error(w, "failed to reach supervisor", http.StatusBadGateway)
		return
	}
	if s.db != nil {
		_, err = s.db.Exec(ctx,
			`INSERT INTO sandboxes(id, user_id, harness, repo_url, status, provider_ref, expires_at)
			 VALUES($1,$2,$3,$4,'running',$5,$6)`,
			id, userID, req.Harness, req.RepoURL, claimRef,
			time.Now().Add(3600*time.Second),
		)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
	} else {
		s.mu.Lock()
		s.memStore[id] = &sandboxRecord{
			ID: id, UserID: userID, Harness: req.Harness,
			Status: "running", ProviderRef: claimRef, CreatedAt: time.Now(),
		}
		s.mu.Unlock()
	}

	attachURL := fmt.Sprintf("ws://%s/v1/sandboxes/%s/terminal", r.Host, id)
	writeJSON(w, http.StatusCreated, map[string]any{
		"sandboxId": id,
		"attachUrl": attachURL,
		"resumed":   resumed,
	})
}

// claimFromPool creates a SandboxClaim and waits for agent-sandbox to assign a warm pod.
// Returns the claim ref ("claim/<ns>/<name>") and the sandbox service FQDN.
func (s *Server) claimFromPool(ctx context.Context, harness string, timeout time.Duration) (string, string, error) {
	warmPool := extensionsv1alpha1.WarmPoolPolicyDefault
	ttl := int32(10)
	claim := &extensionsv1alpha1.SandboxClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "sc-",
			Namespace:    s.sandboxNS,
		},
		Spec: extensionsv1alpha1.SandboxClaimSpec{
			TemplateRef: extensionsv1alpha1.SandboxTemplateRef{Name: harness},
			WarmPool:    &warmPool,
			Lifecycle: &extensionsv1alpha1.Lifecycle{
				ShutdownPolicy:          extensionsv1alpha1.ShutdownPolicyDelete,
				TTLSecondsAfterFinished: &ttl,
			},
		},
	}
	if err := s.k8s.Create(ctx, claim); err != nil {
		return "", "", fmt.Errorf("create SandboxClaim: %w", err)
	}
	claimRef := fmt.Sprintf("claim/%s/%s", s.sandboxNS, claim.Name)

	// Poll until agent-sandbox adopts a warm pod (status.sandbox.name populated).
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			s.destroyClaim(context.Background(), claimRef)
			return "", "", ctx.Err()
		case <-time.After(2 * time.Second):
		}

		var current extensionsv1alpha1.SandboxClaim
		if err := s.k8s.Get(ctx, types.NamespacedName{Namespace: s.sandboxNS, Name: claim.Name}, &current); err != nil {
			continue
		}
		sbName := current.Status.SandboxStatus.Name
		if sbName == "" {
			continue
		}

		// Sandbox is adopted — fetch its service FQDN.
		var sb agentv1alpha1.Sandbox
		if err := s.k8s.Get(ctx, types.NamespacedName{Namespace: s.sandboxNS, Name: sbName}, &sb); err != nil {
			continue
		}
		fqdn := sb.Status.ServiceFQDN
		if fqdn == "" {
			// Fall back to constructed FQDN if status not yet populated.
			fqdn = fmt.Sprintf("%s.%s.svc.cluster.local", sbName, s.sandboxNS)
		}
		return claimRef, fqdn, nil
	}

	s.destroyClaim(context.Background(), claimRef)
	return "", "", fmt.Errorf("timeout waiting for warm pool pod after %s", timeout)
}

// destroyClaim deletes a SandboxClaim by ref ("claim/<ns>/<name>").
// agent-sandbox cascades deletion to the Sandbox and Pod via ShutdownPolicyDelete.
func (s *Server) destroyClaim(ctx context.Context, ref string) {
	_, ns, name, ok := parseClaimRef(ref)
	if !ok {
		return
	}
	claim := &extensionsv1alpha1.SandboxClaim{}
	claim.Name = name
	claim.Namespace = ns
	_ = s.k8s.Delete(ctx, claim)
}

// parseClaimRef parses "claim/<ns>/<name>" into its parts.
func parseClaimRef(ref string) (kind, ns, name string, ok bool) {
	parts := strings.SplitN(ref, "/", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// sandboxFQDNFromClaimRef looks up the sandbox FQDN for a stored claim ref.
func (s *Server) sandboxFQDNFromClaimRef(ctx context.Context, ref string) (string, error) {
	_, ns, name, ok := parseClaimRef(ref)
	if !ok {
		return "", fmt.Errorf("invalid provider ref %q", ref)
	}
	var claim extensionsv1alpha1.SandboxClaim
	if err := s.k8s.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &claim); err != nil {
		return "", fmt.Errorf("get SandboxClaim: %w", err)
	}
	sbName := claim.Status.SandboxStatus.Name
	if sbName == "" {
		return "", fmt.Errorf("SandboxClaim %s/%s not yet adopted", ns, name)
	}
	var sb agentv1alpha1.Sandbox
	if err := s.k8s.Get(ctx, types.NamespacedName{Namespace: ns, Name: sbName}, &sb); err != nil {
		return "", fmt.Errorf("get Sandbox: %w", err)
	}
	if fqdn := sb.Status.ServiceFQDN; fqdn != "" {
		return fqdn, nil
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local", sbName, ns), nil
}

func claimSupervisor(ctx context.Context, url, key, githubToken, harness, repoURL string, sessionSnapshot []byte) error {
	body, _ := json.Marshal(proto.ClaimRequest{
		AnthropicKey:    key,
		GitHubToken:     githubToken,
		Harness:         harness,
		RepoURL:         repoURL,
		SessionSnapshot: sessionSnapshot,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("supervisor returned %d", resp.StatusCode)
	}
	return nil
}

func (s *Server) listSandboxes(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromCtx(r.Context())
	if s.db != nil {
		rows, err := s.db.Query(r.Context(),
			`SELECT id, harness, status, provider_ref, created_at FROM sandboxes WHERE user_id=$1 AND status != 'gone' ORDER BY created_at DESC`,
			userID)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		type row struct {
			ID          string    `json:"id"`
			Harness     string    `json:"harness"`
			Status      string    `json:"status"`
			ProviderRef string    `json:"providerRef"`
			CreatedAt   time.Time `json:"createdAt"`
		}
		var list []row
		for rows.Next() {
			var r row
			rows.Scan(&r.ID, &r.Harness, &r.Status, &r.ProviderRef, &r.CreatedAt)
			list = append(list, r)
		}
		writeJSON(w, http.StatusOK, list)
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*sandboxRecord
	for _, sb := range s.memStore {
		if sb.UserID == userID {
			list = append(list, sb)
		}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) getSandbox(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := userIDFromCtx(r.Context())
	sb, ok := s.lookupSandbox(r, id, userID)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, sb)
}


func (s *Server) stopSandbox(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := userIDFromCtx(r.Context())
	sb, ok := s.lookupSandbox(r, id, userID)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var ref string
	switch v := sb.(type) {
	case *sandboxRecord:
		ref = v.ProviderRef
	default:
		_ = v
		if s.db != nil {
			s.db.QueryRow(r.Context(),
				`SELECT provider_ref FROM sandboxes WHERE id=$1 AND user_id=$2`,
				id, userID,
			).Scan(&ref)
		}
	}

	// Snapshot Claude Code session state before pod teardown (non-fatal).
	if s.db != nil && ref != "" {
		if fqdn, err := s.sandboxFQDNFromClaimRef(r.Context(), ref); err == nil {
			var repoURL string
			s.db.QueryRow(r.Context(), `SELECT repo_url FROM sandboxes WHERE id=$1 AND user_id=$2`, id, userID).Scan(&repoURL)
			if repoURL != "" {
				if err := s.snapshotSession(r.Context(), userID, repoURL, fqdn); err != nil {
					log.Printf("stopSandbox: snapshot: %v (non-fatal)", err)
				}
			}
		}
	}

	if s.db != nil {
		s.db.Exec(r.Context(), `UPDATE sandboxes SET status='gone' WHERE id=$1`, id)
	} else {
		s.mu.Lock()
		if rec, ok := s.memStore[id]; ok {
			rec.Status = "gone"
			ref = rec.ProviderRef
		}
		s.mu.Unlock()
	}

	s.destroyClaim(r.Context(), ref)
	w.WriteHeader(http.StatusNoContent)
}

const maxSnapshotBytes = 15 << 20 // 15 MB

// snapshotSession fetches the Claude Code session state from a live pod,
// encrypts it, and upserts it into agent_snapshots.
func (s *Server) snapshotSession(ctx context.Context, userID, repoURL, fqdn string) error {
	snapshotCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(snapshotCtx, http.MethodGet, fmt.Sprintf("http://%s:8080/snapshot", fqdn), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET /snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil // no session data to save
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /snapshot: status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSnapshotBytes))
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}
	if len(raw) == 0 {
		return nil
	}

	enc, err := auth.EncryptBytes(raw)
	if err != nil {
		return fmt.Errorf("encrypt snapshot: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO agent_snapshots(user_id, repo_url, snapshot)
		 VALUES($1, $2, $3)
		 ON CONFLICT (user_id, repo_url)
		 DO UPDATE SET snapshot=EXCLUDED.snapshot, snapshot_at=NOW()`,
		userID, repoURL, enc,
	)
	return err
}

// terminalProxy upgrades to WebSocket and proxies to the pod's terminal bridge (:8081).
func (s *Server) terminalProxy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := userIDFromCtx(r.Context())
	ref, ok := s.providerRef(r, id, userID)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	fqdn, err := s.sandboxFQDNFromClaimRef(r.Context(), ref)
	if err != nil {
		http.Error(w, "cannot resolve sandbox: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.proxyWebSocket(w, r, fmt.Sprintf("ws://%s:8081/", fqdn))
}

// tunnelProxy is the CLI attach endpoint — identical to terminalProxy.
func (s *Server) tunnelProxy(w http.ResponseWriter, r *http.Request) {
	s.terminalProxy(w, r)
}

func (s *Server) proxyWebSocket(w http.ResponseWriter, r *http.Request, podURL string) {
	client, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer client.CloseNow()

	pod, _, err := websocket.Dial(r.Context(), podURL, nil)
	if err != nil {
		client.Close(websocket.StatusInternalError, "could not reach sandbox")
		return
	}
	defer pod.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// pod → client: preserve message type so binary PTY output and text resize acks pass through.
	go func() {
		defer cancel()
		for {
			mt, msg, err := pod.Read(ctx)
			if err != nil {
				return
			}
			if err := client.Write(ctx, mt, msg); err != nil {
				return
			}
		}
	}()

	// client → pod: text resize messages and binary keystrokes both forwarded as-is.
	for {
		mt, msg, err := client.Read(ctx)
		if err != nil {
			return
		}
		if err := pod.Write(ctx, mt, msg); err != nil {
			return
		}
	}
}

// lookupSandbox returns the sandbox record.
func (s *Server) lookupSandbox(r *http.Request, id, userID string) (any, bool) {
	if s.db != nil {
		type dbRow struct {
			ID          string    `json:"id"`
			Harness     string    `json:"harness"`
			Status      string    `json:"status"`
			ProviderRef string    `json:"providerRef"`
			CreatedAt   time.Time `json:"createdAt"`
		}
		var row dbRow
		err := s.db.QueryRow(r.Context(),
			`SELECT id, harness, status, provider_ref, created_at FROM sandboxes WHERE id=$1 AND user_id=$2`,
			id, userID,
		).Scan(&row.ID, &row.Harness, &row.Status, &row.ProviderRef, &row.CreatedAt)
		if err != nil {
			return nil, false
		}
		return row, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	sb, ok := s.memStore[id]
	if !ok || sb.UserID != userID {
		return nil, false
	}
	return sb, true
}

func (s *Server) providerRef(r *http.Request, id, userID string) (string, bool) {
	if s.db != nil {
		var ref string
		err := s.db.QueryRow(r.Context(),
			`SELECT provider_ref FROM sandboxes WHERE id=$1 AND user_id=$2 AND status != 'gone'`,
			id, userID,
		).Scan(&ref)
		return ref, err == nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	sb, ok := s.memStore[id]
	if !ok || sb.UserID != userID {
		return "", false
	}
	return sb.ProviderRef, true
}

// normalizeRepoURL trims trailing ".git" and "/" so that URLs from the GitHub
// API (clone_url ends in .git) match URLs typed manually by users.
func normalizeRepoURL(u string) string {
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return u
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
