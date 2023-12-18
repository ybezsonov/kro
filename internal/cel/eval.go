package cel

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"google.golang.org/protobuf/types/known/structpb"
)

type EvalResponse struct {
	Result any     `json:"result"`
	Cost   *uint64 `json:"cost, omitempty"`
}

// Eval evaluates the cel expression against the given input
func Eval(exp string, input map[string]any) (string, error) {
	inputVars := make([]cel.EnvOption, 0, len(input))
	for k := range input {
		inputVars = append(inputVars, cel.Variable(k, cel.DynType))
	}
	env, err := cel.NewEnv(append(celEnvOptions, inputVars...)...)
	if err != nil {
		return "", fmt.Errorf("failed to create CEL env: %w", err)
	}
	ast, issues := env.Compile(exp)
	if issues != nil {
		return "", fmt.Errorf("failed to compile the CEL expression: %s", issues.String())
	}
	prog, err := env.Program(ast, celProgramOptions...)
	if err != nil {
		return "", fmt.Errorf("failed to instantiate CEL program: %w", err)
	}
	val, costTracker, err := prog.Eval(input)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate: %w", err)
	}

	response, err := generateResponse(val, costTracker)
	if err != nil {
		return "", fmt.Errorf("failed to generate the response: %w", err)
	}

	out, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal the output: %w", err)
	}
	return string(out), nil
}

func getResults(val *ref.Val) (any, error) {
	if value, err := (*val).ConvertToNative(reflect.TypeOf(&structpb.Value{})); err != nil {
		return nil, err
	} else {
		return value, nil
	}
}

func generateResponse(val ref.Val, costTracker *cel.EvalDetails) (*EvalResponse, error) {
	result, evalError := getResults(&val)
	if evalError != nil {
		return nil, evalError
	}
	cost := costTracker.ActualCost()
	return &EvalResponse{
		Result: result,
		Cost:   cost,
	}, nil
}
