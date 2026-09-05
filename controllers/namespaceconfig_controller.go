/*
Copyright 2020 Red Hat Community of Practice.

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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/controllers/common"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedpatch"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedresource"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// NamespaceConfigReconciler reconciles a NamespaceConfig object
type NamespaceConfigReconciler struct {
	lockedresourcecontroller.EnforcingReconciler
	Log            logr.Logger
	controllerName string
	// templateFilter is built lazily by getTemplateFilter; see there.
	templateFilter        *common.TemplateFilter
	templateFilterOnce    sync.Once
	AllowSystemNamespaces bool
}

// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=namespaceconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=namespaceconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=namespaceconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=*,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NamespaceConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *NamespaceConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespaceconfig", req.NamespacedName)
	common.LogReconcilingStarted(log, "namespaceconfig", req.NamespacedName)
	// Fetch the NamespaceConfig instance
	instance := &redhatcopv1alpha1.NamespaceConfig{}
	err := r.GetClient().Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			log.Info("resource deletion detected - resource not found, skipping reconciliation", "namespaceconfig", req.NamespacedName)
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	// Finalizers are the only thing this reconciler writes to the CR, and they go as a merge patch
	// computed against the copy taken here, so only metadata.finalizers crosses the wire. A
	// whole-object Update also serialised the empty, non-pointer selector structs (`annotationSelector:
	// {}`) into the spec and dropped an author's empty lists (measured in review): the spec is the
	// author's, not this operator's. The patch carries the resourceVersion (optimistic lock), so a
	// finalizer another actor added between the read and this write conflicts and is retried instead
	// of being overwritten, as the Update it replaces did (review).
	original := instance.DeepCopy()
	if !r.IsInitialized(instance) {
		err := r.GetClient().Patch(ctx, instance, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{}))
		if err != nil {
			log.Error(err, "unable to update finalizers", "instance", instance)
			return r.ManageError(ctx, instance, err)
		}
		return reconcile.Result{}, nil
	}

	if util.IsBeingDeleted(instance) {
		common.ForgetMetadataExcluded(instance)
		log.Info("resource deletion detected - processing deletion cleanup", "namespaceconfig", instance.Name, "deletionTimestamp", instance.DeletionTimestamp)
		// Support all old finalizer variants for backward compatibility
		oldFinalizerVariants := []string{
			"namespaceconfig-controller",
			"namespaceconfig-controller.redhat.com",
			"namespaceconfig-controller.redhatcop.redhat.io",
		}

		hasAnyFinalizer := false
		for _, oldFinalizer := range oldFinalizerVariants {
			if util.HasFinalizer(instance, oldFinalizer) {
				hasAnyFinalizer = true
				break
			}
		}
		if !hasAnyFinalizer && !util.HasFinalizer(instance, r.controllerName) {
			return reconcile.Result{}, nil
		}

		err := r.manageCleanUpLogic(ctx, instance)
		if err != nil {
			log.Error(err, "unable to delete instance", "instance", instance)
			return r.ManageError(ctx, instance, err)
		}

		// Remove all old finalizer variants and new finalizer if present
		for _, oldFinalizer := range oldFinalizerVariants {
			if util.HasFinalizer(instance, oldFinalizer) {
				util.RemoveFinalizer(instance, oldFinalizer)
			}
		}
		if util.HasFinalizer(instance, r.controllerName) {
			util.RemoveFinalizer(instance, r.controllerName)
		}

		err = r.GetClient().Patch(ctx, instance, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{}))
		if err != nil {
			// If the resource is already deleted (NotFound), that's fine - just return success
			if apierrors.IsNotFound(err) {
				log.Info("resource deletion completed - resource already deleted during finalizer removal", "namespaceconfig", instance.Name)
				return reconcile.Result{}, nil
			}
			log.Error(err, "unable to update instance", "instance", instance)
			return r.ManageError(ctx, instance, err)
		}
		log.Info("resource deletion completed successfully", "namespaceconfig", instance.Name)
		return reconcile.Result{}, nil
	}
	//get selected namespaces
	selectedNamespaces, err := r.getSelectedNamespaces(ctx, instance)
	if err != nil {
		log.Error(err, "unable to get namespaces selected by", "NamespaceConfig", instance)
		return r.ManageError(ctx, instance, err)
	}

	// Not on the deletion path: a deleting CR must keep its event budget for CleanupIncomplete.
	common.WarnMetadataExcluded(r.GetRecorder(), instance, instance.Spec.Templates)

	lockedResources, err := r.getResourceList(ctx, instance, selectedNamespaces)
	if err != nil {
		log.Error(err, "unable to process resources", "NamespaceConfig", instance, "namespaces", selectedNamespaces)
		return r.ManageError(ctx, instance, err)
	}

	err = r.UpdateLockedResources(ctx, instance, lockedResources, []lockedpatch.LockedPatch{})
	if err != nil {
		log.Error(err, "unable to update locked resources")
		return r.ManageError(ctx, instance, err)
	}

	common.LogResourcesProcessedSuccessfully(log, "namespaceconfig", instance.Name, len(selectedNamespaces), len(lockedResources), "namespaces")

	// Use retry mechanism to handle optimistic concurrency conflicts
	// This re-fetches the instance before each retry to ensure we have the latest resourceVersion
	return common.ManageSuccessWithRetry(r, ctx, req, log, "namespaceconfig", instance.GetGeneration(), func() *redhatcopv1alpha1.NamespaceConfig { return &redhatcopv1alpha1.NamespaceConfig{} })
}

// manageCleanUpLogic removes everything this NamespaceConfig owns before its finalizer goes.
//
// Terminate alone is not enough: it deletes only what the in-memory enforcer was started with, which
// is nothing after an operator restart and nothing after a failed attempt (the entry is dropped), so a
// CR deleted in either state used to finalize with every managed object orphaned. The owned set is
// therefore recomputed from the spec and deleted explicitly (NotFound is ignored). Terminate runs
// first so a started enforcer cannot recreate what is deleted next.
//
// A namespace whose templates no longer render cannot have its objects recomputed; that is reported as
// a Warning event and an error-level log line naming the namespace, and deletion proceeds, because a
// finalizer that can never clear is worse than a documented orphan. A failed DELETE keeps the finalizer.
func (r *NamespaceConfigReconciler) manageCleanUpLogic(ctx context.Context, instance *redhatcopv1alpha1.NamespaceConfig) error {
	if err := r.Terminate(instance, true); err != nil {
		r.Log.Error(err, "unable to terminate enforcing reconciler for", "instance", instance)
		return err
	}
	// A selector that does not compile means the owned set cannot be computed from this spec at all
	// (and such a CR never created anything under it, since selection fails before enforcement).
	// Say so and let the deletion finish; only a real API failure below keeps the finalizer.
	if err := common.ValidateSelectors(common.NamedSelector{Name: "labelSelector", Selector: instance.Spec.LabelSelector}, common.NamedSelector{Name: "annotationSelector", Selector: instance.Spec.AnnotationSelector}); err != nil {
		r.Log.Error(err, "cannot recompute the objects owned by a NamespaceConfig whose selector does not compile; nothing is deleted", "namespaceconfig", instance.Name)
		r.GetRecorder().Event(instance, "Warning", "CleanupIncomplete", err.Error())
		return nil
	}
	selected, err := r.getSelectedNamespaces(ctx, instance)
	if err != nil {
		return fmt.Errorf("unable to list the namespaces selected by NamespaceConfig %s during deletion: %w", instance.Name, err)
	}
	objs := make([]metav1.Object, 0, len(selected))
	for i := range selected {
		objs = append(objs, &selected[i])
	}
	owned, failures := r.getTemplateFilter().OwnedResources(ctx, instance.Spec.Templates, objs)
	for _, f := range failures {
		r.Log.Error(f, "could not recompute the objects owned for one namespace; anything created from that template there is NOT deleted", "namespaceconfig", instance.Name)
		r.GetRecorder().Event(instance, "Warning", "CleanupIncomplete", f.Error())
	}
	if err := r.DeleteUnstructuredResources(ctx, owned); err != nil {
		return fmt.Errorf("unable to delete the objects owned by NamespaceConfig %s: %w", instance.Name, err)
	}
	r.Log.Info("deleted the objects owned by the NamespaceConfig", "namespaceconfig", instance.Name, "objects", len(owned), "namespaces", len(selected))
	return nil
}

// IsInitialized none
func (r *NamespaceConfigReconciler) IsInitialized(instance *redhatcopv1alpha1.NamespaceConfig) bool {
	// True means "nothing to write"; a false return makes the caller Update the object and return.
	// Only finalizers are written here. The spec is the author's: the default excludedPaths are
	// applied in memory when the locked resources are built (common.EffectiveExcludedPaths), not
	// written into spec.templates[].excludedPaths as before, so the CR stays equal to what its author
	// or their Git declared (issue #16; the chart's 0.21.1 entry records the rewrite loop the old
	// behaviour caused against a GitOps controller).
	initialized := true
	if util.IsBeingDeleted(instance) {
		return true
	}

	// Migrate old finalizer to new finalizer (only if not being deleted)
	oldFinalizerName := "namespaceconfig-controller"
	if !util.IsBeingDeleted(instance) && util.HasFinalizer(instance, oldFinalizerName) {
		util.RemoveFinalizer(instance, oldFinalizerName)
		util.AddFinalizer(instance, r.controllerName)
		initialized = false
	}

	// Only add/remove finalizers if not being deleted
	if !util.IsBeingDeleted(instance) {
		if len(instance.Spec.Templates) > 0 && !util.HasFinalizer(instance, r.controllerName) {
			util.AddFinalizer(instance, r.controllerName)
			initialized = false
		}
		if len(instance.Spec.Templates) == 0 && util.HasFinalizer(instance, r.controllerName) {
			util.RemoveFinalizer(instance, r.controllerName)
			initialized = false
		}
	}

	return initialized
}

// getResourceList renders every applicable template for every selected namespace. A render failure is
// returned, not swallowed: the caller ends the reconcile in ManageError and the enforcer never sees a
// partial desired state (see common.TemplateFilter.Render).
func (r *NamespaceConfigReconciler) getResourceList(ctx context.Context, instance *redhatcopv1alpha1.NamespaceConfig, namespaces []corev1.Namespace) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	filter := r.getTemplateFilter()
	for i := range namespaces {
		namespace := &namespaces[i]
		lrs, err := filter.Render(ctx, instance.Spec.Templates, namespace)
		if err != nil {
			return nil, fmt.Errorf("namespaceconfig %s: %w", instance.Name, err)
		}
		if len(lrs) == 0 {
			// No template in this NamespaceConfig applies to this namespace; visible at V(1), not an error.
			r.Log.V(1).Info("skipping namespace - no NamespaceConfig templates match the namespace pattern",
				"namespace", namespace.Name,
				"namespaceconfig", instance.Name)
			continue
		}
		lockedresources = append(lockedresources, lrs...)
	}
	return lockedresources, nil
}

// filterApplicableTemplates keeps the templates that would render something for this namespace, so a
// guarded template is never handed to the renderer for a namespace its guard rejects. The decision
// logic, and why an empty render must be avoided, lives in common.TemplateFilter.
func (r *NamespaceConfigReconciler) filterApplicableTemplates(templates []apis.LockedResourceTemplate, namespace corev1.Namespace) []apis.LockedResourceTemplate {
	return r.getTemplateFilter().FilterApplicable(templates, &namespace)
}

// isTemplateApplicableToNamespace reports whether one template would render something for the namespace.
func (r *NamespaceConfigReconciler) isTemplateApplicableToNamespace(template apis.LockedResourceTemplate, namespace corev1.Namespace) bool {
	return r.getTemplateFilter().IsApplicable(template, &namespace)
}

// getTemplateFilter builds the filter on first use. The rest config its render fallback may need is
// only set once the reconciler is wired to a manager; unit tests construct the reconciler bare, and
// there a nil config is fine because their templates never reach an API lookup.
func (r *NamespaceConfigReconciler) getTemplateFilter() *common.TemplateFilter {
	r.templateFilterOnce.Do(func() {
		r.templateFilter = common.NewTemplateFilter(r.Log.WithName("templatefilter"), r.GetRestConfig())
	})
	return r.templateFilter
}

func (r *NamespaceConfigReconciler) getSelectedNamespaces(context context.Context, namespaceconfig *redhatcopv1alpha1.NamespaceConfig) ([]corev1.Namespace, error) {
	nl := corev1.NamespaceList{}
	selector, err := metav1.LabelSelectorAsSelector(&namespaceconfig.Spec.LabelSelector)
	if err != nil {
		r.Log.Error(err, "unable to create selector from label selector", "selector", &namespaceconfig.Spec.LabelSelector)
		return []corev1.Namespace{}, err
	}

	annotationSelector, err := metav1.LabelSelectorAsSelector(&namespaceconfig.Spec.AnnotationSelector)
	if err != nil {
		r.Log.Error(err, "unable to create ", "selector from", namespaceconfig.Spec.AnnotationSelector)
		return []corev1.Namespace{}, err
	}

	err = r.GetClient().List(context, &nl, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		r.Log.Error(err, "unable to list namespaces with selector", "selector", selector)
		return []corev1.Namespace{}, err
	}

	selectedNamespaces := []corev1.Namespace{}

	for _, namespace := range nl.Items {
		annotationsAsLabels := labels.Set(namespace.Annotations)
		if annotationSelector.Matches(annotationsAsLabels) && (r.AllowSystemNamespaces || !isProhibitedNamespaceName(namespace.GetName())) {
			selectedNamespaces = append(selectedNamespaces, namespace)
		}
	}

	return selectedNamespaces, nil
}

func (r *NamespaceConfigReconciler) findApplicableNameSpaceConfigs(ctx context.Context, namespace corev1.Namespace) ([]redhatcopv1alpha1.NamespaceConfig, error) {
	if !r.AllowSystemNamespaces && isProhibitedNamespaceName(namespace.GetName()) {
		return []redhatcopv1alpha1.NamespaceConfig{}, nil
	}
	//find all the namespaceconfig
	result := []redhatcopv1alpha1.NamespaceConfig{}
	ncl := redhatcopv1alpha1.NamespaceConfigList{}
	err := r.GetClient().List(ctx, &ncl, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to retrieve the list of namespace configs")
		return []redhatcopv1alpha1.NamespaceConfig{}, err
	}
	//for each namespaceconfig see if it selects the namespace
	for _, nc := range ncl.Items {
		// A malformed selector is that CR's problem alone: its own reconcile reports it as
		// ReconcileError. Returning here would enqueue NOTHING for any other CR on every namespace
		// event, and that outage outlives the bad CR until some unrelated event arrives.
		labelSelector, err := metav1.LabelSelectorAsSelector(&nc.Spec.LabelSelector)
		if err != nil {
			r.Log.Error(err, "skipping NamespaceConfig with a malformed labelSelector", "namespaceconfig", nc.Name)
			continue
		}
		annotationSelector, err := metav1.LabelSelectorAsSelector(&nc.Spec.AnnotationSelector)
		if err != nil {
			r.Log.Error(err, "skipping NamespaceConfig with a malformed annotationSelector", "namespaceconfig", nc.Name)
			continue
		}

		labelsAslabels := labels.Set(namespace.GetLabels())
		annotationsAsLabels := labels.Set(namespace.GetAnnotations())
		if labelSelector.Matches(labelsAslabels) && annotationSelector.Matches(annotationsAsLabels) {
			result = append(result, nc)
		}
	}
	return result, nil
}

func isProhibitedNamespaceName(name string) bool {
	return name == "default" || strings.HasPrefix(name, "openshift-") || strings.HasPrefix(name, "kube-")
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.controllerName = "redhatcop.redhat.io/namespaceconfig-controller"
	return ctrl.NewControllerManagedBy(mgr).
		For(&redhatcopv1alpha1.NamespaceConfig{}, builder.WithPredicates(common.ResourceGenerationOrFinalizerOrDeletionTimestampChangedPredicate)).
		Watches(&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind: "Namespace",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			res := []reconcile.Request{}
			ns := a.(*corev1.Namespace)
			ncl, err := r.findApplicableNameSpaceConfigs(ctx, *ns)
			if err != nil {
				r.Log.Error(err, "unable to find applicable NamespaceConfig for namespace", "namespace", ns.Name)
				return []reconcile.Request{}
			}
			for _, namespaceconfig := range ncl {
				res = append(res, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      namespaceconfig.GetName(),
						Namespace: namespaceconfig.GetNamespace(),
					},
				})
			}
			return res
		}), builder.WithPredicates(common.SelectedObjectChangedPredicate)).
		WatchesRawSource(&source.Channel{Source: r.GetStatusChangeChannel()}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
