package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
)

type Engine struct {
	env *cel.Env
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Eval(exp string, input map[string]any) (*EvalResponse, error) {
	inputVars := make([]cel.EnvOption, 0, len(input))
	for k := range input {
		inputVars = append(inputVars, cel.Variable(k, cel.DynType))
	}
	env, err := cel.NewEnv(append(celEnvOptions, inputVars...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL env: %w", err)
	}
	ast, issues := env.Compile(exp)
	if issues != nil {
		return nil, fmt.Errorf("failed to compile the CEL expression: %s", issues.String())
	}
	prog, err := env.Program(ast, celProgramOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate CEL program: %w", err)
	}
	val, costTracker, err := prog.Eval(input)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate: %w", err)
	}

	response, err := generateResponse(val, costTracker)
	if err != nil {
		return nil, fmt.Errorf("failed to generate the response: %w", err)
	}
	return response, nil
}
