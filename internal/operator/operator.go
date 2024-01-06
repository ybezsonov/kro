package operator

import "github.com/aws/symphony/internal/construct"

type _ interface {
	EnsureConstructCRD() error
	EnsureConstructResourceCRDs() error

	RegistrerClaimController() error
	DeregisterClaimController() error
}

type Operator struct {
	Name string
	CG   *construct.Graph
}
