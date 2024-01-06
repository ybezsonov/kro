package construct

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

type Variable struct {
	Expression string
	Kind       VariableKind
	Type       VariableType
	SrcRef     *ResourceRef

	ResolvedValue interface{}
}

type VariableType string

const (
	VariableTypeStaticReference VariableType = "VariableTypeStaticReference"
)

type VariableKind string

const (
	VariableKindUnknown                 VariableKind = "VariableUnknown"
	VariableKindClaimSpecRefrence       VariableKind = "VariableMainConstructRefrence"
	VariableKindClaimStatusRefrence     VariableKind = "VariableClaimStatusRefrence"
	VariableKindResourceSpecReference   VariableKind = "VariableResourceSpecReference"
	VariableKindResourceStatusReference VariableKind = "VariableResourceStatusReference"
)

var (
	referencesRegex = regexp.MustCompile(`\$\{.*\}`)
)

func extractVariables(raw []byte) ([]*Variable, error) {
	matches := referencesRegex.FindAll(raw, -1)
	variables := make([]*Variable, len(matches))
	for i, match := range matches {
		variables[i] = &Variable{Expression: string(match)}
	}
	return variables, nil
}

func (r *Resource) replaceVariables(vars map[string]string) []byte {
	copy := bytes.Clone(r.Raw)
	for expr, elem := range vars {
		trapRegex := regexp.MustCompile(regexExpression(expr))
		copy = trapRegex.ReplaceAll(copy, []byte(elem))
	}
	return copy
}

func trimReferenceSyntax(reference string) string {
	if !strings.HasPrefix(reference, "${") && !strings.HasSuffix(reference, "}") {
		return reference
	}
	reference = strings.TrimLeft(reference, "${")
	reference = strings.TrimRight(reference, "}")
	return reference
}

func regexExpression(expression string) string {
	expression = trimReferenceSyntax(expression)
	expression = fmt.Sprintf(`\$\{%s\}`, expression)
	return strings.ReplaceAll(expression, ".", `\.`)
}
