package resourcegroup

import (
	"testing"

	"github.com/aws/symphony/api/v1alpha1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const (
	IRSAResourceGroupSpec = `definition:
  spec:
    awsAccountID: string
    eksOIDC: string
    permissionsBoundaryArn: string
    policyArns: '[]string'
    serviceAccountName: string
    resourceConfig:
    deletionPolicy: string
    region: string
    tags: map[string]string`

	IRSAResourceA = `apiVersion: iam.services.k8s.aws/v1alpha1
kind: Policy
metadata:
  name: boundary-policy
spec:
  name: boundary-policy
  policyDocument: '{something something}'`

	IRSAResourceB = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${spec.serviceAccountName}
  annotations:
    eks.amazonaws.com/role-arn: ${irsa-role.status.ACKResourceMetadata.ARN}`

	IRSAResourceC = `
apiVersion: iam.services.k8s.aws/v1alpha1
kind: Role
metadata:
  name: irsa-role
spec:
  name: irsa-role
  PermissionsBoundary: ${boundaryPolicy.status.ACKResourceMetadata.ARN}
  policies: ${spec.policyARNs}`
)

func Test_NewGraph(t *testing.T) {
	resourceA := v1alpha1.Resource{
		Name:       "boundaryPolicy",
		Definition: runtime.RawExtension{Raw: []byte(IRSAResourceA)},
	}
	resourceB := v1alpha1.Resource{
		Name:       "service-account",
		Definition: runtime.RawExtension{Raw: []byte(IRSAResourceB)},
	}
	resourceC := v1alpha1.Resource{
		Name:       "irsa-role",
		Definition: runtime.RawExtension{Raw: []byte(IRSAResourceC)},
	}

	g, err := NewGraph([]v1alpha1.Resource{resourceA, resourceB, resourceC})
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	if len(g.Resources) != 3 {
		t.Fatalf("expected 3 resources, got: %d", len(g.Resources))
	}

	/* 	// print resource names and their dependencies
	   	for _, r := range g.Resources {
	   		fmt.Println(r.RuntimeID)
	   		for _, d := range r.Dependencies {
	   			fmt.Println("  d:", d.RuntimeID)
	   		}
	   		for _, c := range r.Children {
	   			fmt.Println("  c:", c.RuntimeID)
	   		}
	   		fmt.Println("")
	   	} */

	order, err := g.getCreationOrder()
	if err != nil {
		t.Fatalf("got order error: %v", err)
	}
	_ = order
}

const (
	A = `
kind: A
spec:
  dependendsOn: ${B.status.Something}
`
	B = `
kind: B
spec:
  dependendsOn: ${C.status.Something}`
	C = `
kind: C
spec:
  dependendsOn: ${A.status.Something}`
)

// testing complex order creation
func Test_NewGraphCircular(t *testing.T) {
	resourceA := v1alpha1.Resource{
		Name:       "A",
		Definition: runtime.RawExtension{Raw: []byte(A)},
	}
	resourceB := v1alpha1.Resource{
		Name:       "B",
		Definition: runtime.RawExtension{Raw: []byte(B)},
	}
	resourceC := v1alpha1.Resource{
		Name:       "C",
		Definition: runtime.RawExtension{Raw: []byte(C)},
	}

	_, err := NewGraph([]v1alpha1.Resource{resourceA, resourceB, resourceC})
	if err == nil {
		t.Fatalf("expected circular error")
	}

	/* 	order, err := g.getCreationOrder()
	   	if err == nil {
	   		t.Fatalf("expected error, got none: %v", err)
	   	}
	   	_ = order */
	/*
		 	fmt.Println("error: ", err.Error())
			fmt.Println("length: ", len(order))
	*/
}

const (
	W = `
kind: W
spec:
  dependendsOn: ${Z.status.Something}
`
	N = `
kind: N
spec:
  dependendsOn: ${X.status.Something}`
	X = `
kind: X
spec:
  dependendsOn: ${Z.status.Something}
`
	Y = `
kind: Y
spec:
  dependendsOn: ${status.test}`
	Z = `
kind: Z
spec:
  dependendsOn: ${Y.status.Something}`

	M = `
kind: M
spec:
  dependendsOn: ${X.status.Something}`
	O = `
kind: M
spec:
  dependendsOn: ${X.status.Something}`
)

func Test_GraphComplex(t *testing.T) {
	resourceW := v1alpha1.Resource{
		Name:       "W",
		Definition: runtime.RawExtension{Raw: []byte(W)},
	}
	resourceX := v1alpha1.Resource{
		Name:       "X",
		Definition: runtime.RawExtension{Raw: []byte(X)},
	}
	resourceY := v1alpha1.Resource{
		Name:       "Y",
		Definition: runtime.RawExtension{Raw: []byte(Y)},
	}
	resourceZ := v1alpha1.Resource{
		Name:       "Z",
		Definition: runtime.RawExtension{Raw: []byte(Z)},
	}
	resourceM := v1alpha1.Resource{
		Name:       "M",
		Definition: runtime.RawExtension{Raw: []byte(M)},
	}
	resourceN := v1alpha1.Resource{
		Name:       "N",
		Definition: runtime.RawExtension{Raw: []byte(N)},
	}
	resourceO := v1alpha1.Resource{
		Name:       "O",
		Definition: runtime.RawExtension{Raw: []byte(O)},
	}

	g, err := NewGraph([]v1alpha1.Resource{
		resourceW, resourceX, resourceY, resourceZ,
		resourceM, resourceN, resourceO,
	})
	if err != nil {
		t.Fatalf("wtf")
	}

	order, err := g.getCreationOrder()
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	expectedOrder := []string{"Y", "Z", "W", "X", "M", "N", "O"}
	if len(order) != len(expectedOrder) {
		t.Fatalf("expected order length %d, got %d", len(expectedOrder), len(order))
	}

	for i, r := range order {
		if r.RuntimeID != expectedOrder[i] {
			t.Fatalf("expected %s, got %s", expectedOrder[i], r.RuntimeID)
		}
	}
}

const (
	irsaClaim = `
apiVersion: x.symphony.k8s.aws/v1alpha1
kind: IRSA
metadata:
  name: my-first-irsa
spec:
  awsAccountID: account-id
  eksOIDC: oidc-name
  permissionsBoundaryArn: arn
  policyARNs: ARN1,ARN2
  serviceAccountName: my-service-account
  resourceConfig:
    deletionPolicy: Retain
    region: us-west-2
    tags:
      key: value
`
)

func TestGraphStaticResolving(t *testing.T) {
	resourceA := v1alpha1.Resource{
		Name:       "boundaryPolicy",
		Definition: runtime.RawExtension{Raw: []byte(IRSAResourceA)},
	}
	resourceB := v1alpha1.Resource{
		Name:       "service-account",
		Definition: runtime.RawExtension{Raw: []byte(IRSAResourceB)},
	}
	resourceC := v1alpha1.Resource{
		Name:       "irsa-role",
		Definition: runtime.RawExtension{Raw: []byte(IRSAResourceC)},
	}

	claimUnstruct := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(irsaClaim), &claimUnstruct)
	require.NoError(t, err)

	g, err := NewGraph([]v1alpha1.Resource{resourceA, resourceB, resourceC})
	g.Claim = Claim{
		Unstructured: &unstructured.Unstructured{
			Object: claimUnstruct,
		},
	}
	require.NoError(t, err)

	err = g.TopologicalSort()
	require.NoError(t, err)
	err = g.ResolvedVariables()
	require.NoError(t, err)
	err = g.ReplaceVariables()
	require.NoError(t, err)

	g.GetResource("boundaryPolicy")
}

const (
	xClaim = `
apiVersion: x.symphony.k8s.aws/v1alpha1
kind: X
metadata:
  name: x-name
spec:
  value: x-value
  param1: param1-value
  param2: param2-value
`

	A1 = `
kind: A
metadata:
  name: a-name
spec:
  value: a-value
  staticRefFromB: ${B1.spec.value}
`

	B1 = `
kind: B
metadata:
  name: b-name
spec:
  value: b-value
  staticRefFromC: ${C1.spec.value}
`

	C1 = `
kind: C
metadata:
  name: c-name
spec:
  value: c-value
  staticRefFromClaim: ${spec.param1}`

	C2 = `
kind: C
metadata:
  name: c-name-2
spec:
  staticRefFromA: ${A1.spec.value}
  staticRefFromB: ${B1.spec.value}
  staticRefFromC: ${C1.spec.value}
  staticRefFromClaim: ${spec.param1}
  staticRefFromClaim2: ${spec.param2}
`
)

func TestGraphStaticResolvingComplex(t *testing.T) {
	resourceA := v1alpha1.Resource{
		Name:       "A1",
		Definition: runtime.RawExtension{Raw: []byte(A1)},
	}
	resourceB := v1alpha1.Resource{
		Name:       "B1",
		Definition: runtime.RawExtension{Raw: []byte(B1)},
	}
	resourceC := v1alpha1.Resource{
		Name:       "C1",
		Definition: runtime.RawExtension{Raw: []byte(C1)},
	}
	resourceC2 := v1alpha1.Resource{
		Name:       "C2",
		Definition: runtime.RawExtension{Raw: []byte(C2)},
	}
	xClaimUnstruct := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(xClaim), &xClaimUnstruct)
	require.NoError(t, err)

	g, err := NewGraph([]v1alpha1.Resource{resourceA, resourceB, resourceC, resourceC2})
	require.NoError(t, err)
	g.Claim = Claim{
		Unstructured: &unstructured.Unstructured{
			Object: xClaimUnstruct,
		},
	}

	err = g.TopologicalSort()
	require.NoError(t, err)
	err = g.ResolvedVariables()
	require.NoError(t, err)
	err = g.ReplaceVariables()
	require.NoError(t, err)

	/* 	bb, _ := json.MarshalIndent(g.Resources[3], "", "  ")
	   	t.Log(string(bb)) */

	a, err := g.GetResource("A1")
	require.NoError(t, err)
	require.Equal(t, "a-value", a.Data["spec"].(map[string]interface{})["value"])
	require.Equal(t, "b-value", a.Data["spec"].(map[string]interface{})["staticRefFromB"])

	b, err := g.GetResource("B1")
	require.NoError(t, err)
	require.Equal(t, "b-value", b.Data["spec"].(map[string]interface{})["value"])
	require.Equal(t, "c-value", b.Data["spec"].(map[string]interface{})["staticRefFromC"])

	c, err := g.GetResource("C1")
	require.NoError(t, err)
	require.Equal(t, "c-value", c.Data["spec"].(map[string]interface{})["value"])
	require.Equal(t, "param1-value", c.Data["spec"].(map[string]interface{})["staticRefFromClaim"])

	c2, err := g.GetResource("C2")
	require.NoError(t, err)
	require.Equal(t, "param1-value", c2.Data["spec"].(map[string]interface{})["staticRefFromClaim"])
	require.Equal(t, "param2-value", c2.Data["spec"].(map[string]interface{})["staticRefFromClaim2"])
	require.Equal(t, "a-value", c2.Data["spec"].(map[string]interface{})["staticRefFromA"])
	require.Equal(t, "b-value", c2.Data["spec"].(map[string]interface{})["staticRefFromB"])
	require.Equal(t, "c-value", c2.Data["spec"].(map[string]interface{})["staticRefFromC"])
}

const (
	C3 = `
kind: C
metadata:
  name: c-name-3
spec:
  staticRefFromA: ${A1.spec.value}
  staticRefFromB: ${B1.spec.value}
  staticRefFromC: ${C1.spec.value}
  staticRefFromClaim: ${spec.param1}
  staticRefFromClaim2: ${spec.param2}
  staticStatusRefFromA: ${A1.status.state}
  staticStatusRefFromB: ${B1.status.state}
`
	C4 = `
kind: C
metadata:
  name: c-name-4
spec:
  staticStatusRefFromC3: ${C3.status.state}
`
)

func TestGraphStaticResolvingStatus(t *testing.T) {
	resourceA := v1alpha1.Resource{
		Name:       "A1",
		Definition: runtime.RawExtension{Raw: []byte(A1)},
	}
	resourceB := v1alpha1.Resource{
		Name:       "B1",
		Definition: runtime.RawExtension{Raw: []byte(B1)},
	}
	resourceC := v1alpha1.Resource{
		Name:       "C1",
		Definition: runtime.RawExtension{Raw: []byte(C1)},
	}
	resourceC2 := v1alpha1.Resource{
		Name:       "C2",
		Definition: runtime.RawExtension{Raw: []byte(C2)},
	}
	resourceC3 := v1alpha1.Resource{
		Name:       "C3",
		Definition: runtime.RawExtension{Raw: []byte(C3)},
	}
	resourceC4 := v1alpha1.Resource{
		Name:       "C4",
		Definition: runtime.RawExtension{Raw: []byte(C4)},
	}
	xClaimUnstruct := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(xClaim), &xClaimUnstruct)
	require.NoError(t, err)

	g, err := NewGraph([]v1alpha1.Resource{
		resourceA, resourceB, resourceC, resourceC2, resourceC3, resourceC4,
	})
	require.NoError(t, err)
	g.Claim = Claim{
		Unstructured: &unstructured.Unstructured{
			Object: xClaimUnstruct,
		},
	}

	err = g.TopologicalSort()
	require.NoError(t, err)
	err = g.ResolvedVariables()
	require.NoError(t, err)
	err = g.ReplaceVariables()
	require.NoError(t, err)

	/* 	bb, _ := json.MarshalIndent(g.Resources[3], "", "  ")
	   	t.Log(string(bb)) */

	a, err := g.GetResource("A1")
	require.NoError(t, err)
	require.Equal(t, "a-value", a.Data["spec"].(map[string]interface{})["value"])
	require.Equal(t, "b-value", a.Data["spec"].(map[string]interface{})["staticRefFromB"])

	b, err := g.GetResource("B1")
	require.NoError(t, err)
	require.Equal(t, "b-value", b.Data["spec"].(map[string]interface{})["value"])
	require.Equal(t, "c-value", b.Data["spec"].(map[string]interface{})["staticRefFromC"])

	c, err := g.GetResource("C1")
	require.NoError(t, err)
	require.Equal(t, "c-value", c.Data["spec"].(map[string]interface{})["value"])
	require.Equal(t, "param1-value", c.Data["spec"].(map[string]interface{})["staticRefFromClaim"])

	c2, err := g.GetResource("C2")
	require.NoError(t, err)
	require.Equal(t, "param1-value", c2.Data["spec"].(map[string]interface{})["staticRefFromClaim"])
	require.Equal(t, "param2-value", c2.Data["spec"].(map[string]interface{})["staticRefFromClaim2"])
	require.Equal(t, "a-value", c2.Data["spec"].(map[string]interface{})["staticRefFromA"])
	require.Equal(t, "b-value", c2.Data["spec"].(map[string]interface{})["staticRefFromB"])
	require.Equal(t, "c-value", c2.Data["spec"].(map[string]interface{})["staticRefFromC"])

	c3, err := g.GetResource("C3")
	require.NoError(t, err)
	require.Equal(t, "${A1.status.state}", c3.Data["spec"].(map[string]interface{})["staticStatusRefFromA"])
	require.Equal(t, "${B1.status.state}", c3.Data["spec"].(map[string]interface{})["staticStatusRefFromB"])

	c4, err := g.GetResource("C4")
	require.NoError(t, err)
	require.Equal(t, "${C3.status.state}", c4.Data["spec"].(map[string]interface{})["staticStatusRefFromC3"])

	err = a.SetStatus(map[string]interface{}{
		"state": "a1-state-is-ready",
	})
	require.NoError(t, err)
	err = b.SetStatus(map[string]interface{}{
		"state": "a2-state-is-ready",
	})
	require.NoError(t, err)
	err = c3.SetStatus(map[string]interface{}{
		"state": "c3-state-is-ready",
	})
	require.NoError(t, err)
	hasStatus := c3.HasStatus()
	require.True(t, hasStatus)
	hasStatus = a.HasStatus()
	require.True(t, hasStatus)
	hasStatus = b.HasStatus()
	require.True(t, hasStatus)
	hasStatus = c.HasStatus()
	require.False(t, hasStatus)

	err = g.ResolvedVariables()
	require.NoError(t, err)

	err = g.ReplaceVariables()
	require.NoError(t, err)

	c3, err = g.GetResource("C3")
	require.NoError(t, err)
	require.Equal(t, "a1-state-is-ready", c3.Data["spec"].(map[string]interface{})["staticStatusRefFromA"])
	require.Equal(t, "a2-state-is-ready", c3.Data["spec"].(map[string]interface{})["staticStatusRefFromB"])

	c4, err = g.GetResource("C4")
	/* 	bb, _ := json.MarshalIndent(g.Resources[5], "", "  ")
	   	t.Log(string(bb)) */
	require.NoError(t, err)
	require.Equal(t, "c3-state-is-ready", c4.Data["spec"].(map[string]interface{})["staticStatusRefFromC3"])
	// fmt.Println(c4.Data["spec"].(map[string]interface{})["staticStatusRefFromC3"])
}

var (
	X1 = `
kind: X
metadata:
  name: a-name
spec:
  ref: nothing
`
	Y1 = `
kind: Y
metadata:
  name: a-name
spec:
  ref: nothing
`

	Z1 = `
kind: Z
metadata:
  name: tttname
spec:
  field: ${X.status.status}
  a:
    c:
      bield: ${Y.status.status}`

	yClaim = `
apiVersion: x.symphony.k8s.aws/v1alpha1
kind: XYZClaim
metadata:
  name: xyz-claim
spec:
  name: xyz-claim`
)

func Test_MultiStageVariableReplcements(t *testing.T) {
	resourceX := v1alpha1.Resource{
		Name:       "X",
		Definition: runtime.RawExtension{Raw: []byte(X1)},
	}
	resourceY := v1alpha1.Resource{
		Name:       "Y",
		Definition: runtime.RawExtension{Raw: []byte(Y1)},
	}
	resourceZ1 := v1alpha1.Resource{
		Name:       "Z",
		Definition: runtime.RawExtension{Raw: []byte(Z1)},
	}

	yClaimUnstruct := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yClaim), &yClaimUnstruct)
	require.NoError(t, err)

	g, err := NewGraph([]v1alpha1.Resource{
		resourceZ1, resourceX, resourceY,
	})
	require.NoError(t, err)
	g.Claim = Claim{
		Unstructured: &unstructured.Unstructured{
			Object: yClaimUnstruct,
		},
	}

	err = g.TopologicalSort()
	require.NoError(t, err)
	err = g.ResolvedVariables()
	require.NoError(t, err)
	err = g.ReplaceVariables()
	require.NoError(t, err)

	// g.String()

}
