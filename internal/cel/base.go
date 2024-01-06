package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/wzshiming/easycel"
)

func NewBasicEngine() *BasicEngine {
	return &BasicEngine{}
}

type BasicEngine struct{}

func (e *BasicEngine) Eval(expression string, object map[string]interface{}) (string, error) {
	registry := easycel.NewRegistry("symphony-cel-engine", easycel.WithTagName("json"))

	//TODO: Figure out a better way to do this
	registry.RegisterVariable("root", map[string]interface{}{})

	env, err := easycel.NewEnvironment(cel.Lib(registry))
	if err != nil {
		return "", err
	}

	prog, err := env.Program("root." + expression)
	if err != nil {
		return "", err
	}
	val, _, err := prog.Eval(map[string]interface{}{
		"root": object,
	})
	if err != nil {
		return "", err
	}

	v, ok := val.Value().(string)
	if !ok {
		return "", fmt.Errorf("not string: got %v of type %v", val.Value(), val.Type().TypeName())
	}

	return v, nil
}
