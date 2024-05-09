package operator

import "github.com/aws/symphony/internal/resourcegroup"

type _ interface {
	EnsureResourceGroupCRD() error
	EnsureResourceGroupResourceCRDs() error

	RegistrerClaimController() error
	DeregisterClaimController() error
}

type Operator struct {
	Name string
	CG   *resourcegroup.Graph
}
