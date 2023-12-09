package schema

import extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

// Transformer is an interface for transforming raw objects into JSONSchemaProps
type Transformer interface {
	// Transform takes a raw object and returns a JSONSchemaProps
	Transform(raw interface{}) (*extv1.JSONSchemaProps, error)
}
