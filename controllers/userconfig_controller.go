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
	"fmt"
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
	// templateFilter is built lazily by getTemplateFilter; see there.
	templateFilter     *common.TemplateFilter
	templateFilterOnce sync.Once
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
func (r *UserConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("userconfig", req.NamespacedName)
	common.LogReconcilingStarted(log, "userconfig", req.NamespacedName)

	// Fetch the UserConfig instance
	instance := &redhatcopv1alpha1.UserConfig{}
	err := r.GetClient().Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			log.Info("resource deletion detected - resource not found, skipping reconciliation", "userconfig", req.NamespacedName)
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
		log.Info("resource deletion detected - processing deletion cleanup", "userconfig", instance.Name, "deletionTimestamp", instance.DeletionTimestamp)
		// Support all old finalizer variants for backward compatibility
		oldFinalizerVariants := []string{
			"userconfig-controller",
			"userconfig-controller.redhat.com",
			"userconfig-controller.redhatcop.redhat.io",
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
			if errors.IsNotFound(err) {
				log.Info("resource deletion completed - resource already deleted during finalizer removal", "userconfig", instance.Name)
				return reconcile.Result{}, nil
			}
			log.Error(err, "unable to update instance", "instance", instance)
			return r.ManageError(ctx, instance, err)
		}
		log.Info("resource deletion completed successfully", "userconfig", instance.Name)
		return reconcile.Result{}, nil
	}

	// Not on the deletion path: a deleting CR must keep its event budget for CleanupIncomplete.
	common.WarnMetadataExcluded(r.GetRecorder(), instance, instance.Spec.Templates)

	//get selected users
	selectedUsers, err := r.getSelectedUsers(ctx, instance)
	if err != nil {
		log.Error(err, "unable to get users selected by", "UserConfig", instance)
		return r.ManageError(ctx, instance, err)
	}

	lockedResources, err := r.getResourceList(ctx, instance, selectedUsers)
	if err != nil {
		log.Error(err, "unable to process resources", "UserConfig", instance, "users", selectedUsers)
		return r.ManageError(ctx, instance, err)
	}

	err = r.UpdateLockedResources(ctx, instance, lockedResources, []lockedpatch.LockedPatch{})
	if err != nil {
		log.Error(err, "unable to update locked resources")
		return r.ManageError(ctx, instance, err)
	}

	common.LogResourcesProcessedSuccessfully(log, "userconfig", instance.Name, len(selectedUsers), len(lockedResources), "users")

	// Use retry mechanism to handle optimistic concurrency conflicts
	// This re-fetches the instance before each retry to ensure we have the latest resourceVersion
	return common.ManageSuccessWithRetry(r, ctx, req, log, "userconfig", instance.GetGeneration(), func() *redhatcopv1alpha1.UserConfig { return &redhatcopv1alpha1.UserConfig{} })
}

// getResourceList renders every applicable template for every selected user. A render failure is
// returned, not swallowed: the caller ends the reconcile in ManageError and the enforcer never sees a
// partial desired state (see common.TemplateFilter.Render).
func (r *UserConfigReconciler) getResourceList(ctx context.Context, instance *redhatcopv1alpha1.UserConfig, users []userv1.User) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	filter := r.getTemplateFilter()
	for i := range users {
		user := &users[i]
		lrs, err := filter.Render(ctx, instance.Spec.Templates, user)
		if err != nil {
			return nil, fmt.Errorf("userconfig %s: %w", instance.Name, err)
		}
		if len(lrs) == 0 {
			// No template in this UserConfig applies to this user; visible at V(1), not an error.
			r.Log.V(1).Info("skipping user - no UserConfig templates match the user pattern",
				"user", user.Name,
				"userconfig", instance.Name)
			continue
		}
		lockedresources = append(lockedresources, lrs...)
	}
	return lockedresources, nil
}

// filterApplicableTemplates keeps the templates that would render something for this user, so a
// guarded template is never handed to the renderer for a user its guard rejects. The decision
// logic, and why an empty render must be avoided, lives in common.TemplateFilter.
func (r *UserConfigReconciler) filterApplicableTemplates(templates []apis.LockedResourceTemplate, user userv1.User) []apis.LockedResourceTemplate {
	return r.getTemplateFilter().FilterApplicable(templates, &user)
}

// isTemplateApplicableToUser reports whether one template would render something for the user.
func (r *UserConfigReconciler) isTemplateApplicableToUser(template apis.LockedResourceTemplate, user userv1.User) bool {
	return r.getTemplateFilter().IsApplicable(template, &user)
}

// getTemplateFilter builds the filter on first use. The rest config its render fallback may need is
// only set once the reconciler is wired to a manager; unit tests construct the reconciler bare, and
// there a nil config is fine because their templates never reach an API lookup.
func (r *UserConfigReconciler) getTemplateFilter() *common.TemplateFilter {
	r.templateFilterOnce.Do(func() {
		r.templateFilter = common.NewTemplateFilter(r.Log.WithName("templatefilter"), r.GetRestConfig())
	})
	return r.templateFilter
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

	for i := range userList.Items {
		user := &userList.Items[i]
		// A user is selected ONCE, however many of its identities match. Appending per identity
		// rendered every template N times, and the enforcer then ran N child controllers for the
		// same object.
		for j := range identitiesList.Items {
			identity := &identitiesList.Items[j]
			if user.GetUID() == identity.User.UID && r.matches(instance, user, identity) {
				selectedUsers = append(selectedUsers, *user)
				break
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

func (r *UserConfigReconciler) findApplicableUserConfigsFromIdentities(ctx context.Context, user *userv1.User, identities []userv1.Identity) ([]redhatcopv1alpha1.UserConfig, error) {
	userConfigList := &redhatcopv1alpha1.UserConfigList{}
	err := r.GetClient().List(ctx, userConfigList, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to get all userconfigs")
		return []redhatcopv1alpha1.UserConfig{}, err
	}
	applicableUserConfigs := []redhatcopv1alpha1.UserConfig{}
	for i := range userConfigList.Items {
		userConfig := &userConfigList.Items[i]
		// One request per applicable UserConfig, whichever identity made it applicable.
		for j := range identities {
			if r.matches(userConfig, user, &identities[j]) {
				applicableUserConfigs = append(applicableUserConfigs, *userConfig)
				break
			}
		}
	}
	return applicableUserConfigs, nil
}

func (r *UserConfigReconciler) findApplicableUserConfigsFromUser(ctx context.Context, user *userv1.User) ([]redhatcopv1alpha1.UserConfig, error) {
	identitiesList := &userv1.IdentityList{}
	err := r.GetClient().List(ctx, identitiesList, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to get all identities")
		return []redhatcopv1alpha1.UserConfig{}, err
	}
	matchingIdentities := []userv1.Identity{}
	for _, identity := range identitiesList.Items {
		cidentity := identity.DeepCopy()
		matchingIdentities = append(matchingIdentities, *cidentity)
	}
	return r.findApplicableUserConfigsFromIdentities(ctx, user, matchingIdentities)
}

// IsInitialized none
func (r *UserConfigReconciler) IsInitialized(instance *redhatcopv1alpha1.UserConfig) bool {
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
	oldFinalizerName := "userconfig-controller"
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

// manageCleanUpLogic removes everything this UserConfig owns before its finalizer goes.
//
// Terminate alone is not enough: it deletes only what the in-memory enforcer was started with, which
// is nothing after an operator restart and nothing after a failed attempt (the entry is dropped), so a
// CR deleted in either state used to finalize with every managed object orphaned. The owned set is
// therefore recomputed from the spec and deleted explicitly (NotFound is ignored). Terminate runs
// first so a started enforcer cannot recreate what is deleted next.
//
// A user whose templates no longer render cannot have its objects recomputed; that is reported as
// a Warning event and an error-level log line naming the user, and deletion proceeds, because a
// finalizer that can never clear is worse than a documented orphan. A failed DELETE keeps the finalizer.
func (r *UserConfigReconciler) manageCleanUpLogic(ctx context.Context, instance *redhatcopv1alpha1.UserConfig) error {
	if err := r.Terminate(instance, true); err != nil {
		r.Log.Error(err, "unable to terminate enforcing reconciler for", "instance", instance)
		return err
	}
	// A selector that does not compile means the owned set cannot be computed from this spec at all
	// (and such a CR never created anything under it, since selection fails before enforcement).
	// Say so and let the deletion finish; only a real API failure below keeps the finalizer.
	if err := common.ValidateSelectors(common.NamedSelector{Name: "labelSelector", Selector: instance.Spec.LabelSelector}, common.NamedSelector{Name: "annotationSelector", Selector: instance.Spec.AnnotationSelector}, common.NamedSelector{Name: "identityExtraFieldSelector", Selector: instance.Spec.IdentityExtraFieldSelector}); err != nil {
		r.Log.Error(err, "cannot recompute the objects owned by a UserConfig whose selector does not compile; nothing is deleted", "userconfig", instance.Name)
		r.GetRecorder().Event(instance, "Warning", "CleanupIncomplete", err.Error())
		return nil
	}
	selected, err := r.getSelectedUsers(ctx, instance)
	if err != nil {
		return fmt.Errorf("unable to list the users selected by UserConfig %s during deletion: %w", instance.Name, err)
	}
	objs := make([]metav1.Object, 0, len(selected))
	for i := range selected {
		objs = append(objs, &selected[i])
	}
	owned, failures := r.getTemplateFilter().OwnedResources(ctx, instance.Spec.Templates, objs)
	for _, f := range failures {
		r.Log.Error(f, "could not recompute the objects owned for one user; anything created from that template there is NOT deleted", "userconfig", instance.Name)
		r.GetRecorder().Event(instance, "Warning", "CleanupIncomplete", f.Error())
	}
	if err := r.DeleteUnstructuredResources(ctx, owned); err != nil {
		return fmt.Errorf("unable to delete the objects owned by UserConfig %s: %w", instance.Name, err)
	}
	r.Log.Info("deleted the objects owned by the UserConfig", "userconfig", instance.Name, "objects", len(owned), "users", len(selected))
	return nil
}

func (r *UserConfigReconciler) findUserFromIdentity(ctx context.Context, identity *userv1.Identity) (*userv1.User, error) {
	userList := &userv1.UserList{}
	err := r.GetClient().List(ctx, userList, &client.ListOptions{})
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
	r.controllerName = "redhatcop.redhat.io/userconfig-controller"
	return ctrl.NewControllerManagedBy(mgr).
		For(&redhatcopv1alpha1.UserConfig{}, builder.WithPredicates(common.ResourceGenerationOrFinalizerOrDeletionTimestampChangedPredicate)).
		Watches(&userv1.User{
			TypeMeta: metav1.TypeMeta{
				Kind: "User",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}
			user := a.(*userv1.User)
			userConfigs, err := r.findApplicableUserConfigsFromUser(ctx, user)
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
		Watches(&userv1.Identity{
			TypeMeta: metav1.TypeMeta{
				Kind: "Identity",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}
			identity := a.(*userv1.Identity)
			user, err := r.findUserFromIdentity(ctx, identity)
			if err != nil {
				// An Identity without a User is an ordinary state (the User is created after the
				// Identity, or was deleted): nothing to enqueue, not an error worth a line per event.
				r.Log.V(1).Info("identity has no user yet, nothing to enqueue", "identity", identity.Name, "reason", err.Error())
				return []reconcile.Request{}
			}
			userConfigs, err := r.findApplicableUserConfigsFromIdentities(ctx, user, []userv1.Identity{*identity})
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
		WatchesRawSource(&source.Channel{Source: r.GetStatusChangeChannel()}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
