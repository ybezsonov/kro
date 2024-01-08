package construct

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/symphony/api/v1alpha1"
	"github.com/google/cel-go/common/types"
	"sigs.k8s.io/yaml"
)

type Graph struct {
	Claim     Claim
	Resources []*Resource
}

func (g *Graph) CopyWithNewClaim(claim Claim) *Graph {
	return &Graph{
		Claim:     claim,
		Resources: copyResources(g.Resources),
	}
}

func (g *Graph) GetResource(runtimeID string) (*Resource, error) {
	for _, r := range g.Resources {
		if r.RuntimeID == runtimeID {
			return r, nil
		}
	}
	return nil, fmt.Errorf("resource not found")
}

func (g *Graph) OrderedResourceList() []string {
	var list []string
	for _, r := range g.Resources {
		list = append(list, r.RuntimeID)
	}
	return list
}

func NewGraph(constructResources []v1alpha1.Resource) (*Graph, error) {
	// Start by walking through the resources and build a map of resources.
	// This map will be used to quickly access a resource by its name.
	resources := make([]*Resource, 0, len(constructResources))
	for _, r := range constructResources {
		resource, err := NewResourceFromRaw(r.Name, r.Definition.Raw)
		if err != nil {
			return nil, fmt.Errorf("couldn't build resource: %s: %v", r.Name, err)
		}
		resources = append(resources, resource)
	}

	// Validate that names are not duplicated.
	err := validateNamesDuplication(resources)
	if err != nil {
		return nil, err
	}

	// So far we have a map of resources, but we don't know the relationships between them.
	// We need to walk over the variables of each resource and find the relationships.

	for _, resource := range resources {
		for _, vrbl := range resource.Variables {
			// for now we only support reference variables
			// todo more logic here...
			switch {
			case strings.HasPrefix(vrbl.Expression, "spec"),
				strings.HasPrefix(vrbl.Expression, "status"),
				strings.HasPrefix(vrbl.Expression, "metadata"):
				// claim reference

			default:
				rsc, variableKind, ok := getVariableResourceRef(vrbl.Expression, resources)
				if !ok {
					return nil, fmt.Errorf("unknown variable/reference: %v : %v", vrbl, resource.RuntimeID)
				}

				if string(variableKind) == string(VariableKindResourceStatusReference) {
					resource.AddDependency(rsc.RuntimeID)
					rsc.AddChildren(resource.RuntimeID)
					vrbl.SrcRef = &ResourceRef{RuntimeID: rsc.RuntimeID}
				}

				vrbl.Kind = variableKind
				vrbl.Type = VariableTypeStaticReference
			}
		}
	}
	// Validate that there are no cyclic dependencies.
	for _, resource := range resources {
		err := detectCyclicDependencies(resource, resources, make(map[string]bool), resource.RuntimeID)
		if err != nil {
			return nil, err
		}
	}

	return &Graph{
		Resources: resources,
	}, nil
}

func (g *Graph) topologicalSort() ([]*Resource, error) {
	// Create a map to store the in-degree (number of incoming edges) for each resource
	inDegree := make(map[string]int)

	// Populate in-degree map
	for _, resource := range g.Resources {
		inDegree[resource.RuntimeID] = 0
	}

	for _, resource := range g.Resources {
		for _, dep := range resource.Dependencies {
			inDegree[dep.RuntimeID]++
		}
	}

	queue := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Perform topological sorting
	var result []*Resource
	for len(queue) > 0 {
		// Pop a resource from the queue
		currentID := queue[0]
		queue = queue[1:]

		// Find the resource with the currentID
		var currentResource *Resource
		for _, resource := range g.Resources {
			if resource.RuntimeID == currentID {
				currentResource = resource
				break
			}
		}

		// Add the current resource to the result
		result = append(result, currentResource)

		// Update in-degrees and enqueue dependencies with in-degree 0
		for _, depRef := range currentResource.Dependencies {
			depID := depRef.RuntimeID
			inDegree[depID]--

			if inDegree[depID] == 0 {
				queue = append(queue, depID)
			}
		}
	}

	// Check if there is a cycle
	if len(result) != len(g.Resources) {
		return nil, errors.New("dependency cycle detected")
	}

	// revert the order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// Parse the resources and determines the order in which they should be created/managed.
func (g *Graph) getCreationOrder() ([]*Resource, error) {
	orderedResources := make([]*Resource, 0, len(g.Resources))
	appended := make(map[string]bool)
	copyResources := copyResources(g.Resources)

	for len(orderedResources) < len(copyResources) {
		for _, resource := range copyResources {
			if appended[resource.RuntimeID] {
				continue
			}

			if resource.HasNDependencies(0) {
				orderedResources = append(orderedResources, resource)
				appended[resource.RuntimeID] = true
				continue
			}
			for _, dependency := range resource.Dependencies {
				for _, lookupR := range copyResources {
					if lookupR.RuntimeID == resource.RuntimeID {
						continue
					}
					if lookupR.RuntimeID == dependency.RuntimeID && appended[lookupR.RuntimeID] {
						lookupR.RemoveChildren(resource.RuntimeID)
						resource.RemoveDependency(dependency.RuntimeID)
						break
					}
				}
			}
		}
	}

	return orderedResources, nil
}

func getResourceByName(runtimeID string, resources []*Resource) (*Resource, error) {
	for _, r := range resources {
		if r.RuntimeID == runtimeID {
			return r, nil
		}
	}
	return nil, fmt.Errorf("resource not found")
}

func getVariableResourceRef(variable string, resources []*Resource) (*Resource, VariableKind, bool) {
	variable = trimReferenceSyntax(variable)

	parts := strings.Split(variable, ".")
	if len(parts) < 2 {
		return nil, VariableKindUnknown, false
	}
	identifier := parts[0]
	if identifier == "spec" {
		return nil, VariableKindClaimSpecRefrence, true
	}
	if identifier == "status" {
		return nil, VariableKindClaimStatusRefrence, true
	}

	for _, r := range resources {
		if r.RuntimeID == identifier {
			if parts[1] == "status" {
				return r, VariableKindResourceStatusReference, true
			}
			return r, VariableKindResourceSpecReference, true
		}
	}
	return nil, VariableKindUnknown, false
}

// detectCyclicDependencies is a recursive function that detects cyclic dependencies between resources.
func detectCyclicDependencies(resource *Resource, resources []*Resource, seen map[string]bool, path string) error {
	seen[resource.RuntimeID] = true
	for _, dependency := range resource.Dependencies {
		if seen[dependency.RuntimeID] {
			return fmt.Errorf("circular dependency detected: %s", path)
		}
		for _, r := range resources {
			if r.RuntimeID == dependency.RuntimeID {
				err := detectCyclicDependencies(r, resources, seen, path+" -> "+r.RuntimeID)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Takes a claim and returns a list of resources with resolved variables.
// The variables are resolved by looking at the claim's spec and other resources spec.
func (g *Graph) GetResourcesWithResolvedStaticVariables(claim Claim) ([]*Resource, error) {
	resolver := NewResolver(g)
	for _, resource := range g.Resources {
		fmt.Println(".... resource main has status: ", resource.RuntimeID, resource.HasStatus())
	}

	copyResources := copyResources(g.Resources)
	// list of resources that have a status
	for _, resource := range copyResources {
		fmt.Println(".... resource copy has status: ", resource.RuntimeID, resource.HasStatus())
	}

	for _, resource := range copyResources {
		for _, variable := range resource.Variables {
			if variable.Type == VariableTypeStaticReference {
				trimedExpression := trimReferenceSyntax(variable.Expression)
				switch variable.Kind {
				case VariableKindClaimSpecRefrence:
					variableValue, err := resolver.ResolverFromClaim(trimedExpression)
					if err != nil {
						return nil, fmt.Errorf("couldn't resolve claim variable: '%v': '%v'", trimedExpression, err)
					}
					variable.ResolvedValue = variableValue
				case VariableKindResourceSpecReference:
					variableValue, err := resolver.ResolverFromResource(trimedExpression, resource)
					if err != nil {
						return nil, fmt.Errorf("couldn't resolve resource variable: %v: %v", trimedExpression, err)
					}
					variable.ResolvedValue = variableValue

				case VariableKindResourceStatusReference:
					fmt.Println("++ resolving status variable: ", trimedExpression, "from", resource.RuntimeID)
					targetResource, err := g.GetResource(variable.SrcRef.RuntimeID)
					if err != nil {
						return nil, fmt.Errorf("!!! couldn't resolve resource variable: %v: %v", trimedExpression, err)
					}
					fmt.Println("++ target has statuss: ", targetResource.RuntimeID, " => ", targetResource.HasStatus())
					if !targetResource.HasStatus() {
						fmt.Println("++ target has no statuss: ", targetResource.RuntimeID, " => ", targetResource.Data)
					}
					if targetResource.HasStatus() {
						variableValue, err := resolver.ResolverFromResource(trimedExpression, resource)
						if err != nil {
							return nil, fmt.Errorf("couldn't resolve resource variable: %v: %v", trimedExpression, err)
						}
						variable.ResolvedValue = variableValue
					}
				}
			}
		}
	}

	return copyResources, nil
}

func (r *Resource) ApplyResolvedVariables() error {
	vars := make(map[string]string)
	for _, variable := range r.Variables {
		if variable.Type == VariableTypeStaticReference {
			if variable.ResolvedValue != nil {
				switch v := variable.ResolvedValue.(type) {
				case string:
					vars[variable.Expression] = v
				case types.String:
					vars[variable.Expression] = v.Value().(string)
				default:
					return fmt.Errorf("unknown variable type: %v: type=%v", variable.ResolvedValue, v)
				}
			}
		}
	}

	r.Raw = r.replaceVariables(vars)
	var newData map[string]interface{}
	err := yaml.Unmarshal(r.Raw, &newData)
	if err != nil {
		return err
	}
	r.Data = newData

	return nil
}

func (g *Graph) TopologicalSort() error {
	orderedResources, err := g.topologicalSort()
	if err != nil {
		return err
	}
	g.Resources = orderedResources
	return nil
}

func (g *Graph) ResolvedVariables() error {
	resources, err := g.GetResourcesWithResolvedStaticVariables(g.Claim)
	if err != nil {
		return err
	}
	g.Resources = resources
	return nil
}

func (g *Graph) PrintVariables() {
	for _, resource := range g.Resources {
		fmt.Println("Resource: ", resource.RuntimeID)
		for _, variable := range resource.Variables {
			fmt.Println("::", variable.Expression, " => ", variable.ResolvedValue)
		}
	}
}

func (g *Graph) ReplaceVariables() error {
	for _, resource := range g.Resources {
		if err := resource.ApplyResolvedVariables(); err != nil {
			return err
		}
	}
	return nil
}

func (g *Graph) String() {
	fmt.Println("---")
	for _, r := range g.Resources {
		fmt.Println(string(r.Raw))
		fmt.Println("---")
	}
}
