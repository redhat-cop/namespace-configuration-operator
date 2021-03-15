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
	"strings"

	"github.com/go-logr/logr"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/controllers/common"
	"github.com/redhat-cop/operator-utils/pkg/util"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedpatch"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedresource"
	"github.com/scylladb/go-set/strset"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// NamespaceConfigReconciler reconciles a NamespaceConfig object
type NamespaceConfigReconciler struct {
	lockedresourcecontroller.EnforcingReconciler
	Log                   logr.Logger
	controllerName        string
	AllowSystemNamespaces bool
}

// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=namespaceconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=namespaceconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=redhatcop.redhat.io,resources=namespaceconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=*,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NamespaceConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *NamespaceConfigReconciler) Reconcile(context context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespaceconfig", req.NamespacedName)
	log.Info("reconciling started")
	// Fetch the NamespaceConfig instance
	instance := &redhatcopv1alpha1.NamespaceConfig{}
	err := r.GetClient().Get(context, req.NamespacedName, instance)
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
	//get selected namespaces
	selectedNamespaces, err := r.getSelectedNamespaces(context, instance)
	if err != nil {
		log.Error(err, "unable to get namespaces selected by", "NamespaceConfig", instance)
		return r.ManageError(context, instance, err)
	}

	lockedResources, err := r.getResourceList(instance, selectedNamespaces)
	if err != nil {
		log.Error(err, "unable to process resources", "NamespaceConfig", instance, "namespaces", selectedNamespaces)
		return r.ManageError(context, instance, err)
	}

	err = r.UpdateLockedResources(context, instance, lockedResources, []lockedpatch.LockedPatch{})
	if err != nil {
		log.Error(err, "unable to update locked resources")
		return r.ManageError(context, instance, err)
	}

	return r.ManageSuccess(context, instance)
}

func (r *NamespaceConfigReconciler) manageCleanUpLogic(instance *redhatcopv1alpha1.NamespaceConfig) error {
	err := r.Terminate(instance, true)
	if err != nil {
		r.Log.Error(err, "unable to terminate enforcing reconciler for", "instance", instance)
		return err
	}
	return nil
}

// IsInitialized none
func (r *NamespaceConfigReconciler) IsInitialized(instance *redhatcopv1alpha1.NamespaceConfig) bool {
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

func (r *NamespaceConfigReconciler) getResourceList(instance *redhatcopv1alpha1.NamespaceConfig, groups []corev1.Namespace) ([]lockedresource.LockedResource, error) {
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

func (r *NamespaceConfigReconciler) getSelectedNamespaces(context context.Context, namespaceconfig *redhatcopv1alpha1.NamespaceConfig) ([]corev1.Namespace, error) {
	nl := corev1.NamespaceList{}
	selector, err := metav1.LabelSelectorAsSelector(&namespaceconfig.Spec.LabelSelector)
	if err != nil {
		r.Log.Error(err, "unable to create selector from label selector", "selector", &namespaceconfig.Spec.LabelSelector)
		return []corev1.Namespace{}, err
	}

	annotationSelector, err := metav1.LabelSelectorAsSelector(&namespaceconfig.Spec.AnnotationSelector)
	if err != nil {
		r.Log.Error(err, "unable to create ", "selector from", namespaceconfig.Spec.AnnotationSelector)
		return []corev1.Namespace{}, err
	}

	err = r.GetClient().List(context, &nl, &client.ListOptions{LabelSelector: selector})
	if err != nil {
		r.Log.Error(err, "unable to list namespaces with selector", "selector", selector)
		return []corev1.Namespace{}, err
	}

	selectedNamespaces := []corev1.Namespace{}

	for _, namespace := range nl.Items {
		annotationsAsLabels := labels.Set(namespace.Annotations)
		if annotationSelector.Matches(annotationsAsLabels) && (r.AllowSystemNamespaces || !isProhibitedNamespaceName(namespace.GetName())) {
			selectedNamespaces = append(selectedNamespaces, namespace)
		}
	}

	return selectedNamespaces, nil
}

func (r *NamespaceConfigReconciler) findApplicableNameSpaceConfigs(namespace corev1.Namespace) ([]redhatcopv1alpha1.NamespaceConfig, error) {
	if !r.AllowSystemNamespaces && isProhibitedNamespaceName(namespace.GetName()) {
		return []redhatcopv1alpha1.NamespaceConfig{}, nil
	}
	//find all the namespaceconfig
	result := []redhatcopv1alpha1.NamespaceConfig{}
	ncl := redhatcopv1alpha1.NamespaceConfigList{}
	err := r.GetClient().List(context.TODO(), &ncl, &client.ListOptions{})
	if err != nil {
		r.Log.Error(err, "unable to retrieve the list of namespace configs")
		return []redhatcopv1alpha1.NamespaceConfig{}, err
	}
	//for each namespaceconfig see if it selects the namespace
	for _, nc := range ncl.Items {
		labelSelector, err := metav1.LabelSelectorAsSelector(&nc.Spec.LabelSelector)
		if err != nil {
			r.Log.Error(err, "unable to create selector from label selector", "selector", &nc.Spec.LabelSelector)
			return []redhatcopv1alpha1.NamespaceConfig{}, err
		}
		annotationSelector, err := metav1.LabelSelectorAsSelector(&nc.Spec.AnnotationSelector)
		if err != nil {
			r.Log.Error(err, "unable to create ", "selector from", nc.Spec.AnnotationSelector)
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
	return name == "default" || strings.HasPrefix(name, "openshift-") || strings.HasPrefix(name, "kube-")
}

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.controllerName = "namespaceconfig-controller"
	return ctrl.NewControllerManagedBy(mgr).
		For(&redhatcopv1alpha1.NamespaceConfig{}, builder.WithPredicates(util.ResourceGenerationOrFinalizerChangedPredicate{})).
		Watches(&source.Kind{Type: &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind: "Namespace",
			},
		}}, handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
			res := []reconcile.Request{}
			ns := a.(*corev1.Namespace)
			ncl, err := r.findApplicableNameSpaceConfigs(*ns)
			if err != nil {
				r.Log.Error(err, "unable to find applicable NamespaceConfig for namespace", "namespace", ns.Name)
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
		})).
		Watches(&source.Channel{Source: r.GetStatusChangeChannel()}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
