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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	krov1alpha1 "github.com/kro-run/kro/api/v1alpha1"
	"github.com/kro-run/kro/pkg/testutil/generator"
)

var _ = Describe("Topology", func() {
	var (
		ctx       context.Context
		namespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespace = fmt.Sprintf("test-%s", rand.String(5))
		// Create namespace
		Expect(env.Client.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		})).To(Succeed())
	})

	It("should correctly order AWS resources in dependency graph", func() {
		rgd := generator.NewResourceGraphDefinition("test-topology",
			generator.WithSchema(
				"TestTopology", "v1alpha1",
				map[string]interface{}{
					"name":    "string",
					"version": "string",
				},
				map[string]interface{}{
					"clusterStatus": "${cluster.status.status}",
				},
			),
			// IAM Role - no dependencies
			generator.WithResource("clusterRole", map[string]interface{}{
				"apiVersion": "iam.services.k8s.aws/v1alpha1",
				"kind":       "Role",
				"metadata": map[string]interface{}{
					"name": "test-cluster-role",
				},
				"spec": map[string]interface{}{
					"name": "test-cluster-role",
					"policies": []interface{}{
						"arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
					},
				},
			}, nil, nil),
			// VPC - no dependencies
			generator.WithResource("vpc", map[string]interface{}{
				"apiVersion": "ec2.services.k8s.aws/v1alpha1",
				"kind":       "VPC",
				"metadata": map[string]interface{}{
					"name": "test-vpc",
				},
				"spec": map[string]interface{}{
					"cidrBlocks": []interface{}{
						"192.168.0.0/16",
					},
					"enableDNSHostnames": true,
					"enableDNSSupport":   true,
				},
			}, nil, nil),
			// Subnet A - depends on VPC
			generator.WithResource("subnetA", map[string]interface{}{
				"apiVersion": "ec2.services.k8s.aws/v1alpha1",
				"kind":       "Subnet",
				"metadata": map[string]interface{}{
					"name": "test-subnet-a",
				},
				"spec": map[string]interface{}{
					"availabilityZone": "us-west-2a",
					"cidrBlock":        "192.168.0.0/18",
					"vpcID":            "${vpc.status.vpcID}",
				},
			}, nil, nil),
			// Subnet B - depends on VPC
			generator.WithResource("subnetB", map[string]interface{}{
				"apiVersion": "ec2.services.k8s.aws/v1alpha1",
				"kind":       "Subnet",
				"metadata": map[string]interface{}{
					"name": "test-subnet-b",
				},
				"spec": map[string]interface{}{
					"availabilityZone": "us-west-2b",
					"cidrBlock":        "192.168.64.0/18",
					"vpcID":            "${vpc.status.vpcID}",
				},
			}, nil, nil),
			// Cluster - depends on VPC, Subnets, and IAM Role
			generator.WithResource("cluster", map[string]interface{}{
				"apiVersion": "eks.services.k8s.aws/v1alpha1",
				"kind":       "Cluster",
				"metadata": map[string]interface{}{
					"name": "${schema.spec.name}",
				},
				"spec": map[string]interface{}{
					"name":    "${schema.spec.name}",
					"roleARN": "${clusterRole.status.ackResourceMetadata.arn}",
					"version": "${schema.spec.version}",
					"resourcesVPCConfig": map[string]interface{}{
						"subnetIDs": []interface{}{
							"${subnetA.status.subnetID}",
							"${subnetB.status.subnetID}",
						},
						"endpointPrivateAccess": false,
						"endpointPublicAccess":  true,
					},
				},
			}, nil, nil),
		)

		Expect(env.Client.Create(ctx, rgd)).To(Succeed())

		// Verify ResourceGraphDefinition topology
		Eventually(func(g Gomega) {
			err := env.Client.Get(ctx, types.NamespacedName{
				Name:      rgd.Name,
				Namespace: namespace,
			}, rgd)
			g.Expect(err).ToNot(HaveOccurred())

			// Verify graph is verified
			var graphCondition *krov1alpha1.Condition
			for _, cond := range rgd.Status.Conditions {
				if cond.Type == krov1alpha1.ResourceGraphDefinitionConditionTypeGraphVerified {
					graphCondition = &cond
					break
				}
			}
			g.Expect(graphCondition).ToNot(BeNil())
			g.Expect(graphCondition.Status).To(Equal(metav1.ConditionTrue))

			// Verify topological order
			g.Expect(rgd.Status.TopologicalOrder).To(HaveLen(5))
			g.Expect(rgd.Status.TopologicalOrder).To(Equal([]string{
				"clusterRole",
				"vpc",
				"subnetA",
				"subnetB",
				"cluster",
			}))
		}, 10*time.Second, time.Second).Should(Succeed())
	})

	It("should detect cyclic dependencies in AWS resource definitions", func() {
		rgd := generator.NewResourceGraphDefinition("test-topology-cyclic",
			generator.WithSchema(
				"TestTopologyCyclic", "v1alpha1",
				map[string]interface{}{
					"name": "string",
				},
				nil,
			),
			generator.WithResource("vpc", map[string]interface{}{
				"apiVersion": "ec2.services.k8s.aws/v1alpha1",
				"kind":       "VPC",
				"metadata": map[string]interface{}{
					"name": "${subnet.status.subnetID}", // Creating cyclic dependency
				},
				"spec": map[string]interface{}{
					"cidrBlocks": []interface{}{
						"192.168.0.0/16",
					},
				},
			}, nil, nil),
			generator.WithResource("subnet", map[string]interface{}{
				"apiVersion": "ec2.services.k8s.aws/v1alpha1",
				"kind":       "Subnet",
				"metadata": map[string]interface{}{
					"name": "test-subnet",
				},
				"spec": map[string]interface{}{
					"vpcID":     "${vpc.status.vpcID}", // Creating cyclic dependency
					"cidrBlock": "192.168.1.0/24",
				},
			}, nil, nil),
		)

		Expect(env.Client.Create(ctx, rgd)).To(Succeed())

		Eventually(func(g Gomega) {
			err := env.Client.Get(ctx, types.NamespacedName{
				Name:      rgd.Name,
				Namespace: namespace,
			}, rgd)
			g.Expect(err).ToNot(HaveOccurred())

			var graphCondition *krov1alpha1.Condition
			for _, cond := range rgd.Status.Conditions {
				if cond.Type == krov1alpha1.ResourceGraphDefinitionConditionTypeGraphVerified {
					graphCondition = &cond
					break
				}
			}
			g.Expect(graphCondition).ToNot(BeNil())
			g.Expect(graphCondition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(*graphCondition.Reason).To(ContainSubstring("graph contains a cycle"))
			g.Expect(rgd.Status.State).To(Equal(krov1alpha1.ResourceGraphDefinitionStateInactive))
		}, 10*time.Second, time.Second).Should(Succeed())
	})
})
