package construct

import (
	"testing"
)

func TestResourceCopy(t *testing.T) {
	r1 := &Resource{
		RuntimeID: "r1",
		Variables: []*Variable{
			{
				Expression: "v1",
			},
			{
				Expression: "v2",
			},
		},
		Dependencies: []*ResourceRef{
			{
				RuntimeID: "r2",
			},
		},
		Children: []*ResourceRef{
			{
				RuntimeID: "r3",
			},
		},
	}
	r2 := r1.Copy()
	if r1.RuntimeID != r2.RuntimeID {
		t.Errorf("r1 and r2 should have the same runtime ID")
	}
	if &r1 == &r2 {
		t.Errorf("r1 and r2 should not be the same object")
	}
	if len(r1.Variables) != len(r2.Variables) {
		t.Errorf("r1 and r2 should have the same number of variables")
	}
	if len(r1.Dependencies) != len(r2.Dependencies) {
		t.Errorf("r1 and r2 should have the same number of dependencies")
	}
	if len(r1.Children) != len(r2.Children) {
		t.Errorf("r1 and r2 should have the same number of children")
	}

	r1.Variables[0].Expression = "v3"
	r1.Dependencies[0].RuntimeID = "r4"
	if r1.Dependencies[0] == r2.Dependencies[0] {
		t.Errorf("r1 and r2 should not have the same dependency")
	}
}

func TestResourceDependenciesAndChildren(t *testing.T) {
	r1 := &Resource{
		RuntimeID: "r1",
	}

	r1.AddDependency("r2")
	r1.AddDependency("r3")
	if !r1.HasDependency("r2") {
		t.Errorf("r1 should have r2 as a dependency")
	}
	if !r1.HasDependency("r3") {
		t.Errorf("r1 should have r3 as a dependency")
	}
	if r1.HasDependency("r4") {
		t.Errorf("r1 should not have r4 as a dependency")
	}
	r1.RemoveDependency("r2")
	if r1.HasDependency("r2") {
		t.Errorf("r1 should not have r2 as a dependency")
	}
	if len(r1.Dependencies) != 1 {
		t.Errorf("r1 should have 1 dependency")
	}
}
