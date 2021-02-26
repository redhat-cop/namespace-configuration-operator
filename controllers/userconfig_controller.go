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
	errs "errors"

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

// UserConfigReconciler reconciles a UserConfig object
type UserConfigReconciler struct {
	lockedresourcecontroller.EnforcingReconciler
	Log            logr.Logger
	controllerName string
}

// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=userconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=userconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=userconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=*,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the UserConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *UserConfigReconciler) Reconcile(context context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("userconfig", req.NamespacedName)

	// Fetch the UserConfig instance
	instance := &redhatcopv1alpha1.UserConfig{}
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
	selectedUsers, err := r.getSelectedUsers(context, instance)
	if err != nil {
		log.Error(err, "unable to get users selected by", "UserConfig", instance)
		return r.ManageError(context, instance, err)
	}

	lockedResources, err := r.getResourceList(instance, selectedUsers)
	if err != nil {
		log.Error(err, "unable to process resources", "UserConfig", instance, "users", selectedUsers)
		return r.ManageError(context, instance, err)
	}

	err = r.UpdateLockedResources(context, instance, lockedResources, []lockedpatch.LockedPatch{})
	if err != nil {
		log.Error(err, "unable to update locked resources")
		return r.ManageError(context, instance, err)
	}

	return r.ManageSuccess(context, instance)
}

func (r *UserConfigReconciler) getResourceList(instance *redhatcopv1alpha1.UserConfig, users []userv1.User) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	for _, user := range users {
		lrs, err := lockedresource.GetLockedResourcesFromTemplatesWithRestConfig(instance.Spec.Templates, r.GetRestConfig(), user)
		if err != nil {
			r.Log.Error(err, "unable to process", "templates", instance.Spec.Templates, "with param", user)
			return []lockedresource.LockedResource{}, err
		}
		lockedresources = append(lockedresources, lrs...)
	}
	return lockedresources, nil
}

func (r *UserConfigReconciler) getSelectedUsers(context context.Context, instance *redhatcopv1alpha1.UserConfig) ([]userv1.User, error) {
	userList := &userv1.UserList{}
	identitiesList := &userv1.IdentityList{}

	err := r.GetClient().List(context, userList, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to get all users")
		return []userv1.User{}, err
	}

	err = r.GetClient().List(context, identitiesList, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to get all identities")
		return []userv1.User{}, err
	}

	selectedUsers := []userv1.User{}

	for _, user := range userList.Items {
		for _, identity := range identitiesList.Items {
			if user.GetUID() == identity.User.UID {
				if r.matches(instance, &user, &identity) {
					selectedUsers = append(selectedUsers, user)
				}
			}
		}
	}
	return selectedUsers, nil
}

func (r *UserConfigReconciler) matches(instance *redhatcopv1alpha1.UserConfig, user *userv1.User, indentity *userv1.Identity) bool {
	extraFieldSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.IdentityExtraFieldSelector)
	if err != nil {
		r.Log.Error(err, "unable to create ", "selector from", instance.Spec.IdentityExtraFieldSelector)
		return false
	}
	labelSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.LabelSelector)
	if err != nil {
		r.Log.Error(err, "unable to create ", "selector from", instance.Spec.LabelSelector)
		return false
	}
	annotationSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.AnnotationSelector)
	if err != nil {
		r.Log.Error(err, "unable to create ", "selector from", instance.Spec.AnnotationSelector)
		return false
	}

	extraFieldAsLabels := labels.Set(indentity.Extra)
	labelsAsLabels := labels.Set(user.Labels)
	annotationsAsLabels := labels.Set(user.Annotations)
	if instance.Spec.ProviderName != "" {
		return extraFieldSelector.Matches(extraFieldAsLabels) && labelSelector.Matches(labelsAsLabels) && annotationSelector.Matches(annotationsAsLabels) && indentity.ProviderName == instance.Spec.ProviderName
	}
	return extraFieldSelector.Matches(extraFieldAsLabels) && labelSelector.Matches(labelsAsLabels) && annotationSelector.Matches(annotationsAsLabels)
}

func (r *UserConfigReconciler) findApplicableUserConfigsFromIdentities(user *userv1.User, identities []userv1.Identity) ([]redhatcopv1alpha1.UserConfig, error) {
	userConfigList := &redhatcopv1alpha1.UserConfigList{}
	err := r.GetClient().List(context.TODO(), userConfigList, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to get all userconfigs")
		return []redhatcopv1alpha1.UserConfig{}, err
	}
	applicableUserConfigs := []redhatcopv1alpha1.UserConfig{}
	for _, userConfig := range userConfigList.Items {
		for _, identity := range identities {
			if r.matches(&userConfig, user, &identity) {
				applicableUserConfigs = append(applicableUserConfigs, userConfig)
			}
		}
	}
	return applicableUserConfigs, nil
}

func (r *UserConfigReconciler) findApplicableUserConfigsFromUser(user *userv1.User) ([]redhatcopv1alpha1.UserConfig, error) {
	identitiesList := &userv1.IdentityList{}
	err := r.GetClient().List(context.TODO(), identitiesList, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to get all identities")
		return []redhatcopv1alpha1.UserConfig{}, err
	}
	matchingIdentities := []userv1.Identity{}
	for _, identity := range identitiesList.Items {
		matchingIdentities = append(matchingIdentities, identity)
	}
	return r.findApplicableUserConfigsFromIdentities(user, matchingIdentities)
}

// IsInitialized none
func (r *UserConfigReconciler) IsInitialized(instance *redhatcopv1alpha1.UserConfig) bool {
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

func (r *UserConfigReconciler) manageCleanUpLogic(instance *redhatcopv1alpha1.UserConfig) error {
	err := r.Terminate(instance, true)
	if err != nil {
		r.Log.Error(err, "unable to terminate enforcing reconciler for", "instance", instance)
		return err
	}
	return nil
}

func (r *UserConfigReconciler) findUserFromIdentity(identity *userv1.Identity) (*userv1.User, error) {
	userList := &userv1.UserList{}
	err := r.GetClient().List(context.TODO(), userList, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to get all users")
		return &userv1.User{}, err
	}

	for _, user := range userList.Items {
		r.Log.V(1).Info("comparing", "user uid", user.GetUID(), " and identity uid", identity.User.UID)
		if user.GetUID() == identity.User.UID {
			return &user, nil
		}
	}
	return &userv1.User{}, errs.New("user not found")
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.controllerName = "userconfig-controller"
	return ctrl.NewControllerManagedBy(mgr).
		For(&redhatcopv1alpha1.UserConfig{}, builder.WithPredicates(util.ResourceGenerationOrFinalizerChangedPredicate{})).
		Watches(&source.Kind{
			Type: &userv1.User{
				TypeMeta: metav1.TypeMeta{
					Kind: "User",
				},
			}}, handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}
			user := a.(*userv1.User)
			userConfigs, err := r.findApplicableUserConfigsFromUser(user)
			if err != nil {
				r.Log.Error(err, "unable to find applicable UserConfigs for", "user", user)
				return []reconcile.Request{}
			}
			for _, userconfig := range userConfigs {
				reconcileRequests = append(reconcileRequests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      userconfig.GetName(),
						Namespace: userconfig.GetNamespace(),
					},
				})
			}
			return reconcileRequests
		})).
		Watches(&source.Kind{
			Type: &userv1.Identity{
				TypeMeta: metav1.TypeMeta{
					Kind: "Identity",
				},
			}}, handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}
			identity := a.(*userv1.Identity)
			user, err := r.findUserFromIdentity(identity)
			if err != nil {
				r.Log.Error(err, "unable to find applicable User for", "identity", identity)
				return []reconcile.Request{}
			}
			userConfigs, err := r.findApplicableUserConfigsFromIdentities(user, []userv1.Identity{*identity})
			if err != nil {
				r.Log.Error(err, "unable to find applicable UserConfigs for", "identity", identity)
				return []reconcile.Request{}
			}
			for _, userconfig := range userConfigs {
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
