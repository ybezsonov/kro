package cel

import (
	"testing"

	"sigs.k8s.io/yaml"
)

func TestEngine(t *testing.T) {
	e := NewBasicEngine()
	value, err := e.Eval("a.b", map[string]interface{}{
		"a": map[string]interface{}{
			"b": "c",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if value != "c" {
		t.Fatal(value)
	}

	value, err = e.Eval("a.b.c.d[0]", map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": map[string]interface{}{
					"d": []string{"e"},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if value != "e" {
		t.Fatal(value)
	}
}

const (
	resourcePod = `
apiVersion: v1
kind: Pod
metadata:
  name: pod-a
  namespace: default
spec:
  containers:
  - name: container-a
    image: nginx
`
)

func TestEngineWithResource(t *testing.T) {
	e := NewBasicEngine()
	var object map[string]interface{}
	yaml.Unmarshal([]byte(resourcePod), &object)

	value, err := e.Eval("object.metadata.name", map[string]interface{}{
		"object": object,
	})
	if err != nil {
		t.Fatal(err)
	}
	if value != "pod-a" {
		t.Fatal(value)
	}

	value, err = e.Eval("object.spec.containers[0].name", map[string]interface{}{
		"object": object,
	})
	if err != nil {
		t.Fatal(err)
	}
	if value != "container-a" {
		t.Fatal(value)
	}
}
