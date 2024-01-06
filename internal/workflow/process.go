package workflow

type ProcessState string

const (
	ProcessStateRunning   ProcessState = "InProgress"
	ProcessStateCompleted ProcessState = "Completed"
	ProcessStateFailed    ProcessState = "Failed"
)

type Process struct {
	Steps         []Step
	ProgressIndex int
	State         ProcessState
}
