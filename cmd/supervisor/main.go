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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
	"github.com/coder/websocket"
	proto "github.com/sandlock/k8s-agent-platform/internal/supervisorproto"
)

const (
	controlAddr     = ":8080"
	terminalAddr    = ":8081"
	scrollbackBytes = 256 << 10 // 256 KB
)

// scrollback is a fixed-size ring buffer accumulating all PTY output.
type scrollback struct {
	mu  sync.Mutex
	buf []byte
	pos int // next write position
	n   int // total bytes ever written
}

func newScrollback(size int) *scrollback {
	return &scrollback{buf: make([]byte, size)}
}

func (s *scrollback) write(p []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range p {
		s.buf[s.pos] = b
		s.pos = (s.pos + 1) % len(s.buf)
		s.n++
	}
}

// snapshot returns all buffered bytes in chronological order.
func (s *scrollback) snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	size := len(s.buf)
	if s.n <= size {
		out := make([]byte, s.n)
		copy(out, s.buf[:s.n])
		return out
	}
	out := make([]byte, size)
	copy(out, s.buf[s.pos:])
	copy(out[size-s.pos:], s.buf[:s.pos])
	return out
}

// broadcaster fans PTY output out to all connected WebSocket clients.
type broadcaster struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{clients: make(map[chan []byte]struct{})}
}

func (b *broadcaster) subscribe() chan []byte {
	ch := make(chan []byte, 256)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broadcaster) unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

func (b *broadcaster) send(p []byte) {
	cp := make([]byte, len(p))
	copy(cp, p)
	b.mu.Lock()
	for ch := range b.clients {
		select {
		case ch <- cp:
		default: // slow client — drop rather than block PTY reader
		}
	}
	b.mu.Unlock()
}

type supervisor struct {
	claimed atomic.Bool
	ptmx    *os.File
	mu      sync.Mutex
	scroll  *scrollback
	bcast   *broadcaster
}

func main() {
	sup := &supervisor{
		scroll: newScrollback(scrollbackBytes),
		bcast:  newBroadcaster(),
	}
	go sup.serveTerminal()
	sup.serveControl()
}

func (s *supervisor) serveControl() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /claim", s.handleClaim)
	mux.HandleFunc("POST /stop", s.handleStop)
	mux.HandleFunc("GET /heartbeat", s.handleHeartbeat)
	mux.HandleFunc("GET /snapshot", s.handleSnapshot)

	log.Printf("control channel listening on %s", controlAddr)
	if err := http.ListenAndServe(controlAddr, mux); err != nil {
		log.Fatalf("control channel: %v", err)
	}
}

func (s *supervisor) handleClaim(w http.ResponseWriter, r *http.Request) {
	if s.claimed.Swap(true) {
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

// handleSnapshot streams a gzip+tar of /home/ubuntu/.claude/ back to the caller.
// Only available after the pod has been claimed. Returns 404 if the directory
// doesn't exist, 409 if the pod hasn't been claimed yet.
func (s *supervisor) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.claimed.Load() {
		http.Error(w, "not claimed", http.StatusConflict)
		return
	}
	cloneDir := "/home/ubuntu/.claude"
	if _, err := os.Stat(cloneDir); os.IsNotExist(err) {
		http.Error(w, "no claude session data", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	if err := tarGzDir(cloneDir, w); err != nil {
		log.Printf("snapshot: %v", err)
	}
}

// tarGzDir writes a gzip+tar of dir into w.
func tarGzDir(dir string, w io.Writer) error {
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(filepath.Dir(dir), path)
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// restoreSnapshot decompresses a gzip+tar snapshot into homeDir.
func restoreSnapshot(data []byte, homeDir string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") {
			continue
		}
		target := filepath.Join(homeDir, clean)
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, fs.FileMode(hdr.Mode)|0700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(hdr.Mode)|0600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return nil
}

func (s *supervisor) launch(req proto.ClaimRequest) error {
	workDir := "/workspace"
	if err := os.MkdirAll(workDir, 0755); err != nil {
		log.Printf("warning: MkdirAll %s: %v", workDir, err)
	}

	if len(req.SessionSnapshot) > 0 {
		if err := restoreSnapshot(req.SessionSnapshot, "/home/ubuntu"); err != nil {
			log.Printf("warning: restore snapshot failed, continuing fresh: %v", err)
		} else {
			log.Printf("restored Claude Code session snapshot (%d bytes)", len(req.SessionSnapshot))
		}
	}

	if req.RepoURL != "" {
		var clone *exec.Cmd
		if strings.Contains(req.RepoURL, "github.com") && req.GitHubToken != "" {
			clone = exec.Command("gh", "repo", "clone", req.RepoURL, workDir, "--", "--depth=1")
			clone.Env = append(os.Environ(), "GITHUB_TOKEN="+req.GitHubToken)
		} else {
			clone = exec.Command("git", "clone", "--depth=1", req.RepoURL, workDir)
		}
		clone.Stdout = os.Stdout
		clone.Stderr = os.Stderr
		if err := clone.Run(); err != nil {
			log.Printf("clone %s: %v (continuing without repo)", req.RepoURL, err)
		}
	}

	harness := harnessCmd(req, workDir)
	harness.Env = append(os.Environ(),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/home/ubuntu",
	)
	if req.AnthropicKey != "" {
		harness.Env = append(harness.Env, "ANTHROPIC_API_KEY="+req.AnthropicKey)
	}
	if req.GitHubToken != "" {
		harness.Env = append(harness.Env, "GITHUB_TOKEN="+req.GitHubToken)
	}

	ptmx, err := pty.Start(harness)
	if err != nil {
		return fmt.Errorf("pty start: %w", err)
	}

	s.mu.Lock()
	s.ptmx = ptmx
	s.mu.Unlock()

	log.Printf("harness %q started (pid %d)", req.Harness, harness.Process.Pid)

	// Single PTY reader: tee into scrollback and broadcast to all WS clients.
	go func() {
		buf := make([]byte, 32<<10)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				s.scroll.write(buf[:n])
				s.bcast.send(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	err = harness.Wait()
	log.Printf("harness exited: %v", err)
	os.Exit(0)
	return nil
}

func harnessCmd(req proto.ClaimRequest, workDir string) *exec.Cmd {
	var cmd *exec.Cmd
	switch req.Harness {
	case "claude-code":
		base := "claude --dangerously-skip-permissions"
		if len(req.SessionSnapshot) > 0 {
			base += " --continue"
		}
		cmd = exec.Command("/bin/sh", "-c", base)
	default:
		cmd = exec.Command("/bin/sh", "-c", req.Harness)
	}
	cmd.Dir = workDir
	return cmd
}

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
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("ws accept: %v", err)
		return
	}
	defer conn.CloseNow()

	// Wait up to 10s for the PTY to be ready.
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

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Subscribe before replaying so we don't miss live output between the two.
	ch := s.bcast.subscribe()
	defer s.bcast.unsubscribe(ch)

	// Wait for the client's first resize message before sending scrollback.
	// This ensures the PTY is resized to the client's actual dimensions before
	// the replayed bytes are rendered, so lines wrap correctly.
	{
		_, msg, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var size struct {
			Rows uint16 `json:"rows"`
			Cols uint16 `json:"cols"`
		}
		if json.Unmarshal(msg, &size) == nil && size.Rows > 0 && size.Cols > 0 {
			pty.Setsize(ptmx, &pty.Winsize{Rows: size.Rows, Cols: size.Cols}) //nolint:errcheck
		}
	}

	// Replay everything the PTY produced before this connection.
	if snap := s.scroll.snapshot(); len(snap) > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, snap); err != nil {
			return
		}
	}

	// Forward live PTY output to client.
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case p, ok := <-ch:
				if !ok {
					return
				}
				if err := conn.Write(ctx, websocket.MessageBinary, p); err != nil {
					return
				}
			}
		}
	}()

	// WebSocket → PTY: binary = keystrokes, text = resize JSON.
	for {
		mt, msg, err := conn.Read(ctx)
		if err != nil {
			return
		}
		switch mt {
		case websocket.MessageBinary:
			ptmx.Write(msg) //nolint:errcheck
		case websocket.MessageText:
			var size struct {
				Rows uint16 `json:"rows"`
				Cols uint16 `json:"cols"`
			}
			if json.Unmarshal(msg, &size) == nil && size.Rows > 0 && size.Cols > 0 {
				pty.Setsize(ptmx, &pty.Winsize{Rows: size.Rows, Cols: size.Cols}) //nolint:errcheck
			}
		}
	}
}