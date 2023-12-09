package dynamiccontroller

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"k8s.io/kubernetes/pkg/apis/certificates"
	ctrl "sigs.k8s.io/controller-runtime"
)

type DynamicController struct {
	// name is an identifier for this particular controller instance.
	name string
	// kubeClient is a dynamic client to the Kubernetes cluster.
	kubeClient *dynamic.DynamicClient
	// synced informs if the controller is synced with the apiserver
	synced cache.InformerSynced
	// handler is the function that will be called when a new work item is added
	// to the queue. The argument to the handler is an interface that should be
	// castable to the appropriate type.
	//
	// Note(a-hilaly) maybe unstructured.Unstructured is a better choice here.
	handler func(context.Context, ctrl.Request) error
	// queue is where incoming work is placed to de-dup and to allow "easy"
	// rate limited requeues.
	queue workqueue.RateLimitingInterface
	// informers is a the map of the registered informers
	informers   map[v1.GroupVersionKind]dynamicinformer.DynamicSharedInformerFactory
	cancelFuncs map[v1.GroupVersionKind]context.CancelFunc
	// Protects access to the informers map. Could have been a sync.Map but we need to
	// optimize for the read case.
	mu sync.RWMutex
	// listers is a map of the registered listers
	listers map[v1.GroupVersionKind]dynamiclister.Lister

	log *logr.Logger
}

func NewDynamicController(
	ctx context.Context,
	name string,
	kubeClient *dynamic.DynamicClient,
	csrInformer *dynamicinformer.DynamicSharedInformerFactory,
	handler func(context.Context, ctrl.Request) error,
) *DynamicController {
	logger := klog.FromContext(ctx)
	dc := &DynamicController{
		name:       name,
		kubeClient: kubeClient,
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
			// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		), "dynamic-controller-queue"),
		handler:   handler,
		informers: map[v1.GroupVersionKind]dynamicinformer.DynamicSharedInformerFactory{},
		log:       &logger,
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

	if !cache.WaitForNamedCacheSync(cc.name, ctx.Done(), cc.synced) {
		return
	}

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
		logger.V(4).Info("Finished syncing abstraction claim request", "elapsedTime", time.Since(startTime))
	}()

	// need to operate on a copy so we don't mutate the csr in the shared cache
	// csr = csr.DeepCopy()
	// handle namespacing
	return cc.handler(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: key}})
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
func (dc *DynamicController) RegisterGVK(gvk v1.GroupVersionKind) {
	gvkInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc.kubeClient, 0, v1.NamespaceAll, nil)
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.informers[gvk] = gvkInformer
	dc.log.V(4).Info("Finished registering GVK", "gvk", gvk)
}

// SafeRegisterGVK registers a new GVK to the informers map safely.
func (dc *DynamicController) SafeRegisterGVK(gvk v1.GroupVersionKind, gvr schema.GroupVersionResource) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if _, ok := dc.informers[gvk]; !ok {
		gvkInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc.kubeClient, 0, v1.NamespaceAll, nil)
		gvkInformer.ForResource(gvr).Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				dc.log.V(4).Info("Adding object")
				dc.enqueueObject(obj)
			},
			UpdateFunc: func(old, new interface{}) {
				dc.log.V(4).Info("Updating object request")
				dc.enqueueObject(new)
			},
			DeleteFunc: func(obj interface{}) {
				csr, ok := obj.(*certificates.CertificateSigningRequest)
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
				}
				dc.log.V(4).Info("Deleting certificate request", "csr", csr.Name)
				dc.enqueueObject(obj)
			},
		})
		dc.informers[gvk] = gvkInformer
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		dc.cancelFuncs[gvk] = cancel

		gvkInformer.Start(ctx.Done())
	}
	dc.log.V(4).Info("Finished safe-registering GVK", "gvk", gvk)
}

func (dc *DynamicController) UnregisterGVK(gvk v1.GroupVersionKind) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	delete(dc.informers, gvk)
}

func (cc *DynamicController) HotRestart() bool {
	// TODO: implement hot restart
	return true
}
