package namespaceconfig

import (
	"context"

	multierror "github.com/hashicorp/go-multierror"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/pkg/apis/redhatcop/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

var log = logf.Log.WithName("controller_namespaceconfig")

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
		ReconcilerBase: util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme()),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("namespaceconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource NamespaceConfig
	err = c.Watch(&source.Kind{Type: &redhatcopv1alpha1.NamespaceConfig{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	namespaceToNamespaceConfig := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			res := []reconcile.Request{}
			ns := a.Object.(*corev1.Namespace)
			client := mgr.GetClient()
			ncl, err := findApplicableNameSpaceConfigs(*ns, &client)
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
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: namespaceToNamespaceConfig,
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileNamespaceConfig{}

// ReconcileNamespaceConfig reconciles a NamespaceConfig object
type ReconcileNamespaceConfig struct {
	util.ReconcilerBase
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
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	namespaces, err := r.getSelectedNamespaces(instance)
	if err != nil {
		log.Error(err, "unable to retrieve the list of selected namespaces", "selector", instance.Spec.Selector)
		return reconcile.Result{}, err
	}

	res := make(chan error)
	for i := range namespaces {
		go func(ns corev1.Namespace) {
			res <- r.applyConfigToNamespace(instance, ns)
		}(namespaces[i])
	}
	var err1 *multierror.Error
	for range namespaces {
		err := <-res
		if err != nil {
			err1 = multierror.Append(err1, err)
		}
	}
	return reconcile.Result{}, err1.ErrorOrNil()
}

func (r *ReconcileNamespaceConfig) getSelectedNamespaces(namespaceconfig *redhatcopv1alpha1.NamespaceConfig) ([]corev1.Namespace, error) {
	nl := corev1.NamespaceList{}
	selector, err := metav1.LabelSelectorAsSelector(&namespaceconfig.Spec.Selector)
	if err != nil {
		log.Error(err, "unable to create selector from label selector", "selector", &namespaceconfig.Spec.Selector)
		return []corev1.Namespace{}, err
	}
	err = r.GetClient().List(context.TODO(), &client.ListOptions{LabelSelector: selector}, &nl)
	if err != nil {
		log.Error(err, "unable to list namespaces with selector", "selector", selector)
		return []corev1.Namespace{}, err
	}
	return nl.Items, nil
}

func (r *ReconcileNamespaceConfig) applyConfigToNamespace(namespaceconfig *redhatcopv1alpha1.NamespaceConfig, namespace corev1.Namespace) error {
	for _, obj := range namespaceconfig.Spec.Resources {
		object, ok := obj.Object.(metav1.Object)
		if !ok {
			return errors.NewBadRequest("unable to convert raw type to metav1.Object")
		}
		err := r.CreateOrUpdateResource(nil, namespace.Name, object)
		if err != nil {
			return err
		}
	}
	return nil
}

func findApplicableNameSpaceConfigs(namespace corev1.Namespace, c *client.Client) ([]redhatcopv1alpha1.NamespaceConfig, error) {
	//find all the namespaceconfig
	result := []redhatcopv1alpha1.NamespaceConfig{}
	ncl := redhatcopv1alpha1.NamespaceConfigList{}
	err := (*c).List(context.TODO(), &client.ListOptions{}, &ncl)
	if err != nil {
		log.Error(err, "unable to retrieve the list of namespace configs")
		return []redhatcopv1alpha1.NamespaceConfig{}, err
	}
	//for each namespaceconfig see if it selects the namespace
	for _, nc := range ncl.Items {
		selector, err := metav1.LabelSelectorAsSelector(&nc.Spec.Selector)
		if err != nil {
			log.Error(err, "unable to create selector from label selector", "selector", &nc.Spec.Selector)
			return []redhatcopv1alpha1.NamespaceConfig{}, err
		}
		if selector.Matches(labels.Set(namespace.Labels)) {
			result = append(result, nc)
		}
	}
	return result, nil
}
