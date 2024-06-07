/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// ResourceGroupSpec defines the desired state of ResourceGroup
type ResourceGroupSpec struct {
	// +kubebuilder:validation:Required
	Kind string `json:"kind,omitempty"`
	// +kubebuilder:validation:Required
	// APIVersion is the kubernetes API version of the resourcegroup.
	//
	// Ideally the user shouldn't have to provide this field.. But rather
	// symphony needs to compute the mutation deltas, manage API versions
	// and provide a default API version.
	APIVersion string `json:"apiVersion,omitempty"`
	// +kubebuilder:validation:Required
	// Schema is the schema of the CRD mapping to a resource group.
	Schema *Schema `json:"schema,omitempty"`
	// +kubebuilder:validation:Required
	//
	// Resources is the list of resources representing the resourcegroup.
	Resources []*Resource `json:"resources,omitempty"`
}

type Schema struct {
	// +kubebuilder:validation:Required
	Spec runtime.RawExtension `json:"spec,omitempty"`
	// +kubebuilder:validation:Required
	Status runtime.RawExtension `json:"status,omitempty"`
	//
	Required []string `json:"required,omitempty"`
}

type Resource struct {
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Required
	Definition runtime.RawExtension `json:"definition,omitempty"`
}

// ResourceGroupStatus defines the observed state of ResourceGroup
type ResourceGroupStatus struct {
	// State is the state of the resourcegroup
	State string `json:"state,omitempty"`
	// GraphState is the state of the resourcegroup graph
	GraphState string `json:"graphState,omitempty"`
	// TopologicalOrder is the topological order of the resourcegroup graph
	TopologicalOrder []string `json:"topologicalOrder,omitempty"`
	// Conditions represent the latest available observations of an object's state
	Conditions []Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ResourceGroup is the Schema for the resourcegroups API
type ResourceGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceGroupSpec   `json:"spec,omitempty"`
	Status ResourceGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ResourceGroupList contains a list of ResourceGroup
type ResourceGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceGroup{}, &ResourceGroupList{})
}
