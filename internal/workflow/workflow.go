package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/symphony/api/v1alpha1"
	"github.com/aws/symphony/internal/construct"
	"github.com/aws/symphony/internal/requeue"
)

// This package transforms the Resource Graph into a Workflow matrix.
// Now we need to know how to deploy, update and delete the resources.

func NewOperator(
	ctx context.Context,
	target schema.GroupVersionResource,
	g *construct.Graph,
	client *dynamic.DynamicClient,
) *Operator {
	log := log.FromContext(ctx)
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
	log           *logr.Logger
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
	// init client for gvk
	client := o.client.Resource(o.target)

	// extract claim from request
	claimUnstructured, err := client.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	o.mainGraph.Claim = construct.Claim{Unstructured: claimUnstructured}

	/* err = o.mainGraph.TopologicalSort()
	if err != nil {
		return err
	} */
	err = o.mainGraph.ResolvedVariables()
	if err != nil {
		return err
	}
	err = o.mainGraph.ReplaceVariables()
	if err != nil {
		return err
	}

	if o.mainGraph.Claim.IsStatus("") {
		err = o.patchClaimStatus(ctx, "IN PROGRESS", []v1alpha1.Condition{})
		if err != nil {
			return err
		}
	}

	o.stateTracker.String()

	fmt.Println("     > starting graph execution")
	for i := range o.mainGraph.Resources {
		resource := o.mainGraph.Resources[i]
		fmt.Println("         > resource: ", resource.RuntimeID)
		fmt.Println("             > current state: ", o.stateTracker.GetState(resource.RuntimeID))
		fmt.Println("             > dependencies ready: ", o.stateTracker.ResourceDependenciesReady(resource.RuntimeID))
		if !o.stateTracker.ResourceDependenciesReady(resource.RuntimeID) {
			return requeue.NeededAfter(fmt.Errorf("resource dependencies not ready"), 5)
		}

		gvr := resource.GVR()

		rUnstructured := resource.Unstructured()

		rname := rUnstructured.GetName()
		fmt.Println("             => resource name: ", rname)

		namespace := rUnstructured.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		rc := o.client.Resource(gvr).Namespace(namespace)

		// Check if resource exists
		observed, err := rc.Get(ctx, rname, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				fmt.Println("             => resource not found, creating using", gvr, namespace)
				_, err := rc.Create(ctx, rUnstructured, metav1.CreateOptions{})
				b, _ := rUnstructured.MarshalJSON()
				fmt.Println(string(b))
				if err != nil {
					return err
				}
				fmt.Println("             => resource created")
				fmt.Println("             => setting state to creating")
				o.stateTracker.SetState(resource.RuntimeID, construct.ResourceStateCreating)
				// fmt.Println("             => requeueing")
				// return requeue.NeededAfter(fmt.Errorf("resource created"), 5*time.Second)
			} else {
				return err
			}
		}
		fmt.Println("             => resource found..")
		if resource.IsStatusless() {
			o.stateTracker.SetState(resource.RuntimeID, construct.ResourceStateReady)
		} else if observed != nil {
			observedStatus, ok := observed.Object["status"]
			fmt.Println("             => resource has status", ok)
			if ok {
				err := resource.SetStatus(observedStatus.(map[string]interface{}))
				if err != nil {
					return err
				}
				o.stateTracker.SetState(resource.RuntimeID, construct.ResourceStateReady)

				// o.mainGraph.PrintVariables()
				err = o.mainGraph.ResolvedVariables()
				if err != nil {
					return err
				}
				o.mainGraph.PrintVariables()
				err = o.mainGraph.ReplaceVariables()
				if err != nil {
					return err
				}
				// fmt.Println("			 => raw data: ", resource.Data)
			}

			// fmt.Println("             => resource status set TO READY")
		}
	}
	if !o.stateTracker.AllReady() {
		fmt.Println("     => not all resources are ready")
		return requeue.NeededAfter(fmt.Errorf("not all resources are ready"), 5*time.Second)
	}
	fmt.Println("     => all resources are ready. done")

	o.stateTracker.String()

	msg := "All resources are ready"
	err = o.patchClaimStatus(ctx, "SUCCESS", []v1alpha1.Condition{
		{
			Type:               v1alpha1.ConditionTypeResourceSynced,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: &metav1.Time{Time: time.Now()},
			Reason:             &msg,
			Message:            &msg,
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func (o *Operator) patchClaimStatus(ctx context.Context, state string, conditions []v1alpha1.Condition) error {
	fmt.Println("patching claim status", o.target)
	claim := o.mainGraph.Claim

	s := map[string]interface{}{
		"state":      state,
		"conditions": conditions,
	}
	claim.Object["status"] = s
	claim.Unstructured.Object["status"] = s
	claimUnstructured := claim.Unstructured
	client := o.client.Resource(o.target)
	fmt.Println(claim.Object)
	fmt.Println("GOING FOR IT", claimUnstructured.GetName())
	/* _, err := client.Namespace("default").Get(ctx, claimUnstructured.GetName(), metav1.GetOptions{})
	if err != nil {
		return err
	} */
	fmt.Println("GOT IT")

	b, _ := json.Marshal(claim.Unstructured)
	_, err := client.Namespace("default").Patch(ctx, claimUnstructured.GetName(), types.MergePatchType, b, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	return nil
}
