package workflow

import "github.com/aws/symphony/internal/resourcegroup"

type StepType string

const (
	StepTypeRead    StepType = "Read"
	StepTypeCreate  StepType = "Create"
	StepTypeUpdate  StepType = "Update"
	StepTypeDelete  StepType = "Delete"
	StepTypeCompare StepType = "Compare"
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
	Action func(*resourcegroup.Graph) error
}
