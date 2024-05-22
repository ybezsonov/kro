package resourcegroup

import (
	"bytes"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type ResourceRef struct {
	RuntimeID string
}

type Resource struct {
	RuntimeID string

	Data map[string]interface{}
	Raw  []byte

	Children     []*ResourceRef
	Dependencies []*ResourceRef

	Variables []*Variable
}

func NewResourceFromRaw(runtimeID string, raw []byte) (*Resource, error) {
	var data map[string]interface{}
	err := yaml.Unmarshal(raw, &data)
	if err != nil {
		return nil, err
	}

	variables, err := extractVariables(raw)
	if err != nil {
		return nil, err
	}

	resource := &Resource{
		RuntimeID:    runtimeID,
		Data:         data,
		Raw:          raw,
		Variables:    variables,
		Children:     make([]*ResourceRef, 0),
		Dependencies: make([]*ResourceRef, 0),
	}
	return resource, nil
}

func (r *Resource) SetStatus(status map[string]interface{}) error {
	r.Data["status"] = status
	raw, err := yaml.Marshal(r.Data)
	if err != nil {
		return err
	}
	r.Raw = raw
	fmt.Println("SET STATUS FOR RESOURCE: ", r.RuntimeID)
	return nil
}

func (r *Resource) HasStatus() bool {
	_, ok := r.Data["status"]
	return ok
}

// Some resources like ServiceAccount, ConfigMap, etc. don't have status
// We need to hard code this for now.
func (r *Resource) IsStatusless() bool {
	group := r.Unstructured().GetAPIVersion()
	kind := r.Unstructured().GetKind()
	s := !strings.Contains(group, "services.k8s.aws") && (kind == "ServiceAccount" ||
		kind == "ConfigMap" ||
		kind == "Secret" ||
		kind == "Role" ||
		kind == "RoleBinding" ||
		kind == "ClusterRole" ||
		kind == "ClusterRoleBinding")
	fmt.Println(".... is statusless", s, r.RuntimeID)
	return s
}

func haveRef(refs []*ResourceRef, runtimeID string) bool {
	for _, ref := range refs {
		if ref.RuntimeID == runtimeID {
			return true
		}
	}
	return false
}

func (r *Resource) HasNDependencies(n int) bool {
	return len(r.Dependencies) == n
}

func (r *Resource) Copy() *Resource {
	rawCopy := bytes.Clone(r.Raw)
	var dataCopy map[string]interface{}
	err := yaml.Unmarshal(rawCopy, &dataCopy)
	if err != nil {
		panic(err)
	}

	childrenCopy := make([]*ResourceRef, len(r.Children))
	for i, child := range r.Children {
		childrenCopy[i] = &ResourceRef{child.RuntimeID}
	}
	dependenciesCopy := make([]*ResourceRef, len(r.Dependencies))
	for i, dependency := range r.Dependencies {
		dependenciesCopy[i] = &ResourceRef{dependency.RuntimeID}
	}

	variablesCopy := make([]*Variable, len(r.Variables))
	for i, variable := range r.Variables {
		variablesCopy[i] = &Variable{
			Expression:    variable.Expression,
			Kind:          variable.Kind,
			Type:          variable.Type,
			ResolvedValue: variable.ResolvedValue,
		}
		if variable.SrcRef != nil {
			variablesCopy[i].SrcRef = &ResourceRef{variable.SrcRef.RuntimeID}
		}
	}

	return &Resource{
		RuntimeID:    r.RuntimeID,
		Data:         dataCopy,
		Raw:          rawCopy,
		Children:     childrenCopy,
		Dependencies: dependenciesCopy,
		Variables:    variablesCopy,
	}
}

func copyResources(resources []*Resource) []*Resource {
	resourcesCopy := make([]*Resource, len(resources))
	for i, resource := range resources {
		resourcesCopy[i] = resource.Copy()
	}
	return resourcesCopy
}

func (r *Resource) HasChildren(runtimeID string) bool {
	return haveRef(r.Children, runtimeID)
}

func (r *Resource) HasDependency(runtimeID string) bool {
	return haveRef(r.Dependencies, runtimeID)
}

func (r *Resource) AddChildren(runtimeID string) {
	if !r.HasChildren(r.RuntimeID) {
		r.Children = append(r.Children, &ResourceRef{runtimeID})
	}
}

func (r *Resource) RemoveChildren(runtimeID string) {
	newChildren := make([]*ResourceRef, 0)
	for _, child := range r.Children {
		if child.RuntimeID != runtimeID {
			newChildren = append(newChildren, child)
		}
	}
	r.Children = newChildren
}

func (r *Resource) RemoveDependency(runtimeID string) {
	newDependencies := make([]*ResourceRef, 0)
	for _, dependency := range r.Dependencies {
		if dependency.RuntimeID != runtimeID {
			newDependencies = append(newDependencies, dependency)
		}
	}
	r.Dependencies = newDependencies
}

func (r *Resource) AddDependency(runtimeID string) {
	if !r.HasDependency(r.RuntimeID) {
		r.Dependencies = append(r.Dependencies, &ResourceRef{runtimeID})
	}
}

func (r *Resource) Unstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: r.Data,
	}
}

func (r *Resource) GVR() schema.GroupVersionResource {
	gvk := r.Unstructured().GroupVersionKind()
	plural := ""
	// if kind finishes with "y" then replace with "ies"
	if strings.HasSuffix(gvk.Kind, "y") {
		plural = strings.TrimSuffix(gvk.Kind, "y") + "ies"
	} else {
		plural = gvk.Kind + "s"
	}
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: strings.ToLower(plural),
	}
}

func (r *Resource) Metadata() metav1.ObjectMeta {
	md, ok := r.Data["metadata"]
	if ok {
		mdInstance, ok := md.(metav1.ObjectMeta)
		if !ok {
			panic("unexpected metadata type")
		}
		return mdInstance
	}
	return metav1.ObjectMeta{}
}

func (r *Resource) WithReplacedVariables(data map[string]string) *Resource {
	return &Resource{
		RuntimeID:    r.RuntimeID,
		Data:         r.Data,
		Raw:          r.replaceVariables(data),
		Children:     r.Children,
		Dependencies: r.Dependencies,
	}
}

func validateNamesDuplication(resources []*Resource) error {
	seen := make(map[string]int)
	for index, resource := range resources {
		if firstIndex, ok := seen[resource.RuntimeID]; ok {
			return fmt.Errorf("resource %s is duplicated: found at %d and %d", resource.RuntimeID, firstIndex, index)
		}
		seen[resource.RuntimeID] = index
	}
	return nil
}
