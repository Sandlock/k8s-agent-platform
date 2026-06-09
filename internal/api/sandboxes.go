package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	sandlockv1alpha1 "github.com/sandlock/k8s-agent-platform/api/v1alpha1"
	"github.com/sandlock/k8s-agent-platform/internal/provider"
	proto "github.com/sandlock/k8s-agent-platform/internal/supervisorproto"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// sandboxRecord is the in-memory fallback used when DATABASE_URL is not set.
type sandboxRecord struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Harness     string    `json:"harness"`
	Status      string    `json:"status"`
	ProviderRef string    `json:"providerRef"`
	CreatedAt   time.Time `json:"createdAt"`
}

type createSandboxRequest struct {
	Harness       string `json:"harness"`
	AnthropicKey  string `json:"anthropicKey,omitempty"`
	UseStoredKey  bool   `json:"useStoredKey,omitempty"`
	RepoURL       string `json:"repoUrl,omitempty"`
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

	userID := userIDFromCtx(r.Context())

	// Resolve API key.
	apiKey := req.AnthropicKey
	if apiKey == "" && req.UseStoredKey && s.db != nil {
		var err error
		apiKey, err = s.storedKeyForUser(r, userID)
		if err != nil || apiKey == "" {
			http.Error(w, "no stored key found — POST /v1/keys first", http.StatusBadRequest)
			return
		}
	}
	ctx := r.Context()

	// Try to claim a warm pool pod first; fall back to cold provision.
	handle, err := s.claimFromPoolOrProvision(ctx, req.Harness, userID)
	if err != nil {
		http.Error(w, "failed to provision sandbox", http.StatusInternalServerError)
		return
	}

	if err := s.waitForReady(ctx, handle, 120*time.Second); err != nil {
		s.prov.Destroy(ctx, handle)
		http.Error(w, "sandbox timed out", http.StatusGatewayTimeout)
		return
	}

	// Send Claim to supervisor — key lives only in this in-memory call.
	supervisorURL := fmt.Sprintf("http://sandbox-%s.%s.svc.cluster.local:8080/claim",
		handle.Name, handle.Namespace)
	if err := claimSupervisor(ctx, supervisorURL, apiKey, req.Harness, req.RepoURL); err != nil {
		s.prov.Destroy(ctx, handle)
		http.Error(w, "failed to reach supervisor", http.StatusBadGateway)
		return
	}

	// Warm a replacement in the background.
	go s.poolMgr.WarmOne(ctx, handle.Namespace, req.Harness)

	id := uuid.New().String()
	if s.db != nil {
		_, err = s.db.Exec(ctx,
			`INSERT INTO sandboxes(id, user_id, harness, repo_url, status, provider_ref, expires_at)
			 VALUES($1,$2,$3,$4,'running',$5,$6)`,
			id, userID, req.Harness, req.RepoURL, handle.String(),
			time.Now().Add(time.Duration(3600)*time.Second),
		)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
	} else {
		s.mu.Lock()
		s.memStore[id] = &sandboxRecord{
			ID: id, UserID: userID, Harness: req.Harness,
			Status: "running", ProviderRef: handle.String(), CreatedAt: time.Now(),
		}
		s.mu.Unlock()
	}

	attachURL := fmt.Sprintf("ws://%s/v1/sandboxes/%s/terminal", r.Host, id)
	writeJSON(w, http.StatusCreated, map[string]string{
		"sandboxId": id,
		"attachUrl": attachURL,
	})
}

// claimFromPoolOrProvision claims a warm pool pod from Kubernetes or cold-provisions one.
func (s *Server) claimFromPoolOrProvision(ctx context.Context, harness, userID string) (provider.Handle, error) {
	if h, ok := s.claimFromPool(ctx, harness, userID); ok {
		return h, nil
	}
	return s.prov.Provision(ctx, provider.SandboxSpec{
		Harness: harness, CPULimit: "1", MemoryLimit: "2Gi",
		TTLSeconds: 3600, IdleTimeoutSeconds: 900,
	})
}

// claimFromPool finds a Ready warm pool Sandbox CR and atomically marks it Claimed.
// Uses optimistic locking: if two requests race, one gets a conflict and retries the next pod.
func (s *Server) claimFromPool(ctx context.Context, harness, userID string) (provider.Handle, bool) {
	var list sandlockv1alpha1.SandboxList
	if err := s.k8s.List(ctx, &list, client.InNamespace("sandboxes")); err != nil {
		return provider.Handle{}, false
	}
	for i := range list.Items {
		sb := &list.Items[i]
		if !sb.Spec.Pool {
			continue
		}
		if sb.Status.Phase != sandlockv1alpha1.PhaseReady {
			continue
		}
		if sb.Status.ClaimedBy != "" {
			continue
		}
		// Attempt optimistic claim — will conflict if another request beats us to it.
		patch := sb.DeepCopy()
		patch.Status.Phase = sandlockv1alpha1.PhaseClaimed
		patch.Status.ClaimedBy = userID
		if err := s.k8s.Status().Update(ctx, patch); err != nil {
			continue // conflict or transient error — try the next pod
		}
		return provider.Handle{Namespace: sb.Namespace, Name: sb.Name}, true
	}
	return provider.Handle{}, false
}

func (s *Server) waitForReady(ctx context.Context, h provider.Handle, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		phase, err := s.prov.Status(ctx, h)
		if err != nil {
			return err
		}
		if phase == provider.PhaseReady || phase == provider.PhaseClaimed {
			return nil
		}
		if phase == provider.PhaseFailed {
			return fmt.Errorf("sandbox failed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("timeout after %s", timeout)
}

func claimSupervisor(ctx context.Context, url, key, harness, repoURL string) error {
	body, _ := json.Marshal(proto.ClaimRequest{AnthropicKey: key, Harness: harness, RepoURL: repoURL})
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

	type refHolder interface{ providerRef() string }
	var ref string
	switch v := sb.(type) {
	case *sandboxRecord:
		ref = v.ProviderRef
	case map[string]any:
		ref, _ = v["providerRef"].(string)
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

	if h, err := provider.ParseHandle(ref); err == nil {
		s.prov.Destroy(r.Context(), h)
	}
	w.WriteHeader(http.StatusNoContent)
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
	h, err := provider.ParseHandle(ref)
	if err != nil {
		http.Error(w, "invalid ref", http.StatusInternalServerError)
		return
	}
	s.proxyWebSocket(w, r,
		fmt.Sprintf("ws://sandbox-%s.%s.svc.cluster.local:8081/", h.Name, h.Namespace))
}

// tunnelProxy provides the WS tunnel endpoint for the CLI attach command (M5).
func (s *Server) tunnelProxy(w http.ResponseWriter, r *http.Request) {
	// Identical to terminalProxy: the CLI uses the same WS protocol.
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

	cn := websocket.NetConn(ctx, client, websocket.MessageBinary)
	pn := websocket.NetConn(ctx, pod, websocket.MessageBinary)

	// pod → client: when pod closes, cancel ctx so cn.Read unblocks below.
	go func() {
		buf := make([]byte, 32<<10)
		for {
			n, err := pn.Read(buf)
			if n > 0 {
				cn.Write(buf[:n])
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// client → pod
	buf := make([]byte, 32<<10)
	for {
		n, err := cn.Read(buf)
		if n > 0 {
			pn.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// lookupSandbox returns the sandbox record as any (DB row map or memStore record).
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

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
