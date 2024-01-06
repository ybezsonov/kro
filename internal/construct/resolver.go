package construct

import (
	"github.com/aws/symphony/internal/cel"
)

type _ interface {
	ResolveClain(expression string) (any, error)
	ResolveResource(expression string) (any, error)
}

type Resolver struct {
	claimCache     map[string]interface{}
	resourcesCache map[string]map[string]interface{}

	CELEngine *cel.SymphonyEngine
}

func NewResolver(g *Graph) *Resolver {
	celEngine, err := cel.NewEngine()
	if err != nil {
		panic(err)
	}
	celEngine.SetClaim(g.Claim.Object)
	for _, r := range g.Resources {
		celEngine.SetResource(r.RuntimeID, r.Data)
	}

	return &Resolver{
		claimCache:     make(map[string]interface{}),
		resourcesCache: make(map[string]map[string]interface{}),
		CELEngine:      celEngine,
	}
}

func (r *Resolver) getResourceCache(runtimeID string) map[string]interface{} {
	cache, ok := r.resourcesCache[runtimeID]
	if ok {
		return cache
	}
	cache = make(map[string]interface{})
	r.resourcesCache[runtimeID] = cache
	return cache
}

func (r *Resolver) ResolverFromClaim(expression string) (interface{}, error) {
	inCache, ok := r.claimCache[expression]
	if ok {
		return inCache, nil
	}
	resp, err := r.CELEngine.EvalClaim(expression)
	if err != nil {
		return "", err
	}
	r.claimCache[expression] = resp
	return resp.Value(), nil
}

func (r *Resolver) ResolverFromResource(expression string, resource *Resource) (interface{}, error) {
	resourceCache := r.getResourceCache(resource.RuntimeID)
	inCache, ok := resourceCache[expression]
	if ok {
		return inCache, nil
	}
	resp, err := r.CELEngine.EvalResource(expression)
	if err != nil {
		return "", err
	}
	resourceCache[expression] = resp
	return resp.Value(), nil
}
