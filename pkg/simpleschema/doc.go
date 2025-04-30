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

package simpleschema

// The main purpose of this package is to provide a new schema for defining
// Custom Resource Definitions (CRDs) in Kubernetes. This schema is used to
// define the structure of the CRD and the validation rules that are applied to
// the CRD.
//
// While a few things are hard-coded right now, the goal is to make this schema
// as flexible as possible so that it can be used to define any CRD in other
// projects.
//
// Example
//
// Here is an example of how to use this schema to define a CRD:
//
//   variables:
//     spec:
//       name: string | required=true description="The name of the resource"
//       count: int | default=3
//       enabled: bool | default=true
//       tags: map[string]string
//     status:
//       conditions: []condition | required=false
//   extraTypes:
//     condition:
//       type: string
//       status: bool
//       reason: string
//       message: string
//       lastTransitionTime: string
//
// In kro you might see us using CEL expressions to define instructions
// for patch back status fields to CRD instances. This is not part of the schema
// standard it self but it is a kro specific extension. For example
//
//  variables:
//    spec:
//      name: string
//    status:
//      conditions: ${deployment.status.conditions}
