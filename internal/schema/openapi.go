package schema

import (
	"fmt"

	"sigs.k8s.io/yaml"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// OpenAPISchemaTransformer is a transformer for OpenAPI schemas
type OpenAPISchemaTransformer struct{}

func NewTransformer() *OpenAPISchemaTransformer {
	return &OpenAPISchemaTransformer{}
}

// Transform takes a raw object and returns a JSONSchemaProps
func (t *OpenAPISchemaTransformer) Transform(specRawObject []byte, statusRawObject []byte) (*extv1.JSONSchemaProps, error) {
	statusObjectMap := make(map[string]interface{})
	if err := yaml.Unmarshal(specRawObject, &statusObjectMap); err != nil {
		return nil, err
	}
	openAPIv3Schema := newBaseResource()
	specSchema := openAPIv3Schema.Properties["spec"]
	// now we recursively walk the objectMap and build the spec schema
	if err := t.buildSchema(statusObjectMap, &specSchema); err != nil {
		return nil, err
	}
	// now we can build the status schema
	statusSchema := openAPIv3Schema.Properties["status"]
	if err := t.buildStatusSchema(statusRawObject, &statusSchema); err != nil {
		return nil, err
	}
	return openAPIv3Schema, nil
}

func (t *OpenAPISchemaTransformer) buildStatusSchema(statusRawObject []byte, schema *extv1.JSONSchemaProps) error {
	statusObjectMap := make(map[string]interface{})
	if err := yaml.Unmarshal(statusRawObject, &statusObjectMap); err != nil {
		return err
	}
	if err := t.buildSchema(statusObjectMap, schema); err != nil {
		return err
	}
	// we need to inject State and Conditions
	schema.Properties["state"] = extv1.JSONSchemaProps{
		Type: "string",
	}
	schema.Properties["conditions"] = extv1.JSONSchemaProps{
		Type: "array",
		Items: &extv1.JSONSchemaPropsOrArray{
			Schema: &extv1.JSONSchemaProps{
				Type: "object",
				Properties: map[string]extv1.JSONSchemaProps{
					"type": {
						Type: "string",
					},
					"status": {
						Type: "string",
					},
					"reason": {
						Type: "string",
					},
					"message": {
						Type: "string",
					},
					"lastTransitionTime": {
						Type: "string",
					},
				},
			},
		},
	}
	return nil
}

func (t *OpenAPISchemaTransformer) buildSchema(objectMap map[string]interface{}, schema *extv1.JSONSchemaProps) error {
	for key, value := range objectMap {
		switch typedValue := value.(type) {
		case map[string]interface{}:
			// we have a nested object
			properties := extv1.JSONSchemaProps{
				Type:       "object",
				Properties: map[string]extv1.JSONSchemaProps{},
			}
			schema.Properties[key] = properties
			if err := t.buildSchema(value.(map[string]interface{}), &properties); err != nil {
				return err
			}
		case []interface{}:
			// we have an array
			properties := extv1.JSONSchemaProps{
				Type: "array",
				Items: &extv1.JSONSchemaPropsOrArray{
					Schema: &extv1.JSONSchemaProps{
						Type: "object",
					},
				},
			}
			schema.Properties[key] = properties
			if err := t.buildSchema(value.(map[string]interface{}), &properties); err != nil {
				return err
			}
		case string:
			// we have a string. The value is the type.
			// For basic types, we can just set the type
			switch typedValue {
			case "string":
				schema.Properties[key] = extv1.JSONSchemaProps{
					Type: "string",
				}
			case "int":
				schema.Properties[key] = extv1.JSONSchemaProps{
					Type: "integer",
				}
			case "bool":
				schema.Properties[key] = extv1.JSONSchemaProps{
					Type: "boolean",
				}
			case "map[string]string":
				schema.Properties[key] = extv1.JSONSchemaProps{
					Type: "object",
					AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
						Schema: &extv1.JSONSchemaProps{
							Type: "string",
						},
					},
				}
			case "[]string":
				schema.Properties[key] = extv1.JSONSchemaProps{
					Type: "array",
					Items: &extv1.JSONSchemaPropsOrArray{
						Schema: &extv1.JSONSchemaProps{
							Type: "string",
						},
					},
				}
			default:
				// User probably defined a complex type
				// NOTE(a-hilaly): figure out how to validate the complex type
				// or parse them.
				return fmt.Errorf("unknown type in schema: key: %s, value: %s", key, typedValue)
			}

		default:
			// we have a primitive
			schema.Properties[key] = extv1.JSONSchemaProps{
				Type: "string",
			}
		}
	}
	return nil
}

func newBaseResource() *extv1.JSONSchemaProps {
	return &extv1.JSONSchemaProps{
		Type:     "object",
		Required: []string{"spec"},
		Properties: map[string]extv1.JSONSchemaProps{
			"apiVersion": {
				Type: "string",
			},
			"kind": {
				Type: "string",
			},
			"metadata": {
				Type: "object",
			},
			"spec": {
				Type:       "object",
				Properties: map[string]extv1.JSONSchemaProps{},
			},
			"status": {
				Type:       "object",
				Properties: map[string]extv1.JSONSchemaProps{},
			},
		},
	}
}
