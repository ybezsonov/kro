package crd

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
)

// Manager is an object that allows for the management of CRDs
// It is mainly responsible for creating and deleting CRDs
type Manager struct {
	Client *apiextensionsv1.ApiextensionsV1Client
}

func NewManager(Client *apiextensionsv1.ApiextensionsV1Client) *Manager {
	return &Manager{
		Client: Client,
	}
}

func (m *Manager) Create(ctx context.Context, crd v1.CustomResourceDefinition) error {
	crd.OwnerReferences = []metav1.OwnerReference{
		{
			Name:       "symphony-controller",
			Kind:       "Construct",
			APIVersion: "x.symphony.k8s.aws/v1alpha1",
			Controller: &[]bool{false}[0],
			UID:        "00000000-0000-0000-0000-000000000000",
		},
	}

	_, err := m.Client.CustomResourceDefinitions().Create(
		ctx,
		&crd,
		metav1.CreateOptions{},
	)
	return err
}

func (m *Manager) Update(ctx context.Context, crd v1.CustomResourceDefinition) error {
	_, err := m.Client.CustomResourceDefinitions().Update(
		ctx,
		&crd,
		metav1.UpdateOptions{},
	)
	return err
}

func (m *Manager) Ensure(ctx context.Context, crd v1.CustomResourceDefinition) error {
	fmt.Println("Ensuring CRD")
	_, err := m.Describe(ctx, crd.Name)
	if err != nil {
		fmt.Println("Creating CRD")
		if strings.Contains(err.Error(), "not found") {
			return m.Create(ctx, crd)
		}
		return err
	}

	/* 	dc := crd.DeepCopy()
	   	patch := client.MergeFrom(og.DeepCopy())
	   	dt, _ := patch.Data(dc)
	   	_, err = m.Client.CustomResourceDefinitions().Patch(ctx, dc.Name, types.StrategicMergePatchType, dt, metav1.PatchOptions{}) */
	return nil
}

func (m *Manager) Describe(ctx context.Context, name string) (*v1.CustomResourceDefinition, error) {
	return m.Client.CustomResourceDefinitions().Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
}

func (m *Manager) Delete(ctx context.Context, name string) error {
	err := m.Client.CustomResourceDefinitions().Delete(
		ctx,
		name,
		metav1.DeleteOptions{},
	)
	return err
}

func FromOpenAPIV3Schema(apiVersion, kind string, schema *v1.JSONSchemaProps) v1.CustomResourceDefinition {
	return v1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ToLower(kind) + "s.x.symphony.k8s.aws",
		},
		Spec: v1.CustomResourceDefinitionSpec{
			Group: "x.symphony.k8s.aws",
			Names: v1.CustomResourceDefinitionNames{
				Kind:     kind,
				ListKind: kind + "List",
				Plural:   strings.ToLower(kind) + "s",
				Singular: strings.ToLower(kind),
			},
			Scope: v1.NamespaceScoped,
			Versions: []v1.CustomResourceDefinitionVersion{
				{
					Name:    apiVersion,
					Served:  true,
					Storage: true,
					Schema: &v1.CustomResourceValidation{
						OpenAPIV3Schema: schema,
					},
				},
			},
		},
	}
}
