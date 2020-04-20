package groupconfig

import (
	"context"
	errs "errors"

	userv1 "github.com/openshift/api/user/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/pkg/common"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedresource"
	"github.com/scylladb/go-set/strset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerName = "groupconfig-controller"

var log = logf.Log.WithName(controllerName)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new GroupConfig Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileGroupConfig{
		EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor(controllerName)),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {

	reconcileGroupConfig, ok := r.(*ReconcileGroupConfig)
	if !ok {
		err := errs.New("unable to convert to ReconcileUserConfig")
		log.Error(err, "unable to convert to ReconcileUserConfig from ", "reconciler", r)
		return err
	}

	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource GroupConfig
	err = c.Watch(&source.Kind{Type: &redhatcopv1alpha1.GroupConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: "GroupConfig",
		},
	}}, &handler.EnqueueRequestForObject{}, util.ResourceGenerationOrFinalizerChangedPredicate{})
	if err != nil {
		return err
	}

	var groupToGroupConfig = handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			reconcileRequests := []reconcile.Request{}
			group := a.Object.(*userv1.Group)
			groupConfigs, err := reconcileGroupConfig.findApplicableGroupConfigsFromGroup(*group)
			if err != nil {
				log.Error(err, "unable to find applicable GroupConfigs for", "group", group)
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
		})

	// Watch for changes to group
	err = c.Watch(&source.Kind{
		Type: &userv1.Group{
			TypeMeta: metav1.TypeMeta{
				Kind: "Group",
			},
		}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: groupToGroupConfig,
	})
	if err != nil {
		return err
	}

	//if interested in updates from the managed resources
	// watch for changes in status in the locked resources
	err = c.Watch(
		&source.Channel{Source: reconcileGroupConfig.GetStatusChangeChannel()},
		&handler.EnqueueRequestForObject{},
	)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileGroupConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileGroupConfig{}

// ReconcileGroupConfig reconciles a GroupConfig object
type ReconcileGroupConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	lockedresourcecontroller.EnforcingReconciler
}

// Reconcile reads that state of the cluster for a GroupConfig object and makes changes based on the state read
// and what is in the GroupConfig.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileGroupConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling GroupConfig")

	// Fetch the GroupConfig instance
	instance := &redhatcopv1alpha1.GroupConfig{}
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
	selectedGroups, err := r.getSelectedGroups(instance)
	if err != nil {
		log.Error(err, "unable to get users selected by", "UserConfig", instance)
		return r.ManageError(instance, err)
	}

	lockedResources, err := r.getResourceList(instance, selectedGroups)
	if err != nil {
		log.Error(err, "unable to process resources", "UserConfig", instance, "users", selectedGroups)
		return r.ManageError(instance, err)
	}

	err = r.UpdateLockedResources(instance, lockedResources)
	if err != nil {
		log.Error(err, "unable to update locked resources")
		return r.ManageError(instance, err)
	}

	return r.ManageSuccess(instance)
}

func (r *ReconcileGroupConfig) getResourceList(instance *redhatcopv1alpha1.GroupConfig, groups []userv1.Group) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	for _, group := range groups {
		lrs, err := lockedresource.GetLockedResourcesFromTemplates(instance.Spec.Templates, group)
		if err != nil {
			log.Error(err, "unable to process", "templates", instance.Spec.Templates, "with param", group)
			return []lockedresource.LockedResource{}, err
		}
		lockedresources = append(lockedresources, lrs...)
	}
	return lockedresources, nil
}

func (r *ReconcileGroupConfig) getSelectedGroups(instance *redhatcopv1alpha1.GroupConfig) ([]userv1.Group, error) {
	groupList := &userv1.GroupList{}

	labelSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.LabelSelector)
	if err != nil {
		log.Error(err, "unable to create ", "selector from", instance.Spec.LabelSelector)
		return []userv1.Group{}, err
	}

	annotationSelector, err := metav1.LabelSelectorAsSelector(&instance.Spec.AnnotationSelector)
	if err != nil {
		log.Error(err, "unable to create ", "selector from", instance.Spec.AnnotationSelector)
		return []userv1.Group{}, err
	}

	err = r.GetClient().List(context.TODO(), groupList, &client.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Error(err, "unable to get groups with", "selector", labelSelector)
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

func (r *ReconcileGroupConfig) findApplicableGroupConfigsFromGroup(group userv1.Group) ([]redhatcopv1alpha1.GroupConfig, error) {
	groupConfigList := &redhatcopv1alpha1.GroupConfigList{}
	err := r.GetClient().List(context.TODO(), groupConfigList, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to get all userconfigs")
		return []redhatcopv1alpha1.GroupConfig{}, err
	}
	applicableGroupConfigs := []redhatcopv1alpha1.GroupConfig{}

	for _, groupConfig := range groupConfigList.Items {
		labelSelector, err := metav1.LabelSelectorAsSelector(&groupConfig.Spec.LabelSelector)
		if err != nil {
			log.Error(err, "unable to create ", "selector from", groupConfig.Spec.LabelSelector)
			return []redhatcopv1alpha1.GroupConfig{}, err
		}

		annotationSelector, err := metav1.LabelSelectorAsSelector(&groupConfig.Spec.AnnotationSelector)
		if err != nil {
			log.Error(err, "unable to create ", "selector from", groupConfig.Spec.AnnotationSelector)
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

func (r *ReconcileGroupConfig) IsInitialized(instance *redhatcopv1alpha1.GroupConfig) bool {
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

func (r *ReconcileGroupConfig) manageCleanUpLogic(instance *redhatcopv1alpha1.GroupConfig) error {
	err := r.Terminate(instance, true)
	if err != nil {
		log.Error(err, "unable to terminate enforcing reconciler for", "instance", instance)
		return err
	}
	return nil
}
