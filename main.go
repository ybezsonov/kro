package main

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/wzshiming/easycel"
)

func eval() (*string, error) {
	registry := easycel.NewRegistry("symphony-cel-engine", easycel.WithTagName("json"))

	// instantly register the two variables we know we'll need
	err := registry.RegisterVariable("variablex", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	registry.RegisterFunction("range",
		func(size types.Int) []int {
			r := make([]int, size)
			for i := range r {
				r[i] = i
			}
			return r
		},
	)
	env, err := easycel.NewEnvironment(cel.Lib(registry))
	if err != nil {
		return nil, err
	}

	expr := `range(0, 10, 5).map(x, {"key"+string(x): x, "value"+string(x): x})`
	prog, err := env.Program(expr)
	if err != nil {
		return nil, err
	}
	val, _, err := prog.Eval(map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	fmt.Println("type:", val.Type())
	fmt.Println("value:", val.Value())
	if err != nil {
		panic(err)
	}
	return nil, nil
}

func main() {
	_, err := eval()
	if err != nil {
		panic(err)
	}
}
