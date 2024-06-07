/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/symphony/api/v1alpha1"
	"github.com/aws/symphony/internal/condition"
	"github.com/aws/symphony/internal/crd"
	"github.com/aws/symphony/internal/dynamiccontroller"
	"github.com/aws/symphony/internal/finalizer"
	openapischema "github.com/aws/symphony/internal/schema"
)

// ResourceGroupReconciler reconciles a ResourceGroup object
type ResourceGroupReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	CRDManager        crd.Manager
	OpenAPISchema     *openapischema.OpenAPISchemaTransformer
	DynamicController *dynamiccontroller.DynamicController
}

//+kubebuilder:rbac:groups=x.symphony.k8s.aws,resources=resourcegroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=x.symphony.k8s.aws,resources=resourcegroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=x.symphony.k8s.aws,resources=resourcegroups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ResourceGroup object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *ResourceGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling", "resource", req.NamespacedName)

	var resourcegroup v1alpha1.ResourceGroup
	err := r.Get(ctx, req.NamespacedName, &resourcegroup)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Got resourcegroup from the api server", "name", req.NamespacedName)

	log.Info("Transforming resourcegroup definition to OpenAPIv3 schema", "name", req.NamespacedName)

	// Handle creation
	oaSchema, err := r.OpenAPISchema.Transform(resourcegroup.Spec.Definition.Spec.Raw, resourcegroup.Spec.Definition.Status.Raw)
	if err != nil {
		log.Info("unable to transform OpenAPI schema")
		return ctrl.Result{}, err
	}

	/* 	yamlSchema, err := yaml.Marshal(oaSchema)
	   	if err != nil {
	   		log.Info("unable to marshal OpenAPI schema")
	   		return ctrl.Result{}, err
	   	}
	   	fmt.Println(string(yamlSchema)) */

	customRD := crd.FromOpenAPIV3Schema(resourcegroup.Spec.ApiVersion, resourcegroup.Spec.Kind, oaSchema)

	/* 	bb, err := yaml.Marshal(customRD)
	   	if err != nil {
	   		log.Info("unable to marshal OpenAPI schema")
	   		return ctrl.Result{}, err
	   	}
	   	fmt.Println(string(bb)) */

	log.Info("Creating custom resource definition", "crd_name", customRD.Name)
	err = r.CRDManager.Ensure(ctx, customRD)
	if err != nil {
		log.Info("unable to ensure CRD")
		return ctrl.Result{}, err
	}

	gvr := schema.GroupVersionResource{
		Group:    customRD.Spec.Group,
		Version:  customRD.Spec.Versions[0].Name,
		Resource: customRD.Spec.Names.Plural,
	}

	// Handle deletions
	if !resourcegroup.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("resourcegroup is deleted")
		err := r.CRDManager.Delete(ctx, customRD.Name)
		if err != nil {
			log.Info("unable to delete CRD")
			return ctrl.Result{}, err
		}
		log.Info("Unregistering GVK in symphony's dynamic controller", "crd_name", customRD.Name, "gvr", gvr)
		r.DynamicController.UnregisterGVK(gvr)
		log.Info("Unregistering workflow operator in symphony's dynamic controller", "crd_name", customRD.Name, "gvr", gvr)
		r.DynamicController.UnregisterWorkflowOperator(gvr)
		log.Info("Removing finalizer from resourcegroup", "crd_name", customRD.Name, "gvr", gvr)
		err = r.setUnmanaged(ctx, &resourcegroup)
		if err != nil {
			log.Info("unable to set unmanaged")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	gvrStr := fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)
	log.Info("Registering GVK in symphony's dynamic controller", "crd_name", customRD.Name, "gvr", gvrStr)
	r.DynamicController.SafeRegisterGVK(gvr)

	log.Info("Registering workflow operator in symphony's dynamic controller", "crd_name", customRD.Name, "gvr", gvrStr)
	orderedResources, err := r.DynamicController.RegisterWorkflowOperator(
		ctx,
		gvr,
		&resourcegroup,
	)
	if err != nil {
		if err := r.setStatusInactive(ctx, &resourcegroup, err); err != nil {
			log.Info("unable to set status inactive")
			return ctrl.Result{}, err
		}
		log.Info("unable to register workflow operator")
		return ctrl.Result{}, nil
	}

	// Set managed
	log.Info("Setting symphony finalizers", "crd_name", customRD.Name, "gvr", gvrStr)
	err = r.setManaged(ctx, &resourcegroup)
	if err != nil {
		log.Info("unable to set managed")
		return ctrl.Result{}, err
	}
	err = r.setStatusActive(ctx, &resourcegroup, orderedResources)
	if err != nil {
		log.Info("unable to set status active")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ResourceGroup{}).
		Complete(r)
}

func (r *ResourceGroupReconciler) setManaged(ctx context.Context, resourcegroup *v1alpha1.ResourceGroup) error {
	newFinalizers := finalizer.AddSymphonyFinalizer(resourcegroup)
	dc := resourcegroup.DeepCopy()
	dc.Finalizers = newFinalizers
	if len(dc.Finalizers) != len(resourcegroup.Finalizers) {
		fmt.Println("  => setting finalizers to: ", newFinalizers)
		patch := client.MergeFrom(resourcegroup.DeepCopy())
		return r.Patch(ctx, dc.DeepCopy(), patch)
	}
	return nil
}

func (r *ResourceGroupReconciler) setStatusActive(
	ctx context.Context, resourcegroup *v1alpha1.ResourceGroup, orderedResources []string,
) error {
	dc := resourcegroup.DeepCopy()
	dc.Status.State = "ACTIVE"
	dc.Status.TopoligicalOrder = orderedResources
	conditions := dc.Status.Conditions
	newConditions := condition.SetCondition(conditions,
		condition.NewReconcilerReadyCondition(
			corev1.ConditionTrue,
			"",
			"micro controller is ready",
		),
	)
	newConditions = condition.SetCondition(newConditions,
		condition.NewGraphSyncedCondition(
			corev1.ConditionTrue,
			"",
			"Directed Acyclic Graph is synced",
		),
	)
	dc.Status.Conditions = newConditions
	patch := client.MergeFrom(resourcegroup.DeepCopy())
	// data, _ := patch.Data(dc)
	return r.Status().Patch(ctx, dc.DeepCopy(), patch)
}

func (r *ResourceGroupReconciler) setStatusInactive(ctx context.Context, resourcegroup *v1alpha1.ResourceGroup, err error) error {
	dc := resourcegroup.DeepCopy()
	dc.Status.State = "INACTIVE"
	conditions := dc.Status.Conditions
	newConditions := condition.SetCondition(conditions,
		condition.NewReconcilerReadyCondition(
			corev1.ConditionFalse,
			"",
			"micro controller is ready",
		),
	)
	newConditions = condition.SetCondition(newConditions,
		condition.NewGraphSyncedCondition(
			corev1.ConditionFalse,
			err.Error(),
			"Directed Acyclic Graph is synced",
		),
	)
	dc.Status.Conditions = newConditions
	patch := client.MergeFrom(resourcegroup.DeepCopy())
	// data, _ := patch.Data(dc)
	return r.Status().Patch(ctx, dc.DeepCopy(), patch)
}

func (r *ResourceGroupReconciler) setUnmanaged(ctx context.Context, resourcegroup *v1alpha1.ResourceGroup) error {
	newFinalizers := finalizer.RemoveSymphonyFinalizer(resourcegroup)
	dc := resourcegroup.DeepCopy()
	dc.Finalizers = newFinalizers
	fmt.Println("  => unsetting finalizers to: ", newFinalizers)
	patch := client.MergeFrom(resourcegroup.DeepCopy())
	return r.Patch(ctx, dc.DeepCopy(), patch)
}
