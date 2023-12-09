package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionType string

const (
	// ConditionTypeResourceSynced indicates the state of the resource in the
	// in kubernetes cluster.
	ConditionTypeResourceSynced ConditionType = "symphony.aws.dev/ResourceSynced"
	// ConditionTypeReconcilerReady indicates the state of the reconciler.
	// Whenever an Abstraction resource is created, Symphony will spin up a
	// reconciler for that resource. This condition indicates the state of the
	// reconciler.
	ConditionTypeReconcilerReady ConditionType = "symphony.aws.dev/ReconcilerReady"
	// ConditionTypeTerminal indicates that the custom resource Spec need to be
	// updated before any further sync.
	ConditionTypeTerminal ConditionType = "symphony.aws.dev/Terminal"
	// ConditionTypeAdvisory indicates any advisory info that may be present in the resource.
	ConditionTypeAdvisory ConditionType = "symphony.aws.dev/Advisory"
)

// Condition is the common struct used by all CRDs managed by ACK service
// controllers to indicate terminal states  of the CR and its backend AWS
// service API resource
type Condition struct {
	// Type is the type of the Condition
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	// +optional
	Reason *string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	// +optional
	Message *string `json:"message,omitempty"`
	// observedGeneration represents the .metadata.generation that the condition was set based upon.
	// For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
	// with respect to the current state of the instance.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}
