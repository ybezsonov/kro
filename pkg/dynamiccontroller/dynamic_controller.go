// Copyright 2025 The Kube Resource Orchestrator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package dynamiccontroller provides a flexible and efficient solution for
// managing multiple GroupVersionResources (GVRs) in a Kubernetes environment.
// It implements a single controller capable of dynamically handling various
// resource types concurrently, adapting to runtime changes without system restarts.
//
// Key features and design considerations:
//
//  1. Multi GVR management: It handles multiple resource types concurrently,
//     creating and managing separate workflows for each.
//
//  2. Dynamic informer management: Creates and deletes informers on the fly
//     for new resource types, allowing real time adaptation to changes in the
//     cluster.
//
//  3. Minimal disruption: Operations on one resource type do not affect
//     the performance or functionality of others.
//
//  4. Minimalism: Unlike controller-runtime, this implementation
//     is tailored specifically for kro's needs, avoiding unnecessary
//     dependencies and overhead.
//
//  5. Future Extensibility: It allows for future enhancements such as
//     sharding and CEL cost aware leader election, which are not readily
//     achievable with k8s.io/controller-runtime.
//
// Why not use k8s.io/controller-runtime:
//
//  1. Static nature: controller-runtime is optimized for statically defined
//     controllers, however kro requires runtime creation and management
//     of controllers for various GVRs.
//
//  2. Overhead reduction: by not including unused features like leader election
//     and certain metrics, this implementation remains minimalistic and efficient.
//
//  3. Customization: this design allows for deep customization and
//     optimization specific to kro's unique requirements for managing
//     multiple GVRs dynamically.
//
// This implementation aims to provide a reusable, efficient, and flexible
// solution for dynamic multi-GVR controller management in Kubernetes environments.
//
// NOTE(a-hilaly): Potentially we might open source this package for broader use cases.
package dynamiccontroller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kro-run/kro/pkg/metadata"
	"github.com/kro-run/kro/pkg/requeue"
)

// Config holds the configuration for DynamicController
type Config struct {
	// Workers specifies the number of workers processing items from the queue
	Workers int
	// ResyncPeriod defines the interval at which the controller will re list
	// the resources, even if there haven't been any changes.
	ResyncPeriod time.Duration
	// QueueMaxRetries is the maximum number of retries for an item in the queue
	// will be retried before being dropped.
	//
	// NOTE(a-hilaly): I'm not very sure how useful is this, i'm trying to avoid
	// situations where reconcile errors exhaust the queue.
	QueueMaxRetries int
	// ShutdownTimeout is the maximum duration to wait for the controller to
	// gracefully shutdown. We ideally want to avoid forceful shutdowns, giving
	// the controller enough time to finish processing any pending items.
	ShutdownTimeout time.Duration
	// MinRetryDelay is the minimum delay before retrying an item in the queue
	MinRetryDelay time.Duration
	// MaxRetryDelay is the maximum delay before retrying an item in the queue
	MaxRetryDelay time.Duration
	// RateLimit is the maximum number of events processed per second
	RateLimit int
	// BurstLimit is the maximum number of events in a burst
	BurstLimit int
}

// DynamicController (DC) is a single controller capable of managing multiple different
// kubernetes resources (GVRs) in parallel. It can safely start watching new
// resources and stop watching others at runtime - hence the term "dynamic". This
// flexibility allows us to accept and manage various resources in a Kubernetes
// cluster without requiring restarts or pod redeployments.
//
// It is mainly inspired by native Kubernetes controllers but designed for more
// flexible and lightweight operation. DC serves as the core component of kro's
// dynamic resource management system. Its primary purpose is to create and manage
// "micro" controllers for custom resources defined by users at runtime (via the
// ResourceGraphDefinition CRs).
type DynamicController struct {
	config Config

	// kubeClient is the dynamic client used to create the informers
	kubeClient dynamic.Interface
	// informers is a safe map of GVR to informers. Each informer is responsible
	// for watching a specific GVR.
	informers sync.Map

	// handlers is a safe map of GVR to workflow operators. Each
	// handler is responsible for managing a specific GVR.
	handlers sync.Map

	// queue is the workqueue used to process items
	queue workqueue.TypedRateLimitingInterface[ObjectIdentifiers]

	log logr.Logger
}

type Handler func(ctx context.Context, req ctrl.Request) error

type informerWrapper struct {
	informer dynamicinformer.DynamicSharedInformerFactory
	shutdown func()
}

// NewDynamicController creates a new DynamicController instance.
func NewDynamicController(
	log logr.Logger,
	config Config,
	kubeClient dynamic.Interface) *DynamicController {
	logger := log.WithName("dynamic-controller")

	dc := &DynamicController{
		config:     config,
		kubeClient: kubeClient,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.NewTypedMaxOfRateLimiter(
			workqueue.NewTypedItemExponentialFailureRateLimiter[ObjectIdentifiers](config.MinRetryDelay, config.MaxRetryDelay),
			&workqueue.TypedBucketRateLimiter[ObjectIdentifiers]{Limiter: rate.NewLimiter(rate.Limit(config.RateLimit), config.BurstLimit)},
		), workqueue.TypedRateLimitingQueueConfig[ObjectIdentifiers]{Name: "dynamic-controller-queue"}),
		log: logger,
		// pass version and pod id from env
	}

	return dc
}

// AllInformerHaveSynced checks if all registered informers have synced, returns
// true if they have.
func (dc *DynamicController) AllInformerHaveSynced() bool {
	var allSynced bool
	var informerCount int

	// Unfortunately we can't know the number of informers in advance, so we need to
	// iterate over all of them to check if they have synced.

	dc.informers.Range(func(key, value interface{}) bool {
		informerCount++
		// possibly panic if the value is not a SharedIndexInformer
		informer, ok := value.(cache.SharedIndexInformer)
		if !ok {
			dc.log.Error(nil, "Failed to cast informer", "key", key)
			allSynced = false
			return false
		}
		if !informer.HasSynced() {
			allSynced = false
			return false
		}
		return true
	})

	if informerCount == 0 {
		return true
	}
	return allSynced
}

// WaitForInformerSync waits for all informers to sync or timeout
func (dc *DynamicController) WaitForInformersSync(stopCh <-chan struct{}) bool {
	dc.log.V(1).Info("Waiting for all informers to sync")
	start := time.Now()
	defer func() {
		dc.log.V(1).Info("Finished waiting for informers to sync", "duration", time.Since(start))
	}()

	return cache.WaitForCacheSync(stopCh, dc.AllInformerHaveSynced)
}

// Run starts the DynamicController.
func (dc *DynamicController) Run(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer dc.queue.ShutDown()

	dc.log.Info("Starting dynamic controller")
	defer dc.log.Info("Shutting down dynamic controller")

	// Wait for all informers to sync
	if !dc.WaitForInformersSync(ctx.Done()) {
		return fmt.Errorf("failed to sync informers")
	}

	// Spin up workers.
	//
	// TODO(a-hilaly): Allow for dynamic scaling of workers.
	for i := 0; i < dc.config.Workers; i++ {
		go wait.UntilWithContext(ctx, dc.worker, time.Second)
	}

	<-ctx.Done()
	return dc.gracefulShutdown(dc.config.ShutdownTimeout)
}

// worker processes items from the queue.
func (dc *DynamicController) worker(ctx context.Context) {
	for dc.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem processes a single item from the queue.
func (dc *DynamicController) processNextWorkItem(ctx context.Context) bool {
	item, shutdown := dc.queue.Get()
	if shutdown {
		return false
	}
	defer dc.queue.Done(item)

	queueLength.Set(float64(dc.queue.Len()))

	err := dc.syncFunc(ctx, item)
	if err == nil || apierrors.IsNotFound(err) {
		dc.queue.Forget(item)
		return true
	}

	gvrKey := fmt.Sprintf("%s/%s/%s", item.GVR.Group, item.GVR.Version, item.GVR.Resource)

	// Handle requeues
	switch typedErr := err.(type) {
	case *requeue.NoRequeue:
		dc.log.Error(typedErr, "Error syncing item, not requeuing", "item", item)
		requeueTotal.WithLabelValues(gvrKey, "no_requeue").Inc()
		dc.queue.Forget(item)
	case *requeue.RequeueNeeded:
		dc.log.V(1).Info("Requeue needed", "item", item, "error", typedErr)
		requeueTotal.WithLabelValues(gvrKey, "requeue").Inc()
		dc.queue.Add(item) // Add without rate limiting
	case *requeue.RequeueNeededAfter:
		dc.log.V(1).Info("Requeue needed after delay", "item", item, "error", typedErr, "delay", typedErr.Duration())
		requeueTotal.WithLabelValues(gvrKey, "requeue_after").Inc()
		dc.queue.AddAfter(item, typedErr.Duration())
	default:
		// Arriving here means we have an unexpected error, we should requeue the item
		// with rate limiting.
		requeueTotal.WithLabelValues(gvrKey, "rate_limited").Inc()
		if dc.queue.NumRequeues(item) < dc.config.QueueMaxRetries {
			dc.log.Error(err, "Error syncing item, requeuing with rate limit", "item", item)
			dc.queue.AddRateLimited(item)
		} else {
			dc.log.Error(err, "Dropping item from queue after max retries", "item", item)
			dc.queue.Forget(item)
		}
	}

	return true
}

// syncFunc reconciles a single item.
func (dc *DynamicController) syncFunc(ctx context.Context, oi ObjectIdentifiers) error {
	gvrKey := fmt.Sprintf("%s/%s/%s", oi.GVR.Group, oi.GVR.Version, oi.GVR.Resource)
	dc.log.V(1).Info("Syncing resourcegraphdefinition instance request", "gvr", gvrKey, "namespacedKey", oi.NamespacedKey)

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		reconcileDuration.WithLabelValues(gvrKey).Observe(duration.Seconds())
		reconcileTotal.WithLabelValues(gvrKey).Inc()
		dc.log.V(1).Info("Finished syncing resourcegraphdefinition instance request",
			"gvr", gvrKey,
			"namespacedKey", oi.NamespacedKey,
			"duration", duration)
	}()

	genericHandler, ok := dc.handlers.Load(oi.GVR)
	if !ok {
		// NOTE(a-hilaly): this might mean that the GVR is not registered, or the workflow operator
		// is not found. We should probably handle this in a better way.
		return fmt.Errorf("no handler found for GVR: %s", gvrKey)
	}

	// this is worth a panic if it fails...
	handlerFunc, ok := genericHandler.(Handler)
	if !ok {
		return fmt.Errorf("invalid handler type for GVR: %s", gvrKey)
	}
	err := handlerFunc(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: oi.NamespacedKey}})
	if err != nil {
		handlerErrorsTotal.WithLabelValues(gvrKey).Inc()
	}
	return err
}

// gracefulShutdown performs a graceful shutdown of the controller.
func (dc *DynamicController) gracefulShutdown(timeout time.Duration) error {
	dc.log.Info("Starting graceful shutdown")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var wg sync.WaitGroup
	dc.informers.Range(func(key, value interface{}) bool {
		wg.Add(1)
		go func(informer *informerWrapper) {
			defer wg.Done()
			informer.informer.Shutdown()
		}(value.(*informerWrapper))
		return true
	})

	// Wait for all informers to shut down or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		dc.log.Info("All informers shut down successfully")
	case <-ctx.Done():
		dc.log.Error(ctx.Err(), "Timeout waiting for informers to shut down")
		return ctx.Err()
	}

	return nil
}

// ObjectIdentifiers is a struct that holds the namespaced key and the GVR of the object.
//
// Since we are handling all the resources using the same handlerFunc, we need to know
// what GVR we're dealing with - so that we can use the appropriate workflow operator.
type ObjectIdentifiers struct {
	// NamespacedKey is the namespaced key of the object. Typically in the format
	// `namespace/name`.
	NamespacedKey string
	GVR           schema.GroupVersionResource
}

// updateFunc is the update event handler for the GVR informers
func (dc *DynamicController) updateFunc(old, new interface{}) {
	newObj, ok := new.(*unstructured.Unstructured)
	if !ok {
		dc.log.Error(nil, "failed to cast new object to unstructured")
		return
	}
	oldObj, ok := old.(*unstructured.Unstructured)
	if !ok {
		dc.log.Error(nil, "failed to cast old object to unstructured")
		return
	}

	if newObj.GetGeneration() == oldObj.GetGeneration() {
		dc.log.V(2).Info("Skipping update due to unchanged generation",
			"name", newObj.GetName(),
			"namespace", newObj.GetNamespace(),
			"generation", newObj.GetGeneration())
		return
	}

	dc.enqueueObject(new, "update")
}

// enqueueObject adds an object to the workqueue
func (dc *DynamicController) enqueueObject(obj interface{}, eventType string) {
	namespacedKey, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		dc.log.Error(err, "Failed to get key for object", "eventType", eventType)
		return
	}

	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		err := fmt.Errorf("object is not an Unstructured")
		dc.log.Error(err, "Failed to cast object to Unstructured", "eventType", eventType, "namespacedKey", namespacedKey)
		return
	}

	gvk := u.GroupVersionKind()
	gvr := metadata.GVKtoGVR(gvk)

	objectIdentifiers := ObjectIdentifiers{
		NamespacedKey: namespacedKey,
		GVR:           gvr,
	}

	dc.log.V(1).Info("Enqueueing object",
		"objectIdentifiers", objectIdentifiers,
		"eventType", eventType)

	informerEventsTotal.WithLabelValues(gvr.String(), eventType).Inc()
	dc.queue.Add(objectIdentifiers)
}

// StartServingGVK registers a new GVK to the informers map safely.
func (dc *DynamicController) StartServingGVK(ctx context.Context, gvr schema.GroupVersionResource, handler Handler) error {
	dc.log.V(1).Info("Registering new GVK", "gvr", gvr)

	_, exists := dc.informers.Load(gvr)
	if exists {
		// Even thought the informer is already registered, we should still
		// update the handler, as it might have changed.
		dc.handlers.Store(gvr, handler)
		// trigger reconciliation of the corresponding gvr's
		objs, err := dc.kubeClient.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list objects for GVR %s: %w", gvr, err)
		}
		for _, obj := range objs.Items {
			dc.enqueueObject(&obj, "update")
		}
		return nil
	}

	// Create a new informer
	gvkInformer := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dc.kubeClient,
		dc.config.ResyncPeriod,
		// Maybe we can make this configurable in the future. Thinking that
		// we might want to filter out some resources, by namespace or labels
		"",
		nil,
	)
	informer := gvkInformer.ForResource(gvr).Informer()

	// Set up event handlers
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { dc.enqueueObject(obj, "add") },
		UpdateFunc: dc.updateFunc,
		DeleteFunc: func(obj interface{}) { dc.enqueueObject(obj, "delete") },
	})
	if err != nil {
		dc.log.Error(err, "Failed to add event handler", "gvr", gvr)
		return fmt.Errorf("failed to add event handler for GVR %s: %w", gvr, err)
	}
	informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		dc.log.Error(err, "Watch error", "gvr", gvr)
	})
	dc.handlers.Store(gvr, handler)

	informerContext := context.Background()
	cancelableContext, cancel := context.WithCancel(informerContext)
	// Start the informer
	go func() {
		dc.log.V(1).Info("Starting informer", "gvr", gvr)
		// time.Sleep(5 * time.Millisecond)
		informer.Run(cancelableContext.Done())
	}()

	dc.log.V(1).Info("Waiting for cache sync", "gvr", gvr)
	startTime := time.Now()
	// Wait for cache sync with a timeout
	synced := cache.WaitForCacheSync(cancelableContext.Done(), informer.HasSynced)
	syncDuration := time.Since(startTime)
	informerSyncDuration.WithLabelValues(gvr.String()).Observe(syncDuration.Seconds())

	if !synced {
		cancel()
		return fmt.Errorf("failed to sync informer cache for GVR %s", gvr)
	}

	dc.informers.Store(gvr, &informerWrapper{
		informer: gvkInformer,
		shutdown: cancel,
	})
	gvrCount.Inc()
	dc.log.V(1).Info("Successfully registered GVK", "gvr", gvr)
	return nil
}

// UnregisterGVK safely removes a GVK from the controller and cleans up associated resources.
func (dc *DynamicController) StopServiceGVK(ctx context.Context, gvr schema.GroupVersionResource) error {
	dc.log.Info("Unregistering GVK", "gvr", gvr)

	// Retrieve the informer
	informerObj, ok := dc.informers.Load(gvr)
	if !ok {
		dc.log.V(1).Info("GVK not registered, nothing to unregister", "gvr", gvr)
		return nil
	}

	wrapper, ok := informerObj.(*informerWrapper)
	if !ok {
		return fmt.Errorf("invalid informer type for GVR: %s", gvr)
	}

	// Stop the informer
	dc.log.V(1).Info("Stopping informer", "gvr", gvr)

	// Cancel the context to stop the informer
	wrapper.shutdown()
	// Wait for the informer to shut down
	wrapper.informer.Shutdown()

	// Remove the informer from the map
	dc.informers.Delete(gvr)

	// Unregister the handler if any
	dc.handlers.Delete(gvr)

	gvrCount.Dec()
	// Clean up any pending items in the queue for this GVR
	// NOTE(a-hilaly): This is a bit heavy.. maybe we can find a better way to do this.
	// Thinking that we might want to have a queue per GVR.
	// dc.cleanupQueue(gvr)
	// time.Sleep(1 * time.Second)
	// isStopped := wrapper.informer.ForResource(gvr).Informer().IsStopped()
	dc.log.V(1).Info("Successfully unregistered GVK", "gvr", gvr)
	return nil
}
