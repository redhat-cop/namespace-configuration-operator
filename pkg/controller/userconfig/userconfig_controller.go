package userconfig

import (
	"context"
	errs "errors"

	userv1 "github.com/openshift/api/user/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/pkg/common"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedpatch"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedresource"
	"github.com/scylladb/go-set/strset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerName = "userconfig-controller"

var log = logf.Log.WithName(controllerName)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new UserConfig Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileUserConfig{
		EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor(controllerName), true),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	reconcileUserConfig, ok := r.(*ReconcileUserConfig)
	if !ok {
		err := errs.New("unable to convert to ReconcileUserConfig")
		log.Error(err, "unable to convert to ReconcileUserConfig from ", "reconciler", r)
		return err
	}

	if ok, err := reconcileUserConfig.IsAPIResourceAvailable(schema.GroupVersionKind{
		Group:   "user.openshift.io",
		Version: "v1",
		Kind:    "User",
	}); !ok || err != nil {
		if err != nil {
			return err
		}
		return nil
	}

	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource UserConfig
	err = c.Watch(&source.Kind{
		Type: &redhatcopv1alpha1.UserConfig{
			TypeMeta: metav1.TypeMeta{
				Kind: "UserConfig",
			},
		}}, &handler.EnqueueRequestForObject{}, util.ResourceGenerationOrFinalizerChangedPredicate{})
	if err != nil {
		return err
	}

	var userToUserConfig = handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}
			user := a.Object.(*userv1.User)
			userConfigs, err := reconcileUserConfig.findApplicableUserConfigsFromUser(user)
			if err != nil {
				log.Error(err, "unable to find applicable UserConfigs for", "user", user)
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
		})

	// Watch for changes to User
	err = c.Watch(&source.Kind{
		Type: &userv1.User{
			TypeMeta: metav1.TypeMeta{
				Kind: "User",
			},
		}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: userToUserConfig,
	})
	if err != nil {
		return err
	}

	var identityToUserConfig = handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}
			identity := a.Object.(*userv1.Identity)
			user, err := reconcileUserConfig.findUserFromIdentity(identity)
			if err != nil {
				log.Error(err, "unable to find applicable User for", "identity", identity)
				return []reconcile.Request{}
			}
			userConfigs, err := reconcileUserConfig.findApplicableUserConfigsFromIdentities(user, []userv1.Identity{*identity})
			if err != nil {
				log.Error(err, "unable to find applicable UserConfigs for", "identity", identity)
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
		})

	// Watch for changes to User
	err = c.Watch(&source.Kind{
		Type: &userv1.Identity{
			TypeMeta: metav1.TypeMeta{
				Kind: "Identity",
			},
		}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: identityToUserConfig,
	})
	if err != nil {
		return err
	}

	//if interested in updates from the managed resources
	// watch for changes in status in the locked resources
	err = c.Watch(
		&source.Channel{Source: reconcileUserConfig.GetStatusChangeChannel()},
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileUserConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileUserConfig{}

// ReconcileUserConfig reconciles a UserConfig object
type ReconcileUserConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	lockedresourcecontroller.EnforcingReconciler
}

// Reconcile reads that state of the cluster for a UserConfig object and makes changes based on the state read
// and what is in the UserConfig.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileUserConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling UserConfig")

	// Fetch the UserConfig instance
	instance := &redhatcopv1alpha1.UserConfig{}
	err := r.GetClient().Get(context.TODO(), request.NamespacedName, instance)
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
		err := r.GetClient().Update(context.TODO(), instance)
		if err != nil {
			log.Error(err, "unable to update instance", "instance", instance)
			return r.ManageError(instance, err)
		}
		return reconcile.Result{}, nil
	}

	if util.IsBeingDeleted(instance) {
		if !util.HasFinalizer(instance, controllerName) {
			return reconcile.Result{}, nil
		}
		err := r.manageCleanUpLogic(instance)
		if err != nil {
			log.Error(err, "unable to delete instance", "instance", instance)
			return r.ManageError(instance, err)
		}
		util.RemoveFinalizer(instance, controllerName)
		err = r.GetClient().Update(context.TODO(), instance)
		if err != nil {
			log.Error(err, "unable to update instance", "instance", instance)
			return r.ManageError(instance, err)
		}
		return reconcile.Result{}, nil
	}

	//get selected users
	selectedUsers, err := r.getSelectedUsers(instance)
	if err != nil {
		log.Error(err, "unable to get users selected by", "UserConfig", instance)
		return r.ManageError(instance, err)
	}

	lockedResources, err := r.getResourceList(instance, selectedUsers)
	if err != nil {
		log.Error(err, "unable to process resources", "UserConfig", instance, "users", selectedUsers)
		return r.ManageError(instance, err)
	}

	err = r.UpdateLockedResources(instance, lockedResources, []lockedpatch.LockedPatch{})
	if err != nil {
		log.Error(err, "unable to update locked resources")
		return r.ManageError(instance, err)
	}

	return r.ManageSuccess(instance)
}

func (r *ReconcileUserConfig) getResourceList(instance *redhatcopv1alpha1.UserConfig, users []userv1.User) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	for _, user := range users {
		lrs, err := lockedresource.GetLockedResourcesFromTemplates(instance.Spec.Templates, user)
		if err != nil {
			log.Error(err, "unable to process", "templates", instance.Spec.Templates, "with param", user)
			return []lockedresource.LockedResource{}, err
		}
		lockedresources = append(lockedresources, lrs...)
	}
	return lockedresources, nil
}

func (r *ReconcileUserConfig) getSelectedUsers(instance *redhatcopv1alpha1.UserConfig) ([]userv1.User, error) {
	userList := &userv1.UserList{}
	identitiesList := &userv1.IdentityList{}

	err := r.GetClient().List(context.TODO(), userList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to get all users")
		return []userv1.User{}, err
	}

	err = r.GetClient().List(context.TODO(), identitiesList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to get all identities")
		return []userv1.User{}, err
	}

	selectedUsers := []userv1.User{}

	for _, user := range userList.Items {
		for _, identity := range identitiesList.Items {
			if user.GetUID() == identity.User.UID {
				if matches(instance, &user, &identity) {
					selectedUsers = append(selectedUsers, user)
				}
			}
		}
	}
	return selectedUsers, nil
}

func matches(instance *redhatcopv1alpha1.UserConfig, user *userv1.User, indentity *userv1.Identity) bool {
	extraFieldSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.IdentityExtraFieldSelector)
	if err != nil {
		log.Error(err, "unable to create ", "selector from", instance.Spec.IdentityExtraFieldSelector)
		return false
	}
	labelSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.LabelSelector)
	if err != nil {
		log.Error(err, "unable to create ", "selector from", instance.Spec.LabelSelector)
		return false
	}
	annotationSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.AnnotationSelector)
	if err != nil {
		log.Error(err, "unable to create ", "selector from", instance.Spec.AnnotationSelector)
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

func (r *ReconcileUserConfig) findApplicableUserConfigsFromIdentities(user *userv1.User, identities []userv1.Identity) ([]redhatcopv1alpha1.UserConfig, error) {
	userConfigList := &redhatcopv1alpha1.UserConfigList{}
	err := r.GetClient().List(context.TODO(), userConfigList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to get all userconfigs")
		return []redhatcopv1alpha1.UserConfig{}, err
	}
	applicableUserConfigs := []redhatcopv1alpha1.UserConfig{}
	for _, userConfig := range userConfigList.Items {
		for _, identity := range identities {
			if matches(&userConfig, user, &identity) {
				applicableUserConfigs = append(applicableUserConfigs, userConfig)
			}
		}
	}
	return applicableUserConfigs, nil
}

func (r *ReconcileUserConfig) findApplicableUserConfigsFromUser(user *userv1.User) ([]redhatcopv1alpha1.UserConfig, error) {
	identitiesList := &userv1.IdentityList{}
	err := r.GetClient().List(context.TODO(), identitiesList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to get all identities")
		return []redhatcopv1alpha1.UserConfig{}, err
	}
	matchingIdentities := []userv1.Identity{}
	for _, identity := range identitiesList.Items {
		matchingIdentities = append(matchingIdentities, identity)
	}
	return r.findApplicableUserConfigsFromIdentities(user, matchingIdentities)
}

// IsInitialized none
func (r *ReconcileUserConfig) IsInitialized(instance *redhatcopv1alpha1.UserConfig) bool {
	needsUpdate := true
	for i := range instance.Spec.Templates {
		currentSet := strset.New(instance.Spec.Templates[i].ExcludedPaths...)
		if !currentSet.IsEqual(strset.Union(common.DefaultExcludedPathsSet, currentSet)) {
			instance.Spec.Templates[i].ExcludedPaths = strset.Union(common.DefaultExcludedPathsSet, currentSet).List()
			needsUpdate = false
		}
	}
	if len(instance.Spec.Templates) > 0 && !util.HasFinalizer(instance, controllerName) {
		util.AddFinalizer(instance, controllerName)
		needsUpdate = false
	}
	if len(instance.Spec.Templates) == 0 && util.HasFinalizer(instance, controllerName) {
		util.RemoveFinalizer(instance, controllerName)
		needsUpdate = false
	}

	return needsUpdate
}

func (r *ReconcileUserConfig) manageCleanUpLogic(instance *redhatcopv1alpha1.UserConfig) error {
	err := r.Terminate(instance, true)
	if err != nil {
		log.Error(err, "unable to terminate enforcing reconciler for", "instance", instance)
		return err
	}
	return nil
}

func (r *ReconcileUserConfig) findUserFromIdentity(identity *userv1.Identity) (*userv1.User, error) {
	userList := &userv1.UserList{}
	err := r.GetClient().List(context.TODO(), userList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to get all users")
		return &userv1.User{}, err
	}

	for _, user := range userList.Items {
		log.V(1).Info("comparing", "user uid", user.GetUID(), " and identity uid", identity.User.UID)
		if user.GetUID() == identity.User.UID {
			return &user, nil
		}
	}
	return &userv1.User{}, errs.New("user not found")
}
