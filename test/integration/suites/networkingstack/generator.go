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

package networkingstack_test

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	krov1alpha1 "github.com/kro-run/kro/api/v1alpha1"
	"github.com/kro-run/kro/pkg/testutil/generator"
)

func networkingStack(
	name string,
) (
	*krov1alpha1.ResourceGraphDefinition,
	func(namespace, name string) *unstructured.Unstructured,
) {
	resourcegraphdefinition := generator.NewResourceGraphDefinition(name,
		generator.WithSchema(
			"NetworkingStack", "v1alpha1",
			map[string]interface{}{
				"name": "string",
			},
			map[string]interface{}{
				"networkingInfo": map[string]interface{}{
					"vpcID":         "${vpc.status.vpcID}",
					"subnetAZA":     "${subnetAZA.status.subnetID}",
					"subnetAZB":     "${subnetAZB.status.subnetID}",
					"subnetAZC":     "${subnetAZC.status.subnetID}",
					"securityGroup": "${securityGroup.status.id}",
				},
			},
		),
		generator.WithResource("vpc", vpcDef(), nil, nil),
		generator.WithResource("securityGroup", securityGroupDef(), nil, nil),
		generator.WithResource("subnetAZA", subnetDef("a", "us-west-2a", "192.168.0.0/18"), nil, nil),
		generator.WithResource("subnetAZB", subnetDef("b", "us-west-2b", "192.168.64.0/18"), nil, nil),
		generator.WithResource("subnetAZC", subnetDef("c", "us-west-2c", "192.168.128.0/18"), nil, nil),
	)

	instanceGenerator := func(namespace, name string) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": fmt.Sprintf("%s/%s", krov1alpha1.KRODomainName, "v1alpha1"),
				"kind":       "NetworkingStack",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
				"spec": map[string]interface{}{
					"name": name,
				},
			},
		}
	}
	return resourcegraphdefinition, instanceGenerator
}

func vpcDef() map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "ec2.services.k8s.aws/v1alpha1",
		"kind":       "VPC",
		"metadata": map[string]interface{}{
			"name": "vpc-${schema.spec.name}",
		},
		"spec": map[string]interface{}{
			"cidrBlocks": []interface{}{
				"192.168.0.0/16",
			},
			"enableDNSHostnames": false,
			"enableDNSSupport":   true,
		},
	}
}

func subnetDef(suffix, az, cidr string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "ec2.services.k8s.aws/v1alpha1",
		"kind":       "Subnet",
		"metadata": map[string]interface{}{
			"name": "subnet-" + suffix + "-${schema.spec.name}",
		},
		"spec": map[string]interface{}{
			"availabilityZone": az,
			"cidrBlock":        cidr,
			"vpcID":            "${vpc.status.vpcID}",
		},
	}
}

func securityGroupDef() map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "ec2.services.k8s.aws/v1alpha1",
		"kind":       "SecurityGroup",
		"metadata": map[string]interface{}{
			"name": "security-group-${schema.spec.name}",
		},
		"spec": map[string]interface{}{
			"vpcID":       "${vpc.status.vpcID}",
			"name":        "my-sg-${schema.spec.name}",
			"description": "something something",
		},
	}
}
