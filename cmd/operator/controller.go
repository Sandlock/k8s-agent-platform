package main

import (
	"context"
	"fmt"

	sandlockv1alpha1 "github.com/sandlock/k8s-agent-platform/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SandboxReconciler reconciles Sandbox custom resources into pods + services.
//
// +kubebuilder:rbac:groups=sandlock.dev,resources=sandboxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sandlock.dev,resources=sandboxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;delete
type SandboxReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	SupervisorImage string
	RuntimeClass    string // optional gVisor / Kata runtime class (M6)
}

const (
	controlPort  = 8080
	terminalPort = 8081
)

func (r *SandboxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.Log.WithName("sandbox").WithValues("sandbox", req.NamespacedName)

	var sb sandlockv1alpha1.Sandbox
	if err := r.Get(ctx, req.NamespacedName, &sb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Ensure pod exists.
	pod, err := r.ensurePod(ctx, &sb)
	if err != nil {
		log.Error(err, "failed to ensure pod")
		return ctrl.Result{}, err
	}

	// Ensure per-sandbox Service exists so the control plane can reach the supervisor.
	if err := r.ensureService(ctx, &sb); err != nil {
		log.Error(err, "failed to ensure service")
		return ctrl.Result{}, err
	}

	// Sync status phase from pod phase.
	desired := r.phaseFromPod(pod)
	if sb.Status.Phase != desired || sb.Status.PodName != pod.Name {
		sb.Status.Phase = desired
		sb.Status.PodName = pod.Name
		if err := r.Status().Update(ctx, &sb); err != nil && !errors.IsConflict(err) {
			return ctrl.Result{}, err
		}
	}

	// Requeue while pod is still starting.
	if desired == sandlockv1alpha1.PhaseWarming {
		return ctrl.Result{RequeueAfter: 2e9}, nil // 2s
	}

	// Session ended — delete the Sandbox CR. The pod and service are GC'd via owner references.
	if desired == sandlockv1alpha1.PhaseRecycling || desired == sandlockv1alpha1.PhaseFailed {
		log.Info("session ended, deleting sandbox", "phase", desired)
		if err := r.Delete(ctx, &sb); err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *SandboxReconciler) ensurePod(ctx context.Context, sb *sandlockv1alpha1.Sandbox) (*corev1.Pod, error) {
	pod := r.buildPod(sb)

	if err := controllerutil.SetControllerReference(sb, pod, r.Scheme); err != nil {
		return nil, err
	}

	existing := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKeyFromObject(pod), existing)
	if err == nil {
		return existing, nil
	}
	if !errors.IsNotFound(err) {
		return nil, err
	}

	if err := r.Create(ctx, pod); err != nil {
		return nil, fmt.Errorf("create pod: %w", err)
	}
	return pod, nil
}

func (r *SandboxReconciler) ensureService(ctx context.Context, sb *sandlockv1alpha1.Sandbox) error {
	svc := r.buildService(sb)
	if err := controllerutil.SetControllerReference(sb, svc, r.Scheme); err != nil {
		return err
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKeyFromObject(svc), existing)
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	return r.Create(ctx, svc)
}


func (r *SandboxReconciler) buildPod(sb *sandlockv1alpha1.Sandbox) *corev1.Pod {
	limits := corev1.ResourceList{}
	if !sb.Spec.Resources.CPU.IsZero() {
		limits[corev1.ResourceCPU] = sb.Spec.Resources.CPU
	}
	if !sb.Spec.Resources.Memory.IsZero() {
		limits[corev1.ResourceMemory] = sb.Spec.Resources.Memory
	}
	if len(limits) == 0 {
		limits = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		}
	}

	uid := int64(1000)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sb.Name,
			Namespace: sb.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "sandlock-operator",
				"sandlock.dev/sandbox":         sb.Name,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    &uid,
				RunAsNonRoot: func(b bool) *bool { return &b }(true),
			},
			Containers: []corev1.Container{
				{
					Name:  "supervisor",
					Image: r.SupervisorImage,
					Ports: []corev1.ContainerPort{
						{Name: "control", ContainerPort: controlPort, Protocol: corev1.ProtocolTCP},
						{Name: "terminal", ContainerPort: terminalPort, Protocol: corev1.ProtocolTCP},
					},
					Resources: corev1.ResourceRequirements{
						Limits:   limits,
						Requests: limits,
					},
					Env: []corev1.EnvVar{
						{Name: "SANDBOX_NAME", Value: sb.Name},
						{Name: "SANDBOX_NAMESPACE", Value: sb.Namespace},
					},
				},
			},
		},
	}
	if r.RuntimeClass != "" {
		pod.Spec.RuntimeClassName = &r.RuntimeClass
	}
	return pod
}

func (r *SandboxReconciler) buildService(sb *sandlockv1alpha1.Sandbox) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sandbox-" + sb.Name,
			Namespace: sb.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"sandlock.dev/sandbox": sb.Name,
			},
			Ports: []corev1.ServicePort{
				{Name: "control", Port: controlPort, TargetPort: intstr.FromInt32(controlPort)},
				{Name: "terminal", Port: terminalPort, TargetPort: intstr.FromInt32(terminalPort)},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

func (r *SandboxReconciler) phaseFromPod(pod *corev1.Pod) sandlockv1alpha1.SandboxPhase {
	switch pod.Status.Phase {
	case corev1.PodRunning:
		return sandlockv1alpha1.PhaseReady
	case corev1.PodFailed:
		return sandlockv1alpha1.PhaseFailed
	case corev1.PodSucceeded:
		return sandlockv1alpha1.PhaseRecycling
	default:
		return sandlockv1alpha1.PhaseWarming
	}
}

func (r *SandboxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sandlockv1alpha1.Sandbox{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
