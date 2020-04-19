package group

import (
	"context"

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

const controllerName = "group-controller"

var log = logf.Log.WithName(controllerName)

/**
The sole purpose of this controller is to clean up objects related a GroupConfig in the evnt that a group for some reason stops qualifiying for it.
*/

// Add creates a new Group Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileGroup{util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor(controllerName))}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Group
	err = c.Watch(&source.Kind{Type: &userv1.Group{
		TypeMeta: metav1.TypeMeta{
			Kind: "Group",
		},
	}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileGroup implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileGroup{}

// ReconcileGroup reconciles a Group object
type ReconcileGroup struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	util.ReconcilerBase
}

// Reconcile reads that state of the cluster for a Group object and makes changes based on the state read
// and what is in the Group.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileGroup) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Group")

	// Fetch the Group instance
	instance := &userv1.Group{}
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

	// this can only be woken up when this group was qualifying for a group config and then it doesn's anymore
	// we need to find out all of the group config it does not match and make sure the objects that it would create do not exist

	unMatchingGroupConfigs, err := r.getUnMatchingGroupConfigs(instance)
	if err != nil {
		log.Error(err, "unable to retrieve unmatching UserConfigs for", "instance", instance)
		return r.ManageError(instance, err)
	}

	toBeDeleteResources, err := r.processTemplatesFor(instance, unMatchingGroupConfigs)
	if err != nil {
		log.Error(err, "unable to process templates for", "instance", instance, "unMatchingUserConfigs", unMatchingGroupConfigs)
		return r.ManageError(instance, err)
	}

	err = r.DeleteResourcesIfExist(common.GetResources(toBeDeleteResources))
	if err != nil {
		log.Error(err, "unable to delete resources", "resources", toBeDeleteResources)
		return r.ManageError(instance, err)
	}

	return r.ManageSuccess(instance)
}

func (r *ReconcileGroup) getUnMatchingGroupConfigs(instance *userv1.Group) ([]redhatcopv1alpha1.GroupConfig, error) {
	groupConfigList := &redhatcopv1alpha1.GroupConfigList{}
	err := r.GetClient().List(context.TODO(), groupConfigList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to get all groupconfigs")
		return []redhatcopv1alpha1.GroupConfig{}, err
	}
	unmatchedGroupConfigs := []redhatcopv1alpha1.GroupConfig{}

	for _, groupConfig := range groupConfigList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&groupConfig.Spec.LabelSelector)
		if err != nil {
			log.Error(err, "unable to create ", "selector from", groupConfig.Spec.LabelSelector)
			return []redhatcopv1alpha1.GroupConfig{}, err
		}
		labels := labels.Set(instance.GetLabels())
		if !selector.Matches(labels) {
			unmatchedGroupConfigs = append(unmatchedGroupConfigs, groupConfig)
		}
	}

	return unmatchedGroupConfigs, nil
}

func (r *ReconcileGroup) processTemplatesFor(instance *userv1.Group, groupConfigs []redhatcopv1alpha1.GroupConfig) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	for _, groupConfig := range groupConfigs {
		lrs, err := lockedresource.GetLockedResourcesFromTemplate(groupConfig.Spec.Templates, instance)
		if err != nil {
			log.Error(err, "unable to process", "templates", groupConfig.Spec.Templates, "with param", instance)
			return []lockedresource.LockedResource{}, err
		}
		lockedresources = append(lockedresources, lrs...)
	}
	return lockedresources, nil
}
