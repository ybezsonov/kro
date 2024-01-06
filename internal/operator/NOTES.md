### Construct operator

Operator will receive construct request creations. It will be responsible of:
- Creating the construct CRD if it doesn't exist
- Verify that all the mentionned CRDs exist

- Update the construct CRD status
- Register a new controller watching the construct CRD

Delegate the claims to the workflow manager
- Create the construct graph
- Create workflow manager