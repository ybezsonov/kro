package resourcegroup

import (
	"fmt"
	"sort"
)

type ResourceState string

const (
	ResourceStateUnknown ResourceState = "unknown"
	// Waiting for a dependency to be resolved/created
	ResourceStatePending ResourceState = "pending"
	// Resource is being created
	ResourceStateCreating ResourceState = "creating"
	// Resource is being updated
	ResourceStateUpdating ResourceState = "updating"
	// Resource is being deleted
	ResourceStateDeleting ResourceState = "deleting"
	// Resource is being replaced
	ResourceStateReplacing ResourceState = "replacing"
	// Resource is ready
	ResourceStateReady ResourceState = "ready"
)

type ResourceStateTracker struct {
	*Resource

	State ResourceState
}

type StateTracker struct {
	ResourceStates map[string]*ResourceStateTracker
}

func NewStateTracker(g *Graph) *StateTracker {
	rs := make(map[string]*ResourceStateTracker)
	for _, resource := range g.Resources {
		rs[resource.RuntimeID] = &ResourceStateTracker{
			Resource: resource,
			State:    ResourceStateUnknown,
		}
	}

	return &StateTracker{
		ResourceStates: rs,
	}
}

func (s *StateTracker) GetState(runtimeID string) ResourceState {
	if state, ok := s.ResourceStates[runtimeID]; ok {
		return state.State
	}
	return ResourceStateUnknown
}

func (s *StateTracker) SetState(runtimeID string, state ResourceState) {
	s.ResourceStates[runtimeID].State = state
}

func (s *StateTracker) ResourceDependenciesReady(runtimeID string) bool {
	resource := s.ResourceStates[runtimeID]
	for _, dependency := range resource.Dependencies {
		if s.GetState(dependency.RuntimeID) != ResourceStateReady {
			return false
		}
	}
	return true
}

func (s *StateTracker) AllReady() bool {
	for _, resource := range s.ResourceStates {
		if resource.State != ResourceStateReady {
			return false
		}
	}
	return true
}

// need to be ordered
func (s *StateTracker) String() {
	order := []string{}
	for _, resource := range s.ResourceStates {
		order = append(order, resource.RuntimeID)
	}
	sort.Strings(order)
	for _, runtimeID := range order {
		resource := s.ResourceStates[runtimeID]
		fmt.Println("  => resource: ", resource.RuntimeID)
		fmt.Println("      => state: ", resource.State)
		fmt.Println("      => dependencies: ")
		for _, dependency := range resource.Dependencies {
			fmt.Println("          => ", dependency.RuntimeID)
		}
	}
}
