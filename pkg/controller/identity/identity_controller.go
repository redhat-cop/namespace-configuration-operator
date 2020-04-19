package identity

import (
	"context"
	errs "errors"

	userv1 "github.com/openshift/api/user/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/pkg/common"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedresource"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerName = "identity-controller"

var log = logf.Log.WithName(controllerName)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Identity Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileIdentity{util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor(controllerName))}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Identity
	err = c.Watch(&source.Kind{Type: &userv1.Identity{
		TypeMeta: metav1.TypeMeta{
			Kind: "Identity",
		},
	}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileIdentity implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileIdentity{}

// ReconcileIdentity reconciles a Identity object
type ReconcileIdentity struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	util.ReconcilerBase
}

// Reconcile reads that state of the cluster for a Identity object and makes changes based on the state read
// and what is in the Identity.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileIdentity) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Identity")

	// Fetch the Identity instance
	instance := &userv1.Identity{}
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

	unMatchingUserConfigs, err := r.getUnMatchingUserConfigs(instance)
	if err != nil {
		log.Error(err, "unable to retrieve unmatching UserConfigs for", "instance", instance)
		return r.ManageError(instance, err)
	}

	toBeDeleteResources, err := r.processTemplatesFor(instance, unMatchingUserConfigs)
	if err != nil {
		log.Error(err, "unable to process templates for", "instance", instance, "unMatchingUserConfigs", unMatchingUserConfigs)
		return r.ManageError(instance, err)
	}

	err = r.DeleteResourcesIfExist(common.GetResources(toBeDeleteResources))
	if err != nil {
		log.Error(err, "unable to delete resources", "resources", toBeDeleteResources)
		return r.ManageError(instance, err)
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileIdentity) getUnMatchingUserConfigs(instance *userv1.Identity) ([]redhatcopv1alpha1.UserConfig, error) {
	userConfigList := &redhatcopv1alpha1.UserConfigList{}
	err := r.GetClient().List(context.TODO(), userConfigList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to get all userconfigs")
		return []redhatcopv1alpha1.UserConfig{}, err
	}
	unmatchedUserConfigs := []redhatcopv1alpha1.UserConfig{}

	for _, groupConfig := range userConfigList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&groupConfig.Spec.IdentityExtraSelector)
		if err != nil {
			log.Error(err, "unable to create ", "selector from", groupConfig.Spec.IdentityExtraSelector)
			return []redhatcopv1alpha1.UserConfig{}, err
		}
		labels := labels.Set(instance.GetLabels())
		if !(selector.Matches(labels) || groupConfig.Spec.ProviderName == instance.ProviderName) {
			unmatchedUserConfigs = append(unmatchedUserConfigs, groupConfig)
		}
	}

	return unmatchedUserConfigs, nil
}

func (r *ReconcileIdentity) processTemplatesFor(instance *userv1.Identity, userConfigs []redhatcopv1alpha1.UserConfig) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	user, err := r.getUser(instance)
	if err != nil {
		log.Error(err, "unable to find user for", "identity", instance)
		return []lockedresource.LockedResource{}, err
	}
	for _, userConfig := range userConfigs {
		lrs, err := lockedresource.GetLockedResourcesFromTemplate(userConfig.Spec.Templates, user)
		if err != nil {
			log.Error(err, "unable to process", "templates", userConfig.Spec.Templates, "with param", user)
			return []lockedresource.LockedResource{}, err
		}
		lockedresources = append(lockedresources, lrs...)
	}
	return lockedresources, nil
}

func (r *ReconcileIdentity) getUser(instance *userv1.Identity) (userv1.User, error) {
	userList := &userv1.UserList{}
	err := r.GetClient().List(context.TODO(), userList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to get all users")
		return userv1.User{}, err
	}
	for _, user := range userList.Items {
		if user.GetUID() == instance.User.UID {
			return user, err
		}
	}
	err = errs.New("user not found")
	log.Error(err, "unable to find user for", "identity", instance)
	return userv1.User{}, err
}
