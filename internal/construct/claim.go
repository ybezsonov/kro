package construct

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type _ struct {
	metav1.ObjectMeta
	Spec runtime.RawExtension
}

type Claim struct {
	*unstructured.Unstructured
}

func (c *Claim) IsStatus(state string) bool {
	status, ok, err := unstructured.NestedString(c.Object, "status", "state")
	if err != nil || !ok {
		return "" == state
	}
	return status == state
}
