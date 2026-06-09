// Package supervisorproto defines the JSON messages exchanged on the supervisor
// control channel (control-plane → supervisor, port 8080).
package supervisorproto

// ClaimRequest is sent by the control plane to start an agent session.
// The API key MUST travel only over this in-memory channel and MUST NOT be
// stored in any Kubernetes resource, log, or persistent store.
type ClaimRequest struct {
	AnthropicKey string `json:"anthropicKey"`
	Harness      string `json:"harness"`
	RepoURL      string `json:"repoUrl,omitempty"`
}

// ClaimResponse is returned by the supervisor after a successful claim.
type ClaimResponse struct {
	Status string `json:"status"` // "attached"
}

// StopRequest asks the supervisor to terminate the session.
type StopRequest struct{}

// HeartbeatResponse is returned by the supervisor's /heartbeat endpoint.
type HeartbeatResponse struct {
	Phase string `json:"phase"` // "idle" | "claimed"
}
