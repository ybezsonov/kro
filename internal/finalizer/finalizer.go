package finalizer

import (
	"github.com/aws/symphony/api/v1alpha1"
)

const (
	// Symphony construct finalizer
	SymphonyFinalizer = "symphony.io/finalizer"
)

func IsSymphonyManaged(c *v1alpha1.Construct) bool {
	return (&Manager{finalizers: []string{SymphonyFinalizer}}).HaveAllFinalizers(c.Finalizers)
}

func AddSymphonyFinalizer(c *v1alpha1.Construct) []string {
	return (&Manager{finalizers: []string{SymphonyFinalizer}}).AddFinalizers(c.Finalizers)
}

func RemoveSymphonyFinalizer(c *v1alpha1.Construct) []string {
	return (&Manager{finalizers: []string{SymphonyFinalizer}}).RemoveFinalizers(c.Finalizers)
}

func removeString(slice []string, s string) []string {
	for i, v := range slice {
		if v == s {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func containsFinalizer(fs []string, target string) bool {
	for _, f := range fs {
		if f == target {
			return true
		}
	}
	return false
}

func New(finalizers ...string) *Manager {
	return &Manager{
		finalizers: finalizers,
	}
}

type Manager struct {
	finalizers []string
}

func (m *Manager) RemoveFinalizers(strs []string) []string {
	for _, f := range m.finalizers {
		strs = removeString(strs, f)
	}
	return strs
}

func (m *Manager) AddFinalizers(strs []string) []string {
	for _, f := range m.finalizers {
		if !containsFinalizer(strs, f) {
			strs = append(strs, f)
		}
	}
	return strs
}

func (m *Manager) HaveAtLeastOneFinalizer(strs []string) bool {
	for _, f := range m.finalizers {
		if containsFinalizer(strs, f) {
			return true
		}
	}
	return false
}

func (m *Manager) HaveAllFinalizers(strs []string) bool {
	for _, f := range m.finalizers {
		if !containsFinalizer(strs, f) {
			return false
		}
	}
	return true
}
