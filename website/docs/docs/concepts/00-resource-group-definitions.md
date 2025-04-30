---
sidebar_position: 1
---

# ResourceGraphDefinitions

ResourceGraphDefinitions are the fundamental building blocks in **kro**. They provide a
way to define, organize, and manage sets of related Kubernetes resources as a
single, reusable unit.

## What is a ResourceGraphDefinition?

A **ResourceGraphDefinition** is a custom resource that lets you create new Kubernetes
APIs for deploying multiple resources together. It acts as a blueprint,
defining:

- What users can configure (schema)
- What resources to create (resources)
- How resources reference each other (dependencies)
- When resources should be included (conditions)
- What status to expose (status)

When you create a **ResourceGraphDefinition**, kro generates a new API (a.k.a Custom
Resource Defintion) in your cluster that others can use to deploy resources in a
consistent, controlled way.

## Anatomy of a ResourceGraphDefinition

A ResourceGraphDefinition, like any Kubernetes resource, consists of three main parts:

1. **Metadata**: name, namespace, labels, etc.
2. **Spec**: Defines the structure and properties of the ResourceGraphDefinition
3. **Status**: Reflects the current state of the ResourceGraphDefinition

The `spec` section of a ResourceGraphDefinition contains two main components:

- **Schema**: Defines what an instance of your API looks like:
  - What users can configure during creation and update
  - What status information they can view
  - Default values and validation rules
- **Resources**: Specifies the Kubernetes resources to create:
  - Resource templates
  - Dependencies between resources
  - Conditions for inclusion
  - Readiness criteria

This structure translates to YAML as follows:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: my-resourcegraphdefinition # Metadata section
spec:
  schema: # Define your API
    apiVersion: v1alpha1 # API version
    kind: MyAPI # API kind
    spec: {} # fields users can configure
    status: {} # fields kro will populate

  # Define the resources kro will manage
  resources:
    - id: resource1
      # declare your resources along with default values and variables
      template: {}
```

Let's look at each component in detail...

## Understanding the Schema

The schema section defines your new API's structure. It determines:

- What fields users can configure when creating instances
- What status information they can view
- Type validation and default values

Here's an example schema:

```yaml
schema:
  apiVersion: v1alpha1
  kind: WebApplication # This becomes your new API type
  spec:
    # Fields users can configure using a simple, straightforward syntax
    name: string
    image: string | default="nginx"
    replicas: integer | default=3
    ingress:
      enabled: boolean | default=false

  status:
    # Fields kro will populate automatically from your resources
    # Types are inferred from these CEL expressions
    availableReplicas: ${deployment.status.availableReplicas}
    conditions: ${deployment.status.conditions}

  validation:
    # Validating admission policies added to the new API type's CRD
    - expression: "${ self.image == 'nginx' || !self.ingress.enabled }"
      message: "Only nginx based applications can have ingress enabled"
```

**kro** follows a different approach for defining your API schema and shapes. It
leverages a human-friendly and readable syntax that is OpenAPI spec compatible.
No need to write complex OpenAPI schemas - just define your fields and types in
a straightforward way. For the complete specification of this format, check out
the [Simple Schema specification](./10-simple-schema.md). Status fields use CEL
expressions to reference fields from resources defined in your ResourceGraphDefinition.
kro automatically:

- Infers the correct types from your expressions
- Validates that referenced resources exist
- Updates these fields as your resources change

## ResourceGraphDefinition Processing

When you create a **ResourceGraphDefinition**, kro processes it in several steps to ensure
correctness and set up the necessary components:

1. **Validation**: kro validates your **ResourceGraphDefinition** to ensure it's well
   formed and follows the correct syntax, maximizing the chances of successful
   deployment, and catching as many errors as possible early on. It:

   - Validates your schema definition follows the simple schema format
   - Ensures all resource templates are valid Kubernetes manifests
   - Checks that referenced values exist and are of the correct type
   - Confirms resource dependencies form a valid Directed Acycled Graph(DAG)
     without cycles
   - Validates all CEL expressions in status fields and conditions

2. **API Generation**: kro generates and registers a new CRD in your cluster
   based on your schema. For example, if your **ResourceGraphDefinition** defines a
   `WebApplication` API, kro creates a CRD that:

   - Provides API validation based on your schema definition
   - Automatically applies default values you've defined
   - Makes status information available to users and other systems
   - Integrates seamlessly with kubectl and other Kubernetes tools

3. **Controller Configuration**: kro configures itself to watch for instances of
   your new API and:

   - Creates all required resources following the dependency order
   - Manages references and value passing between resources
   - Handles the complete lifecycle for create, update, and delete operations
   - Keeps status information up to date based on actual resource states

For instance, when you create a `WebApplication` ResourceGraphDefinition, kro generates
the `webapplications.kro.run` CRD. When users create instances of this API, kro
manages all the underlying resources (Deployments, Services, Custom Resources,
etc.) automatically.

kro continuously monitors your ResourceGraphDefinition for changes, updating the API and
its behavior accordingly.

## ResourceGraphDefinition Instance Example

After the **ResourceGraphDefinition** is validated and registered in the cluster, users
can can create instances of it. Here's an example of how an instance for the
`WebApplication` might look:

```yaml title="my-web-app-instance.yaml"
apiVersion: kro.run/v1alpha1
kind: WebApplication
metadata:
  name: my-web-app
spec:
  appName: awesome-app
  image: nginx:latest
  replicas: 3
```
