package cel

import (
	"testing"

	"sigs.k8s.io/yaml"
)

const (
	claim = `
apiVersion: x.symphony.k8s.aws/v1alpha1
kind: IRSA
metadata:
  name: my-first-irsa
spec:
  awsAccountID: account-id
  eksOIDC: oidc-name
  permissionsBoundaryArn: arn
  policyArns:
  - arn1
  - arn2
  serviceAccountName: my-service-account
  resourceConfig:
    deletionPolicy: Retain
    region: us-west-2
    tags:
      key: value
`
	resourceA = `apiVersion: iam.services.k8s.aws/v1alpha1
kind: Policy
metadata:
  name: boundary-policy
spec:
  name: boundary-policy
  policyDocument: '{something something}'`

	resourceB = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${spec.serviceAccountName}
  annotations:
    eks.amazonaws.com/role-arn: ${irsa-role.status.ACKResourceMetadata.ARN}`

	resourceC = `
apiVersion: iam.services.k8s.aws/v1alpha1
kind: Role
metadata:
  name: irsa-role
spec:
  name: irsa-role
  PermissionsBoundary: ${boundary-policy.status.ACKResourceMetadata.ARN}
  policies: ${spec.policyARNs}
status:
  ACKResourceMetadata:
    ARN: arn:aws:iam::123456789012:role/irsa-role
`
)

func TestSymphonyEn(t *testing.T) {
	claimUnstruct := make(map[string]interface{})
	yaml.Unmarshal([]byte(claim), &claimUnstruct)

	resourceAUnstruct := make(map[string]interface{})
	yaml.Unmarshal([]byte(resourceA), &resourceAUnstruct)
	resourceBUnstruct := make(map[string]interface{})
	yaml.Unmarshal([]byte(resourceB), &resourceBUnstruct)
	resourceCUnstruct := make(map[string]interface{})
	yaml.Unmarshal([]byte(resourceC), &resourceCUnstruct)

	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	e.SetClaim(claimUnstruct)
	e.SetResource("resourceA", resourceAUnstruct)
	e.SetResource("resourceB", resourceBUnstruct)
	e.SetResource("resourceC", resourceCUnstruct)

	value, err := e.EvalClaim("spec.awsAccountID")
	if err != nil {
		t.Fatal(err)
	}
	if value.Value().(string) != "account-id" {
		t.Fatal(value)
	}
	value, err = e.EvalClaim("spec.policyArns[0]")
	if err != nil {
		t.Fatal(err)
	}
	if value.Value().(string) != "arn1" {
		t.Fatal(value)
	}

	value, err = e.EvalClaim("spec.resourceConfig.tags[\"key\"]")
	if err != nil {
		t.Fatal(err)
	}
	if value.Value().(string) != "value" {
		t.Fatal(value)
	}

	value, err = e.EvalResource("resourceA.spec.policyDocument")
	if err != nil {
		t.Fatal(err)
	}
	if value.Value().(string) != "{something something}" {
		t.Fatal(value)
	}

	value, err = e.EvalResource("resourceB.metadata.annotations[\"eks.amazonaws.com/role-arn\"]")
	if err != nil {
		t.Fatal(err)
	}
	if value.Value().(string) != "${irsa-role.status.ACKResourceMetadata.ARN}" {
		t.Fatal(value)
	}

	value, err = e.EvalResource("resourceC.spec.policies")
	if err != nil {
		t.Fatal(err)
	}
	if value.Value().(string) != "${spec.policyARNs}" {
		t.Fatal(value)
	}

	value, err = e.EvalResource("resourceC.spec.PermissionsBoundary")
	if err != nil {
		t.Fatal(err)
	}
	if value.Value().(string) != "${boundary-policy.status.ACKResourceMetadata.ARN}" {
		t.Fatal(value)
	}

	value, err = e.EvalResource("resourceC.status.ACKResourceMetadata.ARN")
	if err != nil {
		t.Fatal(err)
	}
	if value.Value().(string) != "arn:aws:iam::123456789012:role/irsa-role" {
		t.Fatal(value)
	}
}
