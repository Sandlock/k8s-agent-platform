// Package supervisorproto defines the JSON messages exchanged on the supervisor
// control channel (control-plane → supervisor, port 8080).
package supervisorproto

// Skill represents a user-defined Claude Code command file. The supervisor
// writes each skill to ~/.claude/commands/<Name>.md before starting the harness.
type Skill struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// ClaimRequest is sent by the control plane to start an agent session.
// Keys MUST travel only over this in-memory channel and MUST NOT be
// stored in any Kubernetes resource, log, or persistent store.
type ClaimRequest struct {
	AnthropicKey    string `json:"anthropicKey"`
	Harness         string `json:"harness"`
	RepoURL         string `json:"repoUrl,omitempty"`
	Branch          string `json:"branch,omitempty"`
	GitHubToken     string `json:"githubToken,omitempty"`
	// SessionSnapshot is a gzip+tar of ~/.claude/ from a prior sandbox.
	// When non-nil the supervisor restores it before launching the harness
	// so Claude Code can resume the previous session via --continue.
	SessionSnapshot []byte `json:"sessionSnapshot,omitempty"`
	// CallbackURL is the control-plane endpoint the supervisor POSTs a
	// gzip+tar of ~/.claude/ to on harness exit (push-on-exit snapshot).
	CallbackURL string `json:"callbackUrl,omitempty"`
	// Skills are user-defined command files written to ~/.claude/commands/
	// before the harness starts.
	Skills []Skill `json:"skills,omitempty"`
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
