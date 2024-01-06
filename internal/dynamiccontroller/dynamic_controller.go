package dynamiccontroller

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/aws/symphony/api/v1alpha1"
	"github.com/aws/symphony/internal/construct"
	"github.com/aws/symphony/internal/workflow"
	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
)

type DynamicController struct {
	// name is an identifier for this particular controller instance.
	name string
	// kubeClient is a dynamic client to the Kubernetes cluster.
	kubeClient *dynamic.DynamicClient
	// synced informs if the controller is synced with the apiserver
	synced cache.InformerSynced

	workflowOperators map[string]*workflow.Operator

	// handler is the function that will be called when a new work item is added
	// to the queue. The argument to the handler is an interface that should be
	// castable to the appropriate type.
	//
	// Note(a-hilaly) maybe unstructured.Unstructured is a better choice here.
	handlerO func(context.Context, ctrl.Request) error
	// queue is where incoming work is placed to de-dup and to allow "easy"
	// rate limited requeues.
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
	logger := klog.FromContext(ctx)
	// wo := workflow.NewOperator(schema.GroupVersionResource{}, nil, nil)

	dc := &DynamicController{
		name:       name,
		kubeClient: kubeClient,
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
			// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		), "dynamic-controller-queue"),
		handlerO:          handler,
		informers:         map[schema.GroupVersionResource]dynamicinformer.DynamicSharedInformerFactory{},
		cancelFuncs:       map[schema.GroupVersionResource]context.CancelFunc{},
		log:               &logger,
		mu:                sync.RWMutex{},
		listers:           map[schema.GroupVersionResource]dynamiclister.Lister{},
		workflowOperators: map[string]*workflow.Operator{},
	}
	return dc
}

// Run the main goroutine responsible for watching and syncing jobs.
func (cc *DynamicController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer cc.queue.ShutDown()

	logger := klog.FromContext(ctx)
	logger.Info("Starting symphony dynamic controller", "name", cc.name)
	defer logger.Info("Shutting symphony dynamic controller", "name", cc.name)

	/* 	if !cache.WaitForNamedCacheSync(cc.name, ctx.Done(), cc.synced) {
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
	cKey, quit := cc.queue.Get()
	if quit {
		return false
	}
	defer cc.queue.Done(cKey)

	if err := cc.syncFunc(ctx, cKey.(string)); err != nil {
		cc.queue.AddRateLimited(cKey)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("sync %v failed with : %v", cKey, err))
		}
		return true
	}

	cc.queue.Forget(cKey)
	return true

}

func (cc *DynamicController) enqueueObject(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	cc.queue.Add(key)
}

func (cc *DynamicController) syncFunc(ctx context.Context, key string) error {
	logger := klog.FromContext(ctx)
	startTime := time.Now()
	defer func() {
		logger.Info("Finished syncing construct claim request", "elapsedTime", time.Since(startTime))
	}()

	wo := cc.workflowOperators[""]
	return wo.Handler(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: key}})
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
	gvkInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc.kubeClient, 0, metav1.NamespaceAll, nil)
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.informers[gvr] = gvkInformer
	dc.log.V(4).Info("Finished registering GVK", "gvk", gvr)
}

// SafeRegisterGVK registers a new GVK to the informers map safely.
func (dc *DynamicController) SafeRegisterGVK(gvr schema.GroupVersionResource) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if _, ok := dc.informers[gvr]; !ok {
		gvkInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc.kubeClient, 5*time.Second, metav1.NamespaceAll, nil)
		gvkInformer.ForResource(gvr).Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				dc.log.Info("Adding object")
				dc.enqueueObject(obj)
			},
			UpdateFunc: func(old, new interface{}) {
				dc.log.Info("Updating object")
				dc.enqueueObject(new)
			},
			DeleteFunc: func(obj interface{}) {
				/* 				csr, ok := obj.(*certificates.CertificateSigningRequest)
				   				if !ok {
				   					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				   					if !ok {
				   						dc.log.V(2).Info("Couldn't get object from tombstone", "object", obj)
				   						return
				   					}
				   					csr, ok = tombstone.Obj.(*certificates.CertificateSigningRequest)
				   					if !ok {
				   						dc.log.V(2).Info("Tombstone contained object that is not a CSR", "object", obj)
				   						return
				   					}
				   				} */
				dc.log.Info("Deleting object")
				dc.enqueueObject(obj)
			},
		})
		dc.informers[gvr] = gvkInformer
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		dc.cancelFuncs[gvr] = cancel

		time.Sleep(1 * time.Second)
		fmt.Println("Starting informer")
		go gvkInformer.Start(ctx.Done())
	}
	dc.log.V(4).Info("Finished safe-registering GVR", "gvr", gvr)
}

func (dc *DynamicController) UnregisterGVK(gvr schema.GroupVersionResource) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	delete(dc.informers, gvr)
}

func (cc *DynamicController) HotRestart() bool {
	// TODO: implement hot restart
	return true
}

func (cc *DynamicController) RegisterWorkflowOperator(gvr schema.GroupVersionResource, c *v1alpha1.Construct) error {
	resources := make([]v1alpha1.Resource, 0)
	for _, resource := range c.Spec.Resources {
		resources = append(resources, *resource)
	}

	graph, err := construct.NewGraph(resources)
	if err != nil {
		return err
	}

	wo := workflow.NewOperator(gvr, graph, cc.kubeClient.Resource(gvr))
	cc.workflowOperators["gvr"] = wo
	return nil
}
