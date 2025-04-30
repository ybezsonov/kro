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

package core_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	krov1alpha1 "github.com/kro-run/kro/api/v1alpha1"
	"github.com/kro-run/kro/pkg/testutil/generator"
)

var _ = Describe("Recovery", func() {
	var (
		ctx       context.Context
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = fmt.Sprintf("test-%s", rand.String(5))
		// Create namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		Expect(env.Client.Create(ctx, ns)).To(Succeed())
	})

	It("should recover from invalid state and use latest valid configuration", func() {
		// Create initial valid ResourceGraphDefinition
		rgd := generator.NewResourceGraphDefinition("test-recovery",
			generator.WithSchema(
				"TestRecovery", "v1alpha1",
				map[string]interface{}{
					"name":      "string",
					"configKey": "string",
				},
				nil,
			),
			generator.WithResource("initialConfig", map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": "${schema.spec.name}",
				},
				"data": map[string]interface{}{
					"key":     "${schema.spec.configKey}",
					"version": "initial",
				},
			}, nil, nil),
		)

		// Create ResourceGraphDefinition
		Expect(env.Client.Create(ctx, rgd)).To(Succeed())

		// Verify initial ResourceGraphDefinition becomes active
		Eventually(func(g Gomega) {
			err := env.Client.Get(ctx, types.NamespacedName{
				Name: rgd.Name,
			}, rgd)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(rgd.Status.State).To(Equal(krov1alpha1.ResourceGraphDefinitionStateActive))
		}, 10*time.Second, time.Second).Should(Succeed())

		// Update to invalid state with a cyclic dependency
		Eventually(func(g Gomega) {
			err := env.Client.Get(ctx, types.NamespacedName{
				Name: rgd.Name,
			}, rgd)
			g.Expect(err).ToNot(HaveOccurred())

			// Add resources with circular dependency
			rgd.Spec.Resources = append(rgd.Spec.Resources,
				&krov1alpha1.Resource{
					ID: "serviceA",
					Template: toRawExtension(map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
						"metadata": map[string]interface{}{
							"name": "${serviceB.metadata.name}",
						},
					}),
				},
				&krov1alpha1.Resource{
					ID: "serviceB",
					Template: toRawExtension(map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
						"metadata": map[string]interface{}{
							"name": "${serviceA.metadata.name}",
						},
					}),
				},
			)

			err = env.Client.Update(ctx, rgd)
			g.Expect(err).ToNot(HaveOccurred())
		}, 10*time.Second, time.Second).Should(Succeed())

		// Verify ResourceGraphDefinition becomes inactive
		Eventually(func(g Gomega) {
			err := env.Client.Get(ctx, types.NamespacedName{
				Name: rgd.Name,
			}, rgd)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(rgd.Status.State).To(Equal(krov1alpha1.ResourceGraphDefinitionStateInactive))
		}, 10*time.Second, time.Second).Should(Succeed())

		// Update to new valid state with different configuration
		Eventually(func(g Gomega) {
			err := env.Client.Get(ctx, types.NamespacedName{
				Name: rgd.Name,
			}, rgd)
			g.Expect(err).ToNot(HaveOccurred())

			// Replace with new valid resource
			rgd.Spec.Resources = []*krov1alpha1.Resource{
				{
					ID: "itsapodnow",
					Template: toRawExtension(map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name": "${schema.spec.name}",
						},
						"spec": map[string]interface{}{
							"replicas": 1,
							"selector": map[string]interface{}{
								"matchLabels": map[string]interface{}{
									"app": "deployment",
								},
							},
							"template": map[string]interface{}{
								"metadata": map[string]interface{}{
									"labels": map[string]interface{}{
										"app": "deployment",
									},
								},
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name":  "${schema.spec.name}-deployment",
											"image": "nginx",
											"ports": []interface{}{
												map[string]interface{}{
													"containerPort": 777,
												},
											},
										},
									},
								},
							},
						},
					}),
				},
			}

			err = env.Client.Update(ctx, rgd)
			g.Expect(err).ToNot(HaveOccurred())
		}, 10*time.Second, time.Second).Should(Succeed())

		// Verify ResourceGraphDefinition becomes active again
		Eventually(func(g Gomega) {
			err := env.Client.Get(ctx, types.NamespacedName{
				Name: rgd.Name,
			}, rgd)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(rgd.Status.State).To(Equal(krov1alpha1.ResourceGraphDefinitionStateActive))
		}, 10*time.Second, time.Second).Should(Succeed())

		// Create instance
		name := "test-recovery"
		instance := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": fmt.Sprintf("%s/%s", krov1alpha1.KRODomainName, "v1alpha1"),
				"kind":       "TestRecovery",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
				"spec": map[string]interface{}{
					"name":      name,
					"configKey": "testKey",
				},
			},
		}
		Expect(env.Client.Create(ctx, instance)).To(Succeed())

		// Verify instance created Deployment with updated configuration
		Eventually(func(g Gomega) {
			deploy := &appsv1.Deployment{}
			err := env.Client.Get(ctx, types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			}, deploy)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(deploy.Spec.Template.Spec.Containers[0].Image).To(Equal("nginx"))
			g.Expect(deploy.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(777)))

		}, 20*time.Second, time.Second).Should(Succeed())

		// Cleanup
		// Delete instance
		Expect(env.Client.Delete(ctx, instance)).To(Succeed())

		// Verify instance is deleted
		Eventually(func() bool {
			err := env.Client.Get(ctx, types.NamespacedName{
				Name:      name,
				Namespace: namespace,
			}, instance)
			return errors.IsNotFound(err)
		}, 20*time.Second, time.Second).Should(BeTrue())

		// Delete ResourceGraphDefinition
		Expect(env.Client.Delete(ctx, rgd)).To(Succeed())

		// Verify ResourceGraphDefinition is deleted
		Eventually(func() bool {
			err := env.Client.Get(ctx, types.NamespacedName{
				Name: rgd.Name,
			}, &krov1alpha1.ResourceGraphDefinition{})
			return errors.IsNotFound(err)
		}, 20*time.Second, time.Second).Should(BeTrue())
	})
})
