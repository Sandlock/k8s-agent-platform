package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestServer returns a Server with no DB, no k8s client (dev mode).
// Handlers that don't reach the provider or pool manager are safe to test this way.
func newTestServer() *Server {
	return NewServer(nil, nil, nil, nil)
}

func TestRegisterNotImplementedWithoutDB(t *testing.T) {
	srv := newTestServer()
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "s3cr3t"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("register without DB: got %d, want 501", w.Code)
	}
}

func TestLoginNotImplementedWithoutDB(t *testing.T) {
	srv := newTestServer()
	body, _ := json.Marshal(map[string]string{"username": "alice", "password": "s3cr3t"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("login without DB: got %d, want 501", w.Code)
	}
}

func TestListSandboxesEmptyDevMode(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/sandboxes", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("list sandboxes dev mode: got %d, want 200", w.Code)
	}
	var result []interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty list, got %d items", len(result))
	}
}

func TestCreateSandboxRequiresKey(t *testing.T) {
	srv := newTestServer()
	body, _ := json.Marshal(map[string]string{"harness": "claude-code"})
	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("create without key: got %d, want 400", w.Code)
	}
}

func TestStopNonexistentSandbox(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodDelete, "/v1/sandboxes/does-not-exist", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("stop nonexistent: got %d, want 404", w.Code)
	}
}

func TestGetNonexistentSandbox(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/sandboxes/no-such-id", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("get nonexistent: got %d, want 404", w.Code)
	}
}

func TestLogoutNoOp(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("logout: got %d, want 204", w.Code)
	}
}

func TestStoreKeyRequiresDB(t *testing.T) {
	srv := newTestServer()
	body, _ := json.Marshal(map[string]string{"anthropicKey": "sk-ant-test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("store key without DB: got %d, want 501", w.Code)
	}
}
