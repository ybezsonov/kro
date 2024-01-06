package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"github.com/wzshiming/easycel"
)

const (
	claimToken     = "claim"
	resourcesToken = "resources"
)

type SymphonyEngine struct {
	claim     map[string]interface{}
	resources map[string]map[string]interface{}
	registry  *easycel.Registry
	env       *easycel.Environment
}

func NewEngine() (*SymphonyEngine, error) {
	registry := easycel.NewRegistry("symphony-cel-engine", easycel.WithTagName("json"))

	// instantly register the two variables we know we'll need
	err := registry.RegisterVariable(claimToken, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	err = registry.RegisterVariable(resourcesToken, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	env, err := easycel.NewEnvironment(cel.Lib(registry))
	if err != nil {
		return nil, err
	}

	resources := make(map[string]map[string]interface{})

	return &SymphonyEngine{
		resources: resources,
		registry:  registry,
		env:       env,
	}, nil
}

func (se *SymphonyEngine) SetClaim(claim map[string]interface{}) {
	se.claim = claim
}

func (se *SymphonyEngine) SetResource(name string, resource map[string]interface{}) {
	se.resources[name] = resource
}

func (se *SymphonyEngine) EvalClaim(expression string) (ref.Val, error) {
	prog, err := se.env.Program(claimToken + "." + expression)
	if err != nil {
		return nil, err
	}
	val, _, err := prog.Eval(map[string]interface{}{
		claimToken: se.claim,
	})
	if err != nil {
		return nil, err
	}

	return val, nil
}

// TODO Add a paramter to specify the resource name
func (se *SymphonyEngine) EvalResource(expression string) (ref.Val, error) {
	prog, err := se.env.Program("resources" + "." + expression)
	if err != nil {
		return nil, err
	}
	val, _, err := prog.Eval(map[string]interface{}{
		resourcesToken: se.resources,
	})
	if err != nil {
		return nil, err
	}

	return val, nil
}
