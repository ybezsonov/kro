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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ConstructSpec defines the desired state of Construct
type ConstructSpec struct {
	// +kubebuilder:validation:Required
	Kind string `json:"kind,omitempty"`
	// +kubebuilder:validation:Required
	ApiVersion string `json:"apiVersion,omitempty"`
	// +kubebuilder:validation:Required
	Definition *Definition `json:"definition,omitempty"`
	Resources  []*Resource `json:"resources,omitempty"`
}

type Definition struct {
	// +kubebuilder:validation:Required
	Spec runtime.RawExtension `json:"spec,omitempty"`
	// +kubebuilder:validation:Required
	Status   runtime.RawExtension `json:"status,omitempty"`
	Required []string             `json:"required,omitempty"`
}

type Resource struct {
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Required
	Definition runtime.RawExtension `json:"definition,omitempty"`
}

// ConstructStatus defines the observed state of Construct
type ConstructStatus struct {
	// State is the state of the construct
	State string `json:"state,omitempty"`
	// GraphState is the state of the construct graph
	GraphState string `json:"graphState,omitempty"`
	// TopologicalOrder is the topological order of the construct graph
	TopoligicalOrder []string `json:"creationOrder,omitempty"`
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Construct is the Schema for the constructs API
type Construct struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConstructSpec   `json:"spec,omitempty"`
	Status ConstructStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ConstructList contains a list of Construct
type ConstructList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Construct `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Construct{}, &ConstructList{})
}
