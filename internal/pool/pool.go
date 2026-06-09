// Package pool manages the warm sandbox pool.
// It runs as a background goroutine in the control plane.
package pool

import (
	"context"
	"log"
	"time"

	sandlockv1alpha1 "github.com/sandlock/k8s-agent-platform/api/v1alpha1"
	"github.com/sandlock/k8s-agent-platform/internal/provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Manager keeps the warm pool topped up.
type Manager struct {
	k8s      client.Client
	prov     provider.Provider
	interval time.Duration
}

// New creates a pool manager. Call Run to start it.
func New(k8s client.Client, prov provider.Provider) *Manager {
	return &Manager{
		k8s:      k8s,
		prov:     prov,
		interval: 15 * time.Second,
	}
}

// Run reconciles the pool every interval until ctx is cancelled.
func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reconcile(ctx)
		}
	}
}

func (m *Manager) reconcile(ctx context.Context) {
	var pools sandlockv1alpha1.SandboxPoolList
	if err := m.k8s.List(ctx, &pools); err != nil {
		log.Printf("pool: list SandboxPools: %v", err)
		return
	}

	for _, pool := range pools.Items {
		m.reconcilePool(ctx, pool)
	}
}

func (m *Manager) reconcilePool(ctx context.Context, pool sandlockv1alpha1.SandboxPool) {
	ns := pool.Namespace

	var sandboxes sandlockv1alpha1.SandboxList
	if err := m.k8s.List(ctx, &sandboxes, client.InNamespace(ns)); err != nil {
		log.Printf("pool: list sandboxes in %s: %v", ns, err)
		return
	}

	var ready, total int32
	for _, sb := range sandboxes.Items {
		if sb.Spec.Pool {
			total++
			if sb.Status.Phase == sandlockv1alpha1.PhaseReady {
				ready++
			}
		}
	}

	need := pool.Spec.TargetReady - ready
	if need <= 0 || total >= pool.Spec.MaxTotal {
		return
	}

	for i := int32(0); i < need; i++ {
		if total+i >= pool.Spec.MaxTotal {
			break
		}
		_, err := m.prov.Provision(ctx, provider.SandboxSpec{
			Harness:            pool.Spec.Harness,
			CPULimit:           "1",
			MemoryLimit:        "2Gi",
			TTLSeconds:         pool.Spec.PodTTLSeconds,
			IdleTimeoutSeconds: 900,
			Namespace:          ns,
			Pool:               true,
		})
		if err != nil {
			log.Printf("pool: provision warm sandbox: %v", err)
		}
	}
}

// WarmOne provisions a single replacement warm sandbox (called after a claim).
func (m *Manager) WarmOne(ctx context.Context, ns, harness string) {
	_, err := m.prov.Provision(ctx, provider.SandboxSpec{
		Harness:            harness,
		CPULimit:           "1",
		MemoryLimit:        "2Gi",
		TTLSeconds:         3600,
		IdleTimeoutSeconds: 900,
		Namespace:          ns,
		Pool:               true,
	})
	if err != nil {
		log.Printf("pool: warm replacement: %v", err)
	}
}
