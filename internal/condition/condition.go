package condition

import (
	v1alpha1 "github.com/aws/symphony/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCondition returns a new Condition instance.
func NewCondition(t v1alpha1.ConditionType, status corev1.ConditionStatus, reason, message string) v1alpha1.Condition {
	return v1alpha1.Condition{
		Type:               t,
		Status:             status,
		LastTransitionTime: &metav1.Time{Time: metav1.Now().Time},
		Reason:             &reason,
		Message:            &message,
	}
}

func NewTerminalCondition(status corev1.ConditionStatus, reason, message string) v1alpha1.Condition {
	return NewCondition(v1alpha1.ConditionTypeTerminal, status, reason, message)
}

func NewResourceSyncedCondition(status corev1.ConditionStatus, reason, message string) v1alpha1.Condition {
	return NewCondition(v1alpha1.ConditionTypeResourceSynced, status, reason, message)
}

func NewReconcilerReadyCondition(status corev1.ConditionStatus, reason, message string) v1alpha1.Condition {
	return NewCondition(v1alpha1.ConditionTypeReconcilerReady, status, reason, message)
}

func NewAdvisoryCondition(status corev1.ConditionStatus, reason, message string) v1alpha1.Condition {
	return NewCondition(v1alpha1.ConditionTypeAdvisory, status, reason, message)
}

func NewGraphSyncedCondition(status corev1.ConditionStatus, reason, message string) v1alpha1.Condition {
	return NewCondition(v1alpha1.ConditionTypeGraphSynced, status, reason, message)
}

func GetCondition(conditions []v1alpha1.Condition, t v1alpha1.ConditionType) *v1alpha1.Condition {
	for _, c := range conditions {
		if c.Type == t {
			return &c
		}
	}
	return nil
}

func SetCondition(conditions []v1alpha1.Condition, condition v1alpha1.Condition) []v1alpha1.Condition {
	for i, c := range conditions {
		if c.Type == condition.Type {
			conditions[i] = condition
			return conditions
		}
	}
	return append(conditions, condition)
}

func HasCondition(conditions []v1alpha1.Condition, t v1alpha1.ConditionType) bool {
	return GetCondition(conditions, t) != nil
}
