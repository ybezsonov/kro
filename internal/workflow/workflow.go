package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/symphony/internal/construct"
	"github.com/aws/symphony/internal/requeue"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

// This package transforms the Resource Graph into a Workflow matrix.
// Now we need to know how to deploy, update and delete the resources.

func NewOperator(
	ctx context.Context,
	target schema.GroupVersionResource,
	g *construct.Graph,
	client *dynamic.DynamicClient,
) *Operator {
	log := klog.FromContext(ctx)
	return &Operator{
		id:           fmt.Sprintf("operator.%s/%s/%s", target.Group, target.Version, target.Resource),
		log:          &log,
		target:       target,
		client:       client,
		mainGraph:    g,
		stateGraphs:  make(map[string]*construct.Graph),
		stateTracker: construct.NewStateTracker(g),
	}
}

type Operator struct {
	// mu            sync.RWMutex
	id            string
	log           *klog.Logger
	target        schema.GroupVersionResource
	client        *dynamic.DynamicClient
	mainGraph     *construct.Graph
	stateGraphs   map[string]*construct.Graph
	stateTracker  *construct.StateTracker
	CreateProcess []*Process
	// maybe UpdateProcess []*Process
	DeleteProcess []*Process
}

func (o *Operator) Handler(ctx context.Context, req ctrl.Request) error {
	o.log.Info("Handling", "resource", req.NamespacedName, "operator", o.id)

	o.log.Info("Getting unstructured claim from the api server", "name", req.NamespacedName)
	// stripping the namespace from the name
	parts := strings.Split(req.Name, "/")
	name := parts[len(parts)-1]
	namespace := parts[0]
	fmt.Println("  => using name: ", name)
	fmt.Println("  => using namespace: ", namespace)

	// init client for gvk
	client := o.client.Resource(o.target)
	fmt.Println("  => using gvr: ", o.target)

	// extract claim from request
	claimUnstructured, err := client.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	o.log.Info("Setting claim in graph", "name", req.NamespacedName)
	o.mainGraph.Claim = construct.Claim{Unstructured: claimUnstructured}

	err = o.mainGraph.TopologicalSort()
	if err != nil {
		return err
	}
	err = o.mainGraph.ResolvedVariables()
	if err != nil {
		return err
	}
	err = o.mainGraph.ReplaceVariables()
	if err != nil {
		return err
	}

	fmt.Println("     => starting graph execution")
	for _, resource := range o.mainGraph.Resources {
		fmt.Println("         => resource: ", resource.RuntimeID)
		fmt.Println("             => current state: ", o.stateTracker.GetState(resource.RuntimeID))
		fmt.Println("             => dependencies ready: ", o.stateTracker.ResourceDependenciesReady(resource.RuntimeID))
		if !o.stateTracker.ResourceDependenciesReady(resource.RuntimeID) {
			return requeue.NeededAfter(fmt.Errorf("resource dependencies not ready"), 5)
		}

		rUnstructured := resource.Unstructured()
		rname := rUnstructured.GetName()
		fmt.Println("             => resource name: ", rname)

		gvr := resource.GVR()
		namespace := rUnstructured.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		rc := o.client.Resource(gvr).Namespace(namespace)

		// Check if resource exists
		observed, err := rc.Get(ctx, rname, metav1.GetOptions{})
		fmt.Println("             => getting resource. err", err.Error(), gvr)
		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err := rc.Create(ctx, rUnstructured, metav1.CreateOptions{})
				fmt.Println("             => creating...", err.Error(), gvr)
				b, _ := rUnstructured.MarshalJSON()
				fmt.Println(string(b))
				if err != nil {
					return err
				}
				o.stateTracker.SetState(resource.RuntimeID, construct.ResourceStateCreating)
			} else {
				return err
			}
		}
		if observed != nil {
			observedStatus, ok := observed.Object["status"]
			if ok {
				err := resource.SetStatus(observedStatus.(map[string]interface{}))
				if err != nil {
					return err
				}
				o.stateTracker.SetState(resource.RuntimeID, construct.ResourceStateReady)
				// ...
				err = o.mainGraph.ResolvedVariables()
				if err != nil {
					return err
				}
				err = o.mainGraph.ReplaceVariables()
				if err != nil {
					return err
				}
			}
		}
	}
	if !o.stateTracker.AllReady() {
		return requeue.NeededAfter(fmt.Errorf("not all resources are ready"), 5)
	}

	return nil
}
