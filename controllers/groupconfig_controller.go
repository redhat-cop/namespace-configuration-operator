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
	"sync"

	"github.com/go-logr/logr"
	userv1 "github.com/openshift/api/user/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/controllers/common"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedpatch"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedresource"
	"github.com/scylladb/go-set/strset"
	"k8s.io/apimachinery/pkg/api/errors"
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

// GroupConfigReconciler reconciles a GroupConfig object
type GroupConfigReconciler struct {
	lockedresourcecontroller.EnforcingReconciler
	Log            logr.Logger
	controllerName string
	// templateFilter is built lazily by getTemplateFilter; see there.
	templateFilter     *common.TemplateFilter
	templateFilterOnce sync.Once
}

// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=groupconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=groupconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=groupconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=*,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GroupConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *GroupConfigReconciler) Reconcile(context context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("groupconfig", req.NamespacedName)
	common.LogReconcilingStarted(log, "groupconfig", req.NamespacedName)

	// Fetch the GroupConfig instance
	instance := &redhatcopv1alpha1.GroupConfig{}
	err := r.GetClient().Get(context, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			log.Info("resource deletion detected - resource not found, skipping reconciliation", "groupconfig", req.NamespacedName)
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if !r.IsInitialized(instance) {
		err := r.GetClient().Update(context, instance)
		if err != nil {
			log.Error(err, "unable to update instance", "instance", instance)
			return r.ManageError(context, instance, err)
		}
		return reconcile.Result{}, nil
	}

	if util.IsBeingDeleted(instance) {
		log.Info("resource deletion detected - processing deletion cleanup", "groupconfig", instance.Name, "deletionTimestamp", instance.DeletionTimestamp)
		// Support all old finalizer variants for backward compatibility
		oldFinalizerVariants := []string{
			"groupconfig-controller",
			"groupconfig-controller.redhat.com",
			"groupconfig-controller.redhatcop.redhat.io",
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

		err := r.manageCleanUpLogic(instance)
		if err != nil {
			log.Error(err, "unable to delete instance", "instance", instance)
			return r.ManageError(context, instance, err)
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

		err = r.GetClient().Update(context, instance)
		if err != nil {
			// If the resource is already deleted (NotFound), that's fine - just return success
			if errors.IsNotFound(err) {
				log.Info("resource deletion completed - resource already deleted during finalizer removal", "groupconfig", instance.Name)
				return reconcile.Result{}, nil
			}
			log.Error(err, "unable to update instance", "instance", instance)
			return r.ManageError(context, instance, err)
		}
		log.Info("resource deletion completed successfully", "groupconfig", instance.Name)
		return reconcile.Result{}, nil
	}

	//get selected users
	selectedGroups, err := r.getSelectedGroups(context, instance)
	if err != nil {
		log.Error(err, "unable to get groups selected by", "GroupConfig", instance)
		return r.ManageError(context, instance, err)
	}

	lockedResources, err := r.getResourceList(instance, selectedGroups)
	if err != nil {
		log.Error(err, "unable to process resources", "GroupConfig", instance, "groups", selectedGroups)
		return r.ManageError(context, instance, err)
	}

	err = r.UpdateLockedResources(context, instance, lockedResources, []lockedpatch.LockedPatch{})
	if err != nil {
		log.Error(err, "unable to update locked resources")
		return r.ManageError(context, instance, err)
	}

	common.LogResourcesProcessedSuccessfully(log, "groupconfig", instance.Name, len(selectedGroups), len(lockedResources), "groups")

	// Use retry mechanism to handle optimistic concurrency conflicts
	// This re-fetches the instance before each retry to ensure we have the latest resourceVersion
	return common.ManageSuccessWithRetry(r, context, req, log, "groupconfig", func() *redhatcopv1alpha1.GroupConfig { return &redhatcopv1alpha1.GroupConfig{} })
}

func (r *GroupConfigReconciler) getResourceList(instance *redhatcopv1alpha1.GroupConfig, groups []userv1.Group) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	for _, group := range groups {
		// Filter templates that are applicable to this group BEFORE processing
		applicableTemplates := r.filterApplicableTemplates(instance.Spec.Templates, group)

		// Only process templates that are actually applicable
		if len(applicableTemplates) > 0 {
			lrs, err := lockedresource.GetLockedResourcesFromTemplatesWithRestConfig(applicableTemplates, r.GetRestConfig(), group)
			if err != nil {
				r.Log.Error(err, "unable to process", "templates", applicableTemplates, "with param", group)
				return []lockedresource.LockedResource{}, err
			}
			lockedresources = append(lockedresources, lrs...)
		} else {
			// Group is being skipped because no templates in this GroupConfig match the group's pattern
			// This is logged at V(1) level to be visible but not too verbose
			r.Log.V(1).Info("skipping group - no GroupConfig templates match the group pattern",
				"group", group.Name,
				"groupconfig", instance.Name)
		}
	}
	return lockedresources, nil
}

func (r *GroupConfigReconciler) getSelectedGroups(context context.Context, instance *redhatcopv1alpha1.GroupConfig) ([]userv1.Group, error) {
	groupList := &userv1.GroupList{}

	labelSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.LabelSelector)
	if err != nil {
		r.Log.Error(err, "unable to create ", "selector from", instance.Spec.LabelSelector)
		return []userv1.Group{}, err
	}

	annotationSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.AnnotationSelector)
	if err != nil {
		r.Log.Error(err, "unable to create ", "selector from", instance.Spec.AnnotationSelector)
		return []userv1.Group{}, err
	}

	err = r.GetClient().List(context, groupList, &client.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		r.Log.Error(err, "unable to get groups with", "selector", labelSelector)
		return []userv1.Group{}, err
	}

	selectedGroups := []userv1.Group{}
	for _, group := range groupList.Items {
		annotationsAsLabels := labels.Set(group.Annotations)
		if annotationSelector.Matches(annotationsAsLabels) {
			selectedGroups = append(selectedGroups, group)
		}
	}

	return selectedGroups, nil
}

func (r *GroupConfigReconciler) findApplicableGroupConfigsFromGroup(ctx context.Context, group userv1.Group) ([]redhatcopv1alpha1.GroupConfig, error) {
	groupConfigList := &redhatcopv1alpha1.GroupConfigList{}
	err := r.GetClient().List(ctx, groupConfigList, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to get all userconfigs")
		return []redhatcopv1alpha1.GroupConfig{}, err
	}
	applicableGroupConfigs := []redhatcopv1alpha1.GroupConfig{}

	for _, groupConfig := range groupConfigList.Items {
		labelSelector, err := metav1.LabelSelectorAsSelector(&groupConfig.Spec.LabelSelector)
		if err != nil {
			r.Log.Error(err, "unable to create ", "selector from", groupConfig.Spec.LabelSelector)
			return []redhatcopv1alpha1.GroupConfig{}, err
		}

		annotationSelector, err := metav1.LabelSelectorAsSelector(&groupConfig.Spec.AnnotationSelector)
		if err != nil {
			r.Log.Error(err, "unable to create ", "selector from", groupConfig.Spec.AnnotationSelector)
			return []redhatcopv1alpha1.GroupConfig{}, err
		}

		labelsAslabels := labels.Set(group.GetLabels())
		annotationsAsLabels := labels.Set(group.GetAnnotations())
		if labelSelector.Matches(labelsAslabels) && annotationSelector.Matches(annotationsAsLabels) {
			applicableGroupConfigs = append(applicableGroupConfigs, groupConfig)
		}
	}

	return applicableGroupConfigs, nil
}

// IsInitialized none
func (r *GroupConfigReconciler) IsInitialized(instance *redhatcopv1alpha1.GroupConfig) bool {
	needsUpdate := true
	for i := range instance.Spec.Templates {
		currentSet := strset.New(instance.Spec.Templates[i].ExcludedPaths...)
		if !currentSet.IsEqual(strset.Union(common.DefaultExcludedPathsSet, currentSet)) {
			instance.Spec.Templates[i].ExcludedPaths = strset.Union(common.DefaultExcludedPathsSet, currentSet).List()
			needsUpdate = false
		}
	}

	// Migrate old finalizer to new finalizer (only if not being deleted)
	oldFinalizerName := "groupconfig-controller"
	if !util.IsBeingDeleted(instance) && util.HasFinalizer(instance, oldFinalizerName) {
		util.RemoveFinalizer(instance, oldFinalizerName)
		util.AddFinalizer(instance, r.controllerName)
		needsUpdate = false
	}

	// Only add/remove finalizers if not being deleted
	if !util.IsBeingDeleted(instance) {
		if len(instance.Spec.Templates) > 0 && !util.HasFinalizer(instance, r.controllerName) {
			util.AddFinalizer(instance, r.controllerName)
			needsUpdate = false
		}
		if len(instance.Spec.Templates) == 0 && util.HasFinalizer(instance, r.controllerName) {
			util.RemoveFinalizer(instance, r.controllerName)
			needsUpdate = false
		}
	}

	return needsUpdate
}

func (r *GroupConfigReconciler) manageCleanUpLogic(instance *redhatcopv1alpha1.GroupConfig) error {
	err := r.Terminate(instance, true)
	if err != nil {
		r.Log.Error(err, "unable to terminate enforcing reconciler for", "instance", instance)
		return err
	}
	return nil
}

// filterApplicableTemplates keeps the templates that would render something for this group, so a
// guarded template is never handed to the renderer for a group its guard rejects. The decision
// logic, and why an empty render must be avoided, lives in common.TemplateFilter.
func (r *GroupConfigReconciler) filterApplicableTemplates(templates []apis.LockedResourceTemplate, group userv1.Group) []apis.LockedResourceTemplate {
	return r.getTemplateFilter().FilterApplicable(templates, &group)
}

// isTemplateApplicableToGroup reports whether one template would render something for the group.
func (r *GroupConfigReconciler) isTemplateApplicableToGroup(template apis.LockedResourceTemplate, group userv1.Group) bool {
	return r.getTemplateFilter().IsApplicable(template, &group)
}

// getTemplateFilter builds the filter on first use. The rest config its render fallback may need is
// only set once the reconciler is wired to a manager; unit tests construct the reconciler bare, and
// there a nil config is fine because their templates never reach an API lookup.
func (r *GroupConfigReconciler) getTemplateFilter() *common.TemplateFilter {
	r.templateFilterOnce.Do(func() {
		r.templateFilter = common.NewTemplateFilter(r.Log.WithName("templatefilter"), r.GetRestConfig())
	})
	return r.templateFilter
}

// SetupWithManager sets up the controller with the Manager.
func (r *GroupConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.controllerName = "redhatcop.redhat.io/groupconfig-controller"

	return ctrl.NewControllerManagedBy(mgr).
		For(&redhatcopv1alpha1.GroupConfig{}, builder.WithPredicates(common.ResourceGenerationOrFinalizerOrDeletionTimestampChangedPredicate)).
		Watches(&userv1.Group{
			TypeMeta: metav1.TypeMeta{
				Kind: "Group",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}
			group := a.(*userv1.Group)
			groupConfigs, err := r.findApplicableGroupConfigsFromGroup(ctx, *group)
			if err != nil {
				r.Log.Error(err, "unable to find applicable GroupConfigs for", "group", group)
				return []reconcile.Request{}
			}
			for _, userconfig := range groupConfigs {
				reconcileRequests = append(reconcileRequests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      userconfig.GetName(),
						Namespace: userconfig.GetNamespace(),
					},
				})
			}
			return reconcileRequests
		})).
		WatchesRawSource(&source.Channel{Source: r.GetStatusChangeChannel()}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
