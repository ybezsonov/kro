package dynamiccontroller

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/aws/symphony/api/v1alpha1"
	"github.com/aws/symphony/internal/requeue"
	"github.com/aws/symphony/internal/resourcegroup"
	"github.com/aws/symphony/internal/workflow"
	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type DynamicController struct {
	// name is an identifier for this particular controller instance. Useless but I like naming things.
	name string
	// kubeClient is a dynamic client to the Kubernetes cluster.
	kubeClient *dynamic.DynamicClient
	// synced informs if the controller is synced with the apiserver.
	// This is an aggregation of all the InformerSynced.
	synced cache.InformerSynced
	// workflowOperators is a map of the registered workflow operators
	workflowOperators map[schema.GroupVersionResource]*workflow.Operator
	// handler is the function that will be called when a new work item is added
	// to the queue. The argument to the handler is an interface that should be
	// castable to the appropriate type.
	//
	// Note(a-hilaly) maybe unstructured.Unstructured is a better choice here.
	handler func(context.Context, ctrl.Request) error

	// queue is where incoming work is placed to de-dup and to allow "easy"
	// rate limited requeues.
	//
	// Might need multiple queues? one for each registered resource group? How about priority queues.
	queue workqueue.RateLimitingInterface
	// informers is a the map of the registered informers
	informers   map[schema.GroupVersionResource]dynamicinformer.DynamicSharedInformerFactory
	cancelFuncs map[schema.GroupVersionResource]context.CancelFunc
	// Protects access to the informers map. Could have been a sync.Map but we need to
	// optimize for the read case.
	mu sync.RWMutex
	// listers is a map of the registered listers
	listers map[schema.GroupVersionResource]dynamiclister.Lister

	log *logr.Logger
}

func NewDynamicController(
	ctx context.Context,
	name string,
	kubeClient *dynamic.DynamicClient,
	handler func(context.Context, ctrl.Request) error,
) *DynamicController {
	logger := log.FromContext(ctx)
	// wo := workflow.NewOperator(schema.GroupVersionResource{}, nil, nil)

	dc := &DynamicController{
		name:       name,
		kubeClient: kubeClient,
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
			// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		), "dynamic-controller-queue"),
		handler:           handler,
		informers:         map[schema.GroupVersionResource]dynamicinformer.DynamicSharedInformerFactory{},
		cancelFuncs:       map[schema.GroupVersionResource]context.CancelFunc{},
		log:               &logger,
		mu:                sync.RWMutex{},
		listers:           map[schema.GroupVersionResource]dynamiclister.Lister{},
		workflowOperators: map[schema.GroupVersionResource]*workflow.Operator{},
	}
	return dc
}

// Run the main goroutine responsible for watching and syncing jobs.
func (cc *DynamicController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer cc.queue.ShutDown()

	logger := log.FromContext(ctx)
	logger.Info("Starting symphony dynamic controller", "name", cc.name)
	defer logger.Info("Shutting symphony dynamic controller", "name", cc.name)

	/* if !cache.WaitForNamedCacheSync(cc.name, ctx.Done(), cc.synced) {
		return
	} */

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, cc.worker, time.Second)
	}

	<-ctx.Done()
}

// worker runs a thread that dequeues CSRs, handles them, and marks them done.
func (cc *DynamicController) worker(ctx context.Context) {
	for cc.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (cc *DynamicController) processNextWorkItem(ctx context.Context) bool {
	item, quit := cc.queue.Get()
	if quit {
		return false
	}
	defer cc.queue.Done(item)

	itemUnwrapper := item.(ObjectIdentifiers)
	err := cc.syncFunc(ctx, itemUnwrapper)
	fmt.Println("    => DC syncFunc err", err)
	if err != nil {
		if reqErr, ok := err.(*requeue.RequeueNeededAfter); ok {
			cc.queue.AddAfter(item, reqErr.Duration())
		} else {
			gvrKey := fmt.Sprintf("%s/%s/%s/%s", itemUnwrapper.GVR.Group, itemUnwrapper.GVR.Version, itemUnwrapper.GVR.Resource, itemUnwrapper.Key)
			utilruntime.HandleError(fmt.Errorf("sync %v failed with : %v", gvrKey, err))
		}
		return true
	}

	fmt.Println("    => forgetting item", itemUnwrapper.Key)
	cc.queue.Forget(item)
	return true
}

type ObjectIdentifiers struct {
	Key string
	GVR schema.GroupVersionResource
}

func (cc *DynamicController) enqueueObject(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	// obj is a claim of a defined resourcegroup, we do not know much about the contruct
	// so we enqueue two things:
	//   - the claim key (namespacedName)
	//   - the GVR of the claim (this will be usefull to know which handler to call)
	//
	// The reason we have so many handlers is because those handlers are compiled graphs
	// with a set of workflow steps that are specific to the resourcegroup.

	// Since we are using a dynamic informer/client, it is guaranteed that the object is
	// a pointer to unstructured.Unstructured.
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("object is not of type unstructured.Unstructured"))
		return
	}

	// extract group and version from apiVersion
	apiVersion := u.GetAPIVersion()
	parts := strings.Split(apiVersion, "/")
	if len(parts) != 2 {
		utilruntime.HandleError(fmt.Errorf("invalid apiVersion: %s", apiVersion))
		return
	}
	group := parts[0]
	version := parts[1]

	objectIdentifiers := ObjectIdentifiers{
		Key: key,
		GVR: schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: strings.ToLower(u.GetKind()) + "s",
		},
	}

	cc.queue.Add(objectIdentifiers)
}

func (cc *DynamicController) syncFunc(ctx context.Context, oi ObjectIdentifiers) error {
	logger := log.FromContext(ctx)
	startTime := time.Now()
	defer func() {
		logger.Info("Finished syncing resourcegroup claim request", "elapsedTime", time.Since(startTime))
	}()

	fmt.Println("====================================")
	for gvk := range cc.workflowOperators {
		fmt.Println("    => DC workflow operator exist", gvk)
		fmt.Println("    => You are looking for oi.GVR", oi.GVR)
	}

	wo, ok := cc.workflowOperators[oi.GVR]
	if !ok {
		return fmt.Errorf("no workflow operator found for GVR: %s", oi.GVR)
	}
	return wo.Handler(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: oi.Key}})
}

func (dc *DynamicController) Shutdown() {
	dc.queue.ShutDown()
	/* for _, informer := range dc.informers {
		informer.WaitForCacheSync(make(chan struct{}))
	} */
	for _, informer := range dc.informers {
		informer.Shutdown()
	}
}

// RegisterGVK registers a new GVK to the informers map aggressively.
func (dc *DynamicController) RegisterGVK(gvr schema.GroupVersionResource) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	gvkInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc.kubeClient, 0, metav1.NamespaceAll, nil)
	dc.informers[gvr] = gvkInformer
	dc.log.Info("Finished registering GVK", "gvk", gvr)
}

// SafeRegisterGVK registers a new GVK to the informers map safely.
func (dc *DynamicController) SafeRegisterGVK(gvr schema.GroupVersionResource) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if _, ok := dc.informers[gvr]; !ok {
		gvkInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc.kubeClient, 0, metav1.NamespaceAll, nil)
		gvkInformer.ForResource(gvr).Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				dc.log.Info("Adding object")
				dc.enqueueObject(obj)
			},
			UpdateFunc: func(old, new interface{}) {
				// marshall old and new to json and compare them
				oldJSON, _ := old.(*unstructured.Unstructured).MarshalJSON()
				newJSON, _ := new.(*unstructured.Unstructured).MarshalJSON()
				if string(oldJSON) == string(newJSON) {
					fmt.Println("WEIRD: old and new are the same")
					return
				}

				dc.log.Info("Updating object")
				dc.enqueueObject(new)
			},
			DeleteFunc: func(obj interface{}) {
				uu, ok := obj.(*unstructured.Unstructured)
				if !ok {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						dc.log.V(2).Info("Couldn't get object from tombstone", "object", obj)
						return
					}
					uu, ok = tombstone.Obj.(*unstructured.Unstructured)
					if !ok {
						dc.log.V(2).Info("Tombstone contained object that is not an unstructured obj", "object", obj)
						return
					}
				}
				dc.log.Info("Deleting object")
				dc.enqueueObject(uu)
			},
		})
		dc.informers[gvr] = gvkInformer
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		dc.cancelFuncs[gvr] = cancel

		time.Sleep(1 * time.Second)
		dc.log.Info("Starting informer", "gvr", gvr)
		go gvkInformer.Start(ctx.Done())
	}
	fmt.Println("    => DC informers count", len(dc.informers))
	dc.log.Info("Finished safe registering GVR", "gvr", gvr)
}

func (dc *DynamicController) UnregisterGVK(gvr schema.GroupVersionResource) {
	dc.log.Info("Unregistering GVK", "gvr", gvr)
	dc.mu.Lock()
	defer dc.mu.Unlock()
	informer, ok := dc.informers[gvr]
	if ok {
		// dc.log.Info("Stopping informer", "gvr", gvr)
		dc.cancelFuncs[gvr]()
		informer.Shutdown()
		// dc.log.Info("Deleting informer", "gvr", gvr)
		delete(dc.informers, gvr)
	}
	dc.log.Info("Stop informers for GVK", "gvr", gvr)
}

func (cc *DynamicController) HotRestart() bool {
	// TODO: implement hot restart
	return true
}

func (dc *DynamicController) RegisterWorkflowOperator(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	c *v1alpha1.ResourceGroup,
) ([]string, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	dc.log.Info("Creating resourcegroup graph", "name", c.Name)
	resources := make([]v1alpha1.Resource, 0)
	for _, resource := range c.Spec.Resources {
		resources = append(resources, *resource)
	}

	graph, err := resourcegroup.NewGraph(resources)
	if err != nil {
		return nil, err
	}

	err = graph.TopologicalSort()
	if err != nil {
		return nil, err
	}

	dc.log.Info("Creating workflow operator", "gvr", gvr)
	wo := workflow.NewOperator(ctx, gvr, graph, dc.kubeClient)
	dc.workflowOperators[gvr] = wo
	fmt.Println("    => Operators count", len(dc.workflowOperators))
	return graph.OrderedResourceList(), nil
}

func (dc *DynamicController) UnregisterWorkflowOperator(gvr schema.GroupVersionResource) {
	dc.log.Info("Unregistering workflow operator", "gvr", gvr)
	dc.mu.Lock()
	defer dc.mu.Unlock()
	delete(dc.workflowOperators, gvr)
}
