/*
Copyright 2026 Sandlock Authors.

Use of this software is governed by the Business Source License 1.1 included
in the LICENSE file.
*/

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SandboxPhase describes the lifecycle phase of a Sandbox.
type SandboxPhase string

const (
	PhaseWarming  SandboxPhase = "Warming"
	PhaseReady    SandboxPhase = "Ready"
	PhaseClaimed  SandboxPhase = "Claimed"
	PhaseRecycling SandboxPhase = "Recycling"
	PhaseFailed   SandboxPhase = "Failed"
)

// SandboxResourceRequirements mirrors core/v1 ResourceList for CRD embedding.
type SandboxResourceRequirements struct {
	// +optional
	CPU resource.Quantity `json:"cpu,omitempty"`
	// +optional
	Memory resource.Quantity `json:"memory,omitempty"`
}

// SandboxSpec defines the desired state of a Sandbox.
type SandboxSpec struct {
	// Harness identifies which agent harness to run. Currently "claude-code".
	// +kubebuilder:default=claude-code
	Harness string `json:"harness"`

	// Pool marks this Sandbox as part of the warm pool (generic, unclaimed).
	// +optional
	Pool bool `json:"pool,omitempty"`

	// Resources sets CPU and memory limits for the sandbox pod.
	// +optional
	Resources SandboxResourceRequirements `json:"resources,omitempty"`

	// TTLSeconds is the maximum lifetime of the pod.
	// +optional
	// +kubebuilder:default=3600
	TTLSeconds int64 `json:"ttlSeconds,omitempty"`

	// IdleTimeoutSeconds destroys the pod if no heartbeat is received.
	// +optional
	// +kubebuilder:default=900
	IdleTimeoutSeconds int64 `json:"idleTimeoutSeconds,omitempty"`
}

// SandboxStatus reflects the observed state of a Sandbox.
type SandboxStatus struct {
	// Phase is the current lifecycle phase.
	// +optional
	Phase SandboxPhase `json:"phase,omitempty"`

	// PodName is the name of the backing pod.
	// +optional
	PodName string `json:"podName,omitempty"`

	// ClaimedBy holds the user ID that claimed this Sandbox. Never the API key.
	// +optional
	ClaimedBy string `json:"claimedBy,omitempty"`

	// LastHeartbeat is the last time the supervisor reported in.
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// Message holds a human-readable status message.
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=sb
// +kubebuilder:printcolumn:name="Harness",type=string,JSONPath=`.spec.harness`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Pod",type=string,JSONPath=`.status.podName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Sandbox represents one agent pod, warm or claimed.
type Sandbox struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SandboxSpec   `json:"spec,omitempty"`
	Status SandboxStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SandboxList is a list of Sandbox resources.
type SandboxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sandbox `json:"items"`
}

// SandboxPoolSpec defines the desired state of a SandboxPool.
type SandboxPoolSpec struct {
	// Harness identifies which agent harness this pool is for.
	// +kubebuilder:default=claude-code
	Harness string `json:"harness"`

	// TargetReady is the number of Ready pods the operator should maintain.
	// +kubebuilder:default=0
	TargetReady int32 `json:"targetReady"`

	// MaxTotal caps the total number of live sandbox pods.
	// +kubebuilder:default=50
	MaxTotal int32 `json:"maxTotal"`

	// PodTTLSeconds is the maximum lifetime for warm pods.
	// +kubebuilder:default=3600
	PodTTLSeconds int64 `json:"podTtlSeconds"`
}

// SandboxPoolStatus reflects observed pool state.
type SandboxPoolStatus struct {
	// Ready is the current count of Ready sandboxes.
	Ready int32 `json:"ready"`
	// Total is the total count of live sandboxes.
	Total int32 `json:"total"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=sbpool
// +kubebuilder:printcolumn:name="Harness",type=string,JSONPath=`.spec.harness`
// +kubebuilder:printcolumn:name="TargetReady",type=integer,JSONPath=`.spec.targetReady`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.ready`

// SandboxPool controls how many warm Sandboxes the operator maintains.
type SandboxPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SandboxPoolSpec   `json:"spec,omitempty"`
	Status SandboxPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SandboxPoolList is a list of SandboxPool resources.
type SandboxPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SandboxPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Sandbox{}, &SandboxList{})
	SchemeBuilder.Register(&SandboxPool{}, &SandboxPoolList{})
}
