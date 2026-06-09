// Package provider defines the pluggable backend interface for sandbox provisioning.
// Only the Kubernetes implementation ships for MVP; other backends (Firecracker,
// Fly Machines, Fargate) can be added by implementing these four methods plus
// shipping the supervisor in a base image.
package provider

import "context"

// Phase mirrors the Sandbox CR status phase.
type Phase string

const (
	PhaseWarming   Phase = "Warming"
	PhaseReady     Phase = "Ready"
	PhaseClaimed   Phase = "Claimed"
	PhaseRecycling Phase = "Recycling"
	PhaseFailed    Phase = "Failed"
	PhaseGone      Phase = "Gone"
)

// SandboxSpec describes what to provision.
type SandboxSpec struct {
	Harness            string
	CPULimit           string // e.g. "1"
	MemoryLimit        string // e.g. "2Gi"
	TTLSeconds         int64
	IdleTimeoutSeconds int64
	Namespace          string // target namespace; defaults to "sandboxes"
	Pool               bool   // true = warm pool pod (not yet claimed)
}

// Handle is an opaque reference to a provisioned sandbox.
// For Kubernetes this is namespace + CR name.
type Handle struct {
	Namespace string
	Name      string
}

// String returns a provider_ref suitable for DB storage.
func (h Handle) String() string {
	return h.Namespace + "/" + h.Name
}

// Provider is the seam between the control plane and a backend.
// Implement only these four methods to add a new backend.
type Provider interface {
	// Provision creates an unclaimed sandbox (pool warm-up or cold provision).
	Provision(ctx context.Context, spec SandboxSpec) (Handle, error)

	// Status returns the current phase of a sandbox.
	Status(ctx context.Context, h Handle) (Phase, error)

	// Destroy stops and deletes a sandbox irreversibly.
	Destroy(ctx context.Context, h Handle) error

	// List returns all live sandboxes managed by this provider.
	List(ctx context.Context) ([]Handle, error)
}
