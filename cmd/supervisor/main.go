/*
Copyright 2026 Sandlock Authors.

Use of this software is governed by the Business Source License 1.1 included
in the LICENSE file.
*/

// supervisor is the pod-side binary. It:
//   - Idles until a Claim message arrives on the control channel (:8080).
//   - On Claim: launches the agent harness with the API key in the child's
//     environment (never as argv, never written to disk).
//   - Bridges the harness PTY to WebSocket clients on the terminal port (:8081).
//   - Enforces single-use: rejects a second Claim with 409.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"sync/atomic"

	"github.com/creack/pty"
	"github.com/coder/websocket"
	proto "github.com/sandlock/k8s-agent-platform/internal/supervisorproto"
)

const (
	controlAddr  = ":8080"
	terminalAddr = ":8081"
)

type supervisor struct {
	claimed atomic.Bool // set to true after first Claim; rejects subsequent ones
	ptmx    *os.File    // master end of the PTY (set after Claim)
	mu      sync.Mutex
}

func main() {
	// Log the resolved claude path at startup for debugging.
	for _, p := range []string{"/usr/bin/claude", "/usr/local/bin/claude"} {
		if info, err := os.Stat(p); err == nil {
			log.Printf("found claude at %s (size=%d)", p, info.Size())
		}
	}

	sup := &supervisor{}

	go sup.serveTerminal()
	sup.serveControl()
}

// serveControl listens for JSON control messages from the control plane.
func (s *supervisor) serveControl() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /claim", s.handleClaim)
	mux.HandleFunc("POST /stop", s.handleStop)
	mux.HandleFunc("GET /heartbeat", s.handleHeartbeat)

	log.Printf("control channel listening on %s", controlAddr)
	if err := http.ListenAndServe(controlAddr, mux); err != nil {
		log.Fatalf("control channel: %v", err)
	}
}

func (s *supervisor) handleClaim(w http.ResponseWriter, r *http.Request) {
	if s.claimed.Swap(true) {
		// Single-use: reject second claim.
		http.Error(w, "already claimed", http.StatusConflict)
		return
	}

	var req proto.ClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.claimed.Store(false)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	go func() {
		if err := s.launch(req); err != nil {
			log.Printf("launch error: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proto.ClaimResponse{Status: "attached"})
}

func (s *supervisor) handleStop(w http.ResponseWriter, r *http.Request) {
	log.Println("stop received — exiting")
	w.WriteHeader(http.StatusNoContent)
	// Give the response time to flush before exiting.
	go func() { os.Exit(0) }()
}

func (s *supervisor) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	phase := "idle"
	if s.claimed.Load() {
		phase = "claimed"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proto.HeartbeatResponse{Phase: phase})
}

// launch optionally clones a repo, then starts the agent harness with a PTY.
func (s *supervisor) launch(req proto.ClaimRequest) error {
	workDir := "/workspace"
	os.MkdirAll(workDir, 0755)

	if req.RepoURL != "" {
		clone := exec.Command("git", "clone", "--depth=1", req.RepoURL, workDir)
		clone.Stdout = os.Stdout
		clone.Stderr = os.Stderr
		if err := clone.Run(); err != nil {
			log.Printf("clone %s: %v (continuing without repo)", req.RepoURL, err)
		}
	}

	harness := harnessCmd(req, workDir)
	// Key goes into child env only — never argv, never disk.
	harness.Env = os.Environ()
	if req.AnthropicKey != "" {
		harness.Env = append(harness.Env, "ANTHROPIC_API_KEY="+req.AnthropicKey)
	}

	ptmx, err := pty.Start(harness)
	if err != nil {
		return fmt.Errorf("pty start: %w", err)
	}

	s.mu.Lock()
	s.ptmx = ptmx
	s.mu.Unlock()

	log.Printf("harness %q started (pid %d)", req.Harness, harness.Process.Pid)
	err = harness.Wait()
	log.Printf("harness exited: %v — pod will terminate", err)

	// Die with the harness so the pod is single-use.
	os.Exit(0)
	return nil
}

// harnessCmd maps a harness name to an exec.Cmd running in workDir.
func harnessCmd(req proto.ClaimRequest, workDir string) *exec.Cmd {
	var cmd *exec.Cmd
	switch req.Harness {
	case "claude-code":
		// Search common install paths since the supervisor process may not
		// have a full PATH env (Kubernetes launches it directly, no shell).
		claudePath := ""
		for _, p := range []string{
			"/usr/bin/claude",
			"/usr/local/bin/claude",
			"/usr/lib/node_modules/.bin/claude",
			"/usr/local/lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe",
			"/usr/lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe",
		} {
			if _, err := os.Stat(p); err == nil {
				claudePath = p
				break
			}
		}
		if claudePath == "" {
			// Last resort: ask the shell to find it.
			out, err := exec.Command("bash", "-lc", "which claude").Output()
			if err == nil {
				claudePath = strings.TrimSpace(string(out))
			}
		}
		if claudePath == "" {
			claudePath = "/usr/bin/claude"
		}
		cmd = exec.Command(claudePath, "--dangerously-skip-permissions")
	default:
		cmd = exec.Command(req.Harness)
	}
	cmd.Dir = workDir
	return cmd
}

// serveTerminal accepts WebSocket connections and bridges them to the harness PTY.
func (s *supervisor) serveTerminal() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleTerminalWS)

	log.Printf("terminal bridge listening on %s", terminalAddr)
	if err := http.ListenAndServe(terminalAddr, mux); err != nil {
		log.Fatalf("terminal bridge: %v", err)
	}
}

func (s *supervisor) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // internal cluster traffic only
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}
	defer conn.CloseNow()

	// Wait up to 10s for the PTY to be ready (claim starts it in a goroutine).
	var ptmx *os.File
	for i := 0; i < 100; i++ {
		s.mu.Lock()
		ptmx = s.ptmx
		s.mu.Unlock()
		if ptmx != nil {
			break
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	if ptmx == nil {
		conn.Close(websocket.StatusNormalClosure, "not yet claimed")
		return
	}

	// Bridge: WebSocket ↔ PTY using a net.Conn adapter for clean bidirectional copy.
	netConn := websocket.NetConn(r.Context(), conn, websocket.MessageBinary)
	go func() { io.Copy(ptmx, netConn) }()
	io.Copy(netConn, ptmx)
}
