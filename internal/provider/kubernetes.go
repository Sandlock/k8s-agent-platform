package provider

import (
	"context"
	"fmt"
	"strings"

	sandlockv1alpha1 "github.com/sandlock/k8s-agent-platform/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultNamespace = "sandboxes"

// KubernetesProvider implements Provider via Sandbox custom resources.
// The operator watches these CRs and reconciles them into real pods.
type KubernetesProvider struct {
	client client.Client
}

// NewKubernetesProvider creates a provider backed by the given controller-runtime client.
func NewKubernetesProvider(c client.Client) *KubernetesProvider {
	return &KubernetesProvider{client: c}
}

// Provision creates a Sandbox CR. The operator will reconcile it into a pod.
func (p *KubernetesProvider) Provision(ctx context.Context, spec SandboxSpec) (Handle, error) {
	ns := spec.Namespace
	if ns == "" {
		ns = defaultNamespace
	}

	sb := &sandlockv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "sb-",
			Namespace:    ns,
		},
		Spec: sandlockv1alpha1.SandboxSpec{
			Harness:            spec.Harness,
			Pool:               spec.Pool,
			TTLSeconds:         spec.TTLSeconds,
			IdleTimeoutSeconds: spec.IdleTimeoutSeconds,
		},
	}

	if spec.CPULimit != "" {
		sb.Spec.Resources.CPU = resource.MustParse(spec.CPULimit)
	}
	if spec.MemoryLimit != "" {
		sb.Spec.Resources.Memory = resource.MustParse(spec.MemoryLimit)
	}

	if err := p.client.Create(ctx, sb); err != nil {
		return Handle{}, fmt.Errorf("create Sandbox CR: %w", err)
	}

	return Handle{Namespace: sb.Namespace, Name: sb.Name}, nil
}

// Status returns the current phase by reading the Sandbox CR status.
func (p *KubernetesProvider) Status(ctx context.Context, h Handle) (Phase, error) {
	var sb sandlockv1alpha1.Sandbox
	if err := p.client.Get(ctx, types.NamespacedName{Namespace: h.Namespace, Name: h.Name}, &sb); err != nil {
		return PhaseFailed, fmt.Errorf("get Sandbox CR: %w", err)
	}
	return Phase(sb.Status.Phase), nil
}

// Destroy deletes the Sandbox CR; the operator garbage-collects the pod via owner references.
func (p *KubernetesProvider) Destroy(ctx context.Context, h Handle) error {
	var sb sandlockv1alpha1.Sandbox
	if err := p.client.Get(ctx, types.NamespacedName{Namespace: h.Namespace, Name: h.Name}, &sb); err != nil {
		return client.IgnoreNotFound(err)
	}
	return p.client.Delete(ctx, &sb)
}

// List returns handles for all Sandbox CRs across all namespaces.
func (p *KubernetesProvider) List(ctx context.Context) ([]Handle, error) {
	var list sandlockv1alpha1.SandboxList
	if err := p.client.List(ctx, &list); err != nil {
		return nil, fmt.Errorf("list Sandbox CRs: %w", err)
	}
	handles := make([]Handle, len(list.Items))
	for i, sb := range list.Items {
		handles[i] = Handle{Namespace: sb.Namespace, Name: sb.Name}
	}
	return handles, nil
}

// ParseHandle reconstructs a Handle from a provider_ref string (namespace/name).
func ParseHandle(ref string) (Handle, error) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return Handle{}, fmt.Errorf("invalid provider_ref %q", ref)
	}
	return Handle{Namespace: parts[0], Name: parts[1]}, nil
}
