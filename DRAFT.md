# notes on extending ACK controller

Maybe start with CRD Wrappers? i want to ensure a certain validation on my CRs?
like annotations and memory < 10 - region etc...

Then we can talk about MultiCRDWrappers? MultiCRDComposers?

Need to talk about ownership and ownership references, meaning that the parent CR
owns the children CR, hence deleting the parent CR should cause the deletion of
the children CRs

Why is this even ACK related? it should be open to be used for different controllers
too.

initially as ACK composer? but moving to aws/symphony

```yaml
apiVersion: extentions.k8s.io/v1
kind: CustomResourceDefinition # CRDBuilder # CRDLinker
metadata:
  name: replicateds3.extentions.k8s.aws
  labels:
    {}
    # Introduce CEL Budget per X ?
spec:
  # maybe use annotation
  reconcile:
    pause: false
    period: 10s # The idea behind hooks! too early to think about?

  name: ReplicatedS3
  extentionSpec:
    primaryRegion: string # what would you do for objects?
    secondaryRegion: string
    acl: string
    objectLockEnabledForBucket: bool
  extentionValidation: CEL # CEL Expression

  # maybe not needed?
  # because the CR will have the extention specs and those will be variables too..
  variables:
    - policy: "{}"
    - namePrefix: replicated-bucket
  resources:
    - name: my-primary-bucket #only used for maping # but could be used to inject in spec stuff
      definition:
        apiVersion: s3.services.k8s.aws
        kind: Bucket
        metadata:
          annotations:
            k8s.aws/region: $spec.primaryRegion
            description: $resource.name
            someweirdCELStuff: $resource[my-primary-bucket].definition.metadata.annotations[someweirdCELStuff] # circular
        spec:
          name: $variables.namePrefix-$ACK_RANDOM_STRING
          policy: $variables.policy
          acl: $spec.acl
          objectLockEnabledForBucket: $spec.objectLockEnabledForBucket
      validation: CEL # CEL Expression
      hooks:
        onCondition:
          condition: ACKSynced=True
          expression: {}
    - resource: { spec: {}, metadata: {}, kind: null, apiVersion: null } # lambda function
      validation: CEL # CEL Expression
      hooks:
        onACKSynced:
          expression: {}
        onAdopted: maybe
  hooks:
    onSubResoucresReady:
      copyToSecret: {} # ?
      waitForAdoptedResource: {} # ?
```
