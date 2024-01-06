package workflow

import (
	"context"
	"fmt"

	"github.com/aws/symphony/internal/construct"
	"github.com/aws/symphony/internal/requeue"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
)

// This package transforms the Resource Graph into a Workflow matrix.
// Now we need to know how to deploy, update and delete the resources.

func NewOperator(
	target schema.GroupVersionResource,
	g *construct.Graph,
	client dynamic.NamespaceableResourceInterface,
) *Operator {
	return &Operator{
		target:       target,
		Graph:        g,
		client:       client,
		stateTracker: construct.NewStateTracker(g),
	}
}

type Operator struct {
	target schema.GroupVersionResource

	client dynamic.NamespaceableResourceInterface

	Graph *construct.Graph

	stateTracker *construct.StateTracker

	CreateProcess []*Process
	// maybe UpdateProcess []*Process
	DeleteProcess []*Process
}

func (o *Operator) Handler(ctx context.Context, req ctrl.Request) error {
	// extract claim from request
	claimUnstructured, err := o.client.Get(ctx, req.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	o.Graph.Claim = construct.Claim{Unstructured: claimUnstructured}
	err = o.Graph.TopologicalSort()
	if err != nil {
		return err
	}
	err = o.Graph.ResolvedVariables()
	if err != nil {
		return err
	}
	err = o.Graph.ReplaceVariables()
	if err != nil {
		return err
	}

	for _, resource := range o.Graph.Resources {
		if !o.stateTracker.ResourceDependenciesReady(resource.RuntimeID) {
			return requeue.NeededAfter(fmt.Errorf("resource dependencies not ready"), 5)
		}

		// Check if resource exists
		observed, err := o.client.Get(ctx, resource.Metadata().Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err := o.client.Create(ctx, resource.Unstructured(), metav1.CreateOptions{})
				if err != nil {
					return err
				}
				o.stateTracker.SetState(resource.RuntimeID, construct.ResourceStateCreating)
			} else {
				return err
			}
		}
		if observed != nil {
			observedStatus, ok := observed.Object["status"]
			if ok {
				err := resource.SetStatus(observedStatus.(map[string]interface{}))
				if err != nil {
					return err
				}
				o.stateTracker.SetState(resource.RuntimeID, construct.ResourceStateReady)
				// ...
				err = o.Graph.ResolvedVariables()
				if err != nil {
					return err
				}
				err = o.Graph.ReplaceVariables()
				if err != nil {
					return err
				}
			}
		}
	}
	if !o.stateTracker.AllReady() {
		return requeue.NeededAfter(fmt.Errorf("not all resources are ready"), 5)
	}

	return nil
}
