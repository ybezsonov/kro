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

package metadata

import (
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/release-utils/version"

	"github.com/kro-run/kro/api/v1alpha1"
)

const (
	// LabelKROPrefix is the label key prefix used to identify KRO owned resources.
	LabelKROPrefix = v1alpha1.KRODomainName + "/"
)

const (
	NodeIDLabel = LabelKROPrefix + "node-id"

	OwnedLabel      = LabelKROPrefix + "owned"
	KROVersionLabel = LabelKROPrefix + "kro-version"

	InstanceIDLabel        = LabelKROPrefix + "instance-id"
	InstanceLabel          = LabelKROPrefix + "instance-name"
	InstanceNamespaceLabel = LabelKROPrefix + "instance-namespace"

	ResourceGraphDefinitionIDLabel        = LabelKROPrefix + "resource-graph-definition-id"
	ResourceGraphDefinitionNameLabel      = LabelKROPrefix + "resource-graph-definition-name"
	ResourceGraphDefinitionNamespaceLabel = LabelKROPrefix + "resource-graph-definition-namespace"
	ResourceGraphDefinitionVersionLabel   = LabelKROPrefix + "resource-graph-definition-version"
)

// IsKROOwned returns true if the resource is owned by KRO.
func IsKROOwned(meta metav1.ObjectMeta) bool {
	v, ok := meta.Labels[OwnedLabel]
	return ok && booleanFromString(v)
}

// SetKROOwned sets the OwnedLabel to true on the resource.
func SetKROOwned(meta metav1.ObjectMeta) {
	setLabel(&meta, OwnedLabel, stringFromBoolean(true))
}

// SetKROUnowned sets the OwnedLabel to false on the resource.
func SetKROUnowned(meta metav1.ObjectMeta) {
	setLabel(&meta, OwnedLabel, stringFromBoolean(false))
}

var (
	ErrDuplicatedLabels = errors.New("duplicate labels")
)

var _ Labeler = GenericLabeler{}

// Labeler is an interface that defines a set of labels that can be
// applied to a resource.
type Labeler interface {
	Labels() map[string]string
	ApplyLabels(metav1.Object)
	Merge(Labeler) (Labeler, error)
}

// GenericLabeler is a map of labels that can be applied to a resource.
// It implements the Labeler interface.
type GenericLabeler map[string]string

// Labels returns the labels.
func (gl GenericLabeler) Labels() map[string]string {
	return gl
}

// ApplyLabels applies the labels to the resource.
func (gl GenericLabeler) ApplyLabels(meta metav1.Object) {
	for k, v := range gl {
		setLabel(meta, k, v)
	}
}

// Merge merges the labels from the other labeler into the current
// labeler. If there are any duplicate keys, an error is returned.
func (gl GenericLabeler) Merge(other Labeler) (Labeler, error) {
	newLabels := gl.Copy()
	for k, v := range other.Labels() {
		if _, ok := newLabels[k]; ok {
			return nil, fmt.Errorf("%v: found key '%s' in both maps", ErrDuplicatedLabels, k)
		}
		newLabels[k] = v
	}
	return GenericLabeler(newLabels), nil
}

// Copy returns a copy of the labels.
func (gl GenericLabeler) Copy() map[string]string {
	newGenericLabeler := map[string]string{}
	for k, v := range gl {
		newGenericLabeler[k] = v
	}
	return newGenericLabeler
}

// NewResourceGraphDefinitionLabeler returns a new labeler that sets the
// ResourceGraphDefinitionLabel and ResourceGraphDefinitionIDLabel labels on a resource.
func NewResourceGraphDefinitionLabeler(rgMeta metav1.Object) GenericLabeler {
	return map[string]string{
		ResourceGraphDefinitionIDLabel:   string(rgMeta.GetUID()),
		ResourceGraphDefinitionNameLabel: rgMeta.GetName(),
	}
}

// NewInstanceLabeler returns a new labeler that sets the InstanceLabel and
// InstanceIDLabel labels on a resource. The InstanceLabel is the namespace
// and name of the instance that was reconciled to create the resource.
func NewInstanceLabeler(instanceMeta metav1.Object) GenericLabeler {
	return map[string]string{
		InstanceIDLabel:        string(instanceMeta.GetUID()),
		InstanceLabel:          instanceMeta.GetName(),
		InstanceNamespaceLabel: instanceMeta.GetNamespace(),
	}
}

// NewKROMetaLabeler returns a new labeler that sets the OwnedLabel,
// KROVersion, and ControllerPodID labels on a resource.
func NewKROMetaLabeler() GenericLabeler {
	return map[string]string{
		OwnedLabel:      "true",
		KROVersionLabel: safeVersion(version.GetVersionInfo().GitVersion),
	}
}

func safeVersion(version string) string {
	if validation.IsValidLabelValue(version) == nil {
		return version
	}
	// The script we use might add '+dirty' to development branches,
	// so let's try replacing '+' with '-'.
	return strings.ReplaceAll(version, "+", "-")
}

func booleanFromString(s string) bool {
	// for the sake of simplicity we'll avoid doing any kind
	// of parsing here. Since those labels are set by the controller
	// it self. We'll expect the same values back.
	return s == "true"
}

func stringFromBoolean(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Helper function to set a label
func setLabel(meta metav1.Object, key, value string) {
	labels := meta.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	meta.SetLabels(labels)
}
