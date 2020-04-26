package namespaceconfig

import (
	"context"
	"errors"
	"strings"

	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/pkg/common"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedresource"
	"github.com/scylladb/go-set/strset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const controllerName = "namespace-config-operator"

var log = logf.Log.WithName(controllerName)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new NamespaceConfig Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {

	return &ReconcileNamespaceConfig{
		EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetEventRecorderFor(controllerName)),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	reconcileNamespaceConfig, ok := r.(*ReconcileNamespaceConfig)
	if !ok {
		return errors.New("unable to convert to ReconcileNamespaceConfig")
	}
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource NamespaceConfig
	err = c.Watch(&source.Kind{Type: &redhatcopv1alpha1.NamespaceConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: "NamespaceConfig",
		},
	}}, &handler.EnqueueRequestForObject{}, util.ResourceGenerationOrFinalizerChangedPredicate{})
	if err != nil {
		return err
	}

	namespaceToNamespaceConfig := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			res := []reconcile.Request{}
			ns := a.Object.(*corev1.Namespace)
			ncl, err := reconcileNamespaceConfig.findApplicableNameSpaceConfigs(*ns)
			if err != nil {
				log.Error(err, "unable to find applicable NamespaceConfig for namespace", "namespace", ns.Name)
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
		})

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner NamespaceConfig
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind: "Namespace",
		},
	}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: namespaceToNamespaceConfig,
	})
	if err != nil {
		return err
	}

	//if interested in updates from the managed resources
	// watch for changes in status in the locked resources
	err = c.Watch(
		&source.Channel{Source: reconcileNamespaceConfig.GetStatusChangeChannel()},
		&handler.EnqueueRequestForObject{},
	)

	return nil
}

var _ reconcile.Reconciler = &ReconcileNamespaceConfig{}

// ReconcileNamespaceConfig reconciles a NamespaceConfig object
type ReconcileNamespaceConfig struct {
	lockedresourcecontroller.EnforcingReconciler
}

// Reconcile reads that state of the cluster for a NamespaceConfig object and makes changes based on the state read
// and what is in the NamespaceConfig.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNamespaceConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling NamespaceConfig")

	// Fetch the NamespaceConfig instance
	instance := &redhatcopv1alpha1.NamespaceConfig{}
	err := r.GetClient().Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
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
	selectedNamespaces, err := r.getSelectedNamespaces(instance)
	if err != nil {
		log.Error(err, "unable to get namespaces selected by", "NamespaceConfig", instance)
		return r.ManageError(instance, err)
	}

	lockedResources, err := r.getResourceList(instance, selectedNamespaces)
	if err != nil {
		log.Error(err, "unable to process resources", "NamespaceConfig", instance, "namespaces", selectedNamespaces)
		return r.ManageError(instance, err)
	}

	err = r.UpdateLockedResources(instance, lockedResources)
	if err != nil {
		log.Error(err, "unable to update locked resources")
		return r.ManageError(instance, err)
	}

	return r.ManageSuccess(instance)
}

func (r *ReconcileNamespaceConfig) manageCleanUpLogic(instance *redhatcopv1alpha1.NamespaceConfig) error {
	err := r.Terminate(instance, true)
	if err != nil {
		log.Error(err, "unable to terminate enforcing reconciler for", "instance", instance)
		return err
	}
	return nil
}

func (r *ReconcileNamespaceConfig) IsInitialized(instance *redhatcopv1alpha1.NamespaceConfig) bool {
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

func (r *ReconcileNamespaceConfig) getResourceList(instance *redhatcopv1alpha1.NamespaceConfig, groups []corev1.Namespace) ([]lockedresource.LockedResource, error) {
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

func (r *ReconcileNamespaceConfig) getSelectedNamespaces(namespaceconfig *redhatcopv1alpha1.NamespaceConfig) ([]corev1.Namespace, error) {
	nl := corev1.NamespaceList{}
	selector, err := metav1.LabelSelectorAsSelector(&namespaceconfig.Spec.LabelSelector)
	if err != nil {
		log.Error(err, "unable to create selector from label selector", "selector", &namespaceconfig.Spec.LabelSelector)
		return []corev1.Namespace{}, err
	}

	annotationSelector, err := metav1.LabelSelectorAsSelector(&namespaceconfig.Spec.AnnotationSelector)
	if err != nil {
		log.Error(err, "unable to create ", "selector from", namespaceconfig.Spec.AnnotationSelector)
		return []corev1.Namespace{}, err
	}

	err = r.GetClient().List(context.TODO(), &nl, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		log.Error(err, "unable to list namespaces with selector", "selector", selector)
		return []corev1.Namespace{}, err
	}

	selectedNamespaces := []corev1.Namespace{}

	for _, namespace := range nl.Items {
		annotationsAsLabels := labels.Set(namespace.Annotations)
		if annotationSelector.Matches(annotationsAsLabels) && !isProhibitedNamespaceName(namespace.GetName()) {
			selectedNamespaces = append(selectedNamespaces, namespace)
		}
	}

	return selectedNamespaces, nil
}

func (r *ReconcileNamespaceConfig) findApplicableNameSpaceConfigs(namespace corev1.Namespace) ([]redhatcopv1alpha1.NamespaceConfig, error) {
	if isProhibitedNamespaceName(namespace.GetName()) {
		return []redhatcopv1alpha1.NamespaceConfig{}, nil
	}
	//find all the namespaceconfig
	result := []redhatcopv1alpha1.NamespaceConfig{}
	ncl := redhatcopv1alpha1.NamespaceConfigList{}
	err := r.GetClient().List(context.TODO(), &ncl, &client.ListOptions{})
	if err != nil {
		log.Error(err, "unable to retrieve the list of namespace configs")
		return []redhatcopv1alpha1.NamespaceConfig{}, err
	}
	//for each namespaceconfig see if it selects the namespace
	for _, nc := range ncl.Items {
		labelSelector, err := metav1.LabelSelectorAsSelector(&nc.Spec.LabelSelector)
		if err != nil {
			log.Error(err, "unable to create selector from label selector", "selector", &nc.Spec.LabelSelector)
			return []redhatcopv1alpha1.NamespaceConfig{}, err
		}
		annotationSelector, err := metav1.LabelSelectorAsSelector(&nc.Spec.AnnotationSelector)
		if err != nil {
			log.Error(err, "unable to create ", "selector from", nc.Spec.AnnotationSelector)
			return []redhatcopv1alpha1.NamespaceConfig{}, err
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
	return name == "default" || strings.HasPrefix(name, "openshift") || strings.HasPrefix(name, "kube")
}
