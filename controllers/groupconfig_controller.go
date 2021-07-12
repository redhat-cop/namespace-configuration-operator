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
	"github.com/go-logr/logr"
	userv1 "github.com/openshift/api/user/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/controllers/common"
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
	InitGroupCount int16
	groupCounter   int16
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

	// Fetch the GroupConfig instance
	instance := &redhatcopv1alpha1.GroupConfig{}
	err := r.GetClient().Get(context, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
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
		if !util.HasFinalizer(instance, r.controllerName) {
			return reconcile.Result{}, nil
		}
		err := r.manageCleanUpLogic(instance)
		if err != nil {
			log.Error(err, "unable to delete instance", "instance", instance)
			return r.ManageError(context, instance, err)
		}
		util.RemoveFinalizer(instance, r.controllerName)
		err = r.GetClient().Update(context, instance)
		if err != nil {
			log.Error(err, "unable to update instance", "instance", instance)
			return r.ManageError(context, instance, err)
		}
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

	return r.ManageSuccess(context, instance)
}

func (r *GroupConfigReconciler) getResourceList(instance *redhatcopv1alpha1.GroupConfig, groups []userv1.Group) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	for _, group := range groups {
		lrs, err := lockedresource.GetLockedResourcesFromTemplatesWithRestConfig(instance.Spec.Templates, r.GetRestConfig(), group)
		if err != nil {
			r.Log.Error(err, "unable to process", "templates", instance.Spec.Templates, "with param", group)
			return []lockedresource.LockedResource{}, err
		}
		lockedresources = append(lockedresources, lrs...)
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

func (r *GroupConfigReconciler) findApplicableGroupConfigsFromGroup(group userv1.Group) ([]redhatcopv1alpha1.GroupConfig, error) {
	groupConfigList := &redhatcopv1alpha1.GroupConfigList{}
	err := r.GetClient().List(context.TODO(), groupConfigList, &client.ListOptions{})
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
	if len(instance.Spec.Templates) > 0 && !util.HasFinalizer(instance, r.controllerName) {
		util.AddFinalizer(instance, r.controllerName)
		needsUpdate = false
	}
	if len(instance.Spec.Templates) == 0 && util.HasFinalizer(instance, r.controllerName) {
		util.RemoveFinalizer(instance, r.controllerName)
		needsUpdate = false
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

// SetupWithManager sets up the controller with the Manager.
func (r *GroupConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.controllerName = "groupconfig-controller"

	return ctrl.NewControllerManagedBy(mgr).
		For(&redhatcopv1alpha1.GroupConfig{}, builder.WithPredicates(util.ResourceGenerationOrFinalizerChangedPredicate{})).
		Watches(&source.Kind{
			Type: &userv1.Group{
				TypeMeta: metav1.TypeMeta{
					Kind: "Group",
				},
			}}, handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {

			// Skip watching pre-existing namespaces
			if r.InitGroupCount == -1 {
				gl := &userv1.GroupList{}
				if err := r.GetClient().List(context.TODO(), gl); err != nil {
					r.Log.Error(err, "unable to list groups")
					return []reconcile.Request{}
				}
				r.InitGroupCount = int16(len(gl.Items))
			}
			if r.groupCounter < r.InitGroupCount {
				r.groupCounter++
				return []reconcile.Request{}
			}

			// Main watcher
			reconcileRequests := []reconcile.Request{}
			group := a.(*userv1.Group)
			groupConfigs, err := r.findApplicableGroupConfigsFromGroup(*group)
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
		Watches(&source.Channel{Source: r.GetStatusChangeChannel()}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
