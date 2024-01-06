package workflow

import "github.com/aws/symphony/internal/construct"

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
	//
	// Action is the action that this step is performing.
	Action func(*construct.Graph) error
}
