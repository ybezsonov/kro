package workflow

import (
	"context"
	"sync"

	"github.com/aws/symphony/internal/graph"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// This package transforms the Resource Graph into a Workflow matrix.
// Now we need to know how to deploy, update and delete the resources.

func New(
	name string,
	owner metav1.OwnerReference,
	target schema.GroupVersionResource,
	c *graph.Collection,
	client dynamic.NamespaceableResourceInterface,
) *Operator {
	return &Operator{
		name:       name,
		owner:      owner,
		target:     target,
		Collection: c,
		client:     client,
	}
}

type Operator struct {
	defaultNamestring string
	name              string
	owner             metav1.OwnerReference
	target            schema.GroupVersionResource

	Collection *graph.Collection
	client     dynamic.NamespaceableResourceInterface

	mu sync.Mutex

	ReadOne func(namespacedName string) (*unstructured.Unstructured, error)
	Delta   func() error

	CreateSteps [][]*Step
	UpdateSteps [][]*Step
	DeleteSteps [][]*Step
}

type StepType string

const (
	StepTypeKubernetesCreate StepType = "KubernetesCreate"
	StepTypeKubernetesUpdate StepType = "KubernetesUpdate"
	StepTypeKubernetesDelete StepType = "KubernetesDelete"
	StepTypeKubernetesRead   StepType = "KubernetesRead"
	StepTypeKubernetesDelta  StepType = "KubernetesDelta"
	StepTypeResourceReplace  StepType = "ResourceReplace"
)

type Step struct {
	// The name of the resource that this step is operating on.
	ResourceName string
	// The name of the step.
	Name string
	// Type is the type of the step.
	Type StepType
	// Action is the action that this step is performing.
	Action func(*graph.Collection) error
}

func (o *Operator) Create() error {
	for _, step := range o.CreateSteps {
		for _, s := range step {
			err := s.Action(o.Collection)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *Operator) Update() error {
	for _, step := range o.UpdateSteps {
		for _, s := range step {
			err := s.Action(o.Collection)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *Operator) Delete() error {
	for _, step := range o.DeleteSteps {
		for _, s := range step {
			err := s.Action(o.Collection)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *Operator) Build(collection *graph.Collection) error {
	o.Collection = collection
	o.CreateSteps = o.buildCreateSteps()
	o.UpdateSteps = o.buildUpdateSteps()
	o.DeleteSteps = o.buildDeleteSteps()
	return nil
}

func (o *Operator) buildCreateSteps() [][]*Step {
	// first we need to find the root resources.
	// root resources are resources that are not referenced by any other resource.
	// Then we need to find the resources that depend on the root resources.
	// Then we need to find the resources that depend on the resources that depend on the root resources.
	// And so on.

	steps := make([][]*Step, 0)

	rootResources := make([]*graph.Resource, 0)

	for _, resource := range o.Collection.Resources {
		if len(resource.DependsOn) == 0 {
			// this is a root resource.
			rootResources = append(rootResources, resource)
		}
	}

	queuedResouces := map[string]bool{}
	rootSteps := make([]*Step, 0)
	for _, resource := range rootResources {
		rootSteps = append(rootSteps, &Step{
			ResourceName: resource.Name,
			Name:         "Create resource " + resource.Name,
			Action: func(collection *graph.Collection) error {
				unstructr := resource.Unstructured()

				// Create the resource
				_, err := o.client.Namespace(unstructr.GetNamespace()).Apply(
					context.Background(), unstructr.GetName(), &unstructr, metav1.ApplyOptions{},
				)
				return err
			},
		})
	}

	steps = append(steps, rootSteps)

	nextdDepth := 1
	for len(queuedResouces) != len(o.Collection.Resources) {
		for _, resource := range o.Collection.Resources {
			nextdDepth++
			_ = resource
		}
	}

	return nil
}

func (o *Operator) buildUpdateSteps() [][]*Step {
	return nil
}

func (o *Operator) buildDeleteSteps() [][]*Step {
	return nil
}
