# Symphony

### Features:

- Allow any resources to be composed (not necessarily )
- Should allow namespaced composition
- Should allow advanced validation (CEL)
- Dynamic resource duplication (N of X resource + M of Y resource)

- Resoucre references/ crossnamespace references ??

  - Might need to make the controller able to do gradual deployments
  - Reconcilliation might be more complex
    1. C1 => apply X1 then Y1(X1)
    2. f(C1) <=> f(X1)= X2
    3. C1 => apply X2 then Y1(X2)

- metadata.resourceOwnership is mandatory here
- Should we leverage code generation? instead of runtime usage
