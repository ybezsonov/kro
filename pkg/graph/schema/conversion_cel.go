// Copyright 2025 The Kube Resource Orchestrator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema

import (
	"fmt"

	"github.com/google/cel-go/common/types/ref"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	krocel "github.com/kro-run/kro/pkg/cel"
)

// inferSchemaFromCELValue infers a JSONSchemaProps from a CEL value.
func inferSchemaFromCELValue(val ref.Val) (*extv1.JSONSchemaProps, error) {
	if val == nil {
		return nil, fmt.Errorf("value is nil")
	}
	goRuntimeVal, err := krocel.GoNativeType(val)
	if err != nil {
		return nil, fmt.Errorf("failed to convert CEL value to Go: %w", err)
	}
	return inferSchemaTypeFromGoValue(goRuntimeVal)
}

func inferSchemaTypeFromGoValue(goRuntimeVal interface{}) (*extv1.JSONSchemaProps, error) {
	switch goRuntimeVal := goRuntimeVal.(type) {
	case bool:
		return &extv1.JSONSchemaProps{
			Type: "boolean",
		}, nil
	case int64:
		return &extv1.JSONSchemaProps{
			Type: "integer",
		}, nil
	case uint64:
		return &extv1.JSONSchemaProps{
			Type: "integer",
		}, nil
	case float64:
		return &extv1.JSONSchemaProps{
			Type: "number",
		}, nil
	case string:
		return &extv1.JSONSchemaProps{
			Type: "string",
		}, nil
	case []interface{}:
		return inferArraySchema(goRuntimeVal)
	case map[string]interface{}:
		return inferObjectSchema(goRuntimeVal)
	default:
		return nil, fmt.Errorf("unsupported type: %T", goRuntimeVal)
	}
}

func inferArraySchema(arr []interface{}) (*extv1.JSONSchemaProps, error) {
	schema := &extv1.JSONSchemaProps{
		Type: "array",
	}

	if len(arr) > 0 {
		itemSchema, err := inferSchemaTypeFromGoValue(arr[0])
		if err != nil {
			return nil, fmt.Errorf("failed to infer schema for array item: %w", err)
		}
		schema.Items = &extv1.JSONSchemaPropsOrArray{
			Schema: itemSchema,
		}
	}

	return schema, nil
}

func inferObjectSchema(obj map[string]interface{}) (*extv1.JSONSchemaProps, error) {
	schema := &extv1.JSONSchemaProps{
		Type:       "object",
		Properties: make(map[string]extv1.JSONSchemaProps),
	}

	for key, value := range obj {
		propSchema, err := inferSchemaTypeFromGoValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to infer schema for property %s: %w", key, err)
		}
		schema.Properties[key] = *propSchema
	}

	return schema, nil
}
