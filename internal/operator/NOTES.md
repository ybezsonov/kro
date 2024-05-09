### ResourceGroup operator

Operator will receive resourcegroup request creations. It will be responsible of:
- Creating the resourcegroup CRD if it doesn't exist
- Verify that all the mentionned CRDs exist

- Update the resourcegroup CRD status
- Register a new controller watching the resourcegroup CRD

Delegate the claims to the workflow manager
- Create the resourcegroup graph
- Create workflow manager