package resourcegroup

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceGroup interface {
	Resources() []*ResourceBase
	Graph() *Graph
}

type ResourceGroupClaim ResourceBase

type ResourceBase interface {
	Identifier() string
	Index() int
	GVR() *schema.GroupVersionResource
	Metadata() metav1.TypeMeta
	Spec() runtime.RawExtension
	Status() runtime.RawExtension
}

type ResourceManager interface {
	Sync() error
	WaitForState(state interface{}) error
	GetReferences()
	SetReferences()
	GetChildren()
	GetDependencies()
}

type _Graph interface {
	Root() ResourceBase
}
