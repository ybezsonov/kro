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

package crd

import (
	"fmt"
	"strings"

	"github.com/gobuffalo/flect"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kro-run/kro/api/v1alpha1"
)

// SynthesizeCRD generates a CustomResourceDefinition for a given API version and kind
// with the provided spec and status schemas~
func SynthesizeCRD(group, apiVersion, kind string, spec, status extv1.JSONSchemaProps, statusFieldsOverride bool) *extv1.CustomResourceDefinition {
	crdGroup := group
	if crdGroup == "" {
		crdGroup = v1alpha1.KRODomainName
	}
	return newCRD(crdGroup, apiVersion, kind, newCRDSchema(spec, status, statusFieldsOverride))
}

func newCRD(group, apiVersion, kind string, schema *extv1.JSONSchemaProps) *extv1.CustomResourceDefinition {
	pluralKind := flect.Pluralize(strings.ToLower(kind))
	return &extv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s.%s", pluralKind, group),
			OwnerReferences: nil, // Injecting owner references is the responsibility of the caller.
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     kind,
				ListKind: kind + "List",
				Plural:   pluralKind,
				Singular: strings.ToLower(kind),
			},
			Scope: extv1.NamespaceScoped,
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    apiVersion,
					Served:  true,
					Storage: true,
					Schema: &extv1.CustomResourceValidation{
						OpenAPIV3Schema: schema,
					},
					Subresources: &extv1.CustomResourceSubresources{
						Status: &extv1.CustomResourceSubresourceStatus{},
					},
					AdditionalPrinterColumns: defaultAdditionalPrinterColumns,
				},
			},
		},
	}
}

func newCRDSchema(spec, status extv1.JSONSchemaProps, statusFieldsOverride bool) *extv1.JSONSchemaProps {
	if status.Properties == nil {
		status.Properties = make(map[string]extv1.JSONSchemaProps)
	}
	// if statusFieldsOverride is true, we will override the status fields with the default ones
	// TODO(a-hilaly): Allow users to override the default status fields.
	if statusFieldsOverride {
		if _, ok := status.Properties["state"]; !ok {
			status.Properties["state"] = defaultStateType
		}
		if _, ok := status.Properties["conditions"]; !ok {
			status.Properties["conditions"] = defaultConditionsType
		}
	}

	return &extv1.JSONSchemaProps{
		Type:     "object",
		Required: []string{},
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
			"spec":   spec,
			"status": status,
		},
	}
}
