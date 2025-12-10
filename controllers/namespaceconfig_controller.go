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
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/controllers/common"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
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

// manageSuccessWithRetry attempts to call ManageSuccess with retry logic to handle
// optimistic concurrency conflicts. It re-fetches the instance before each retry
// to ensure we have the latest resourceVersion.
func (r *NamespaceConfigReconciler) manageSuccessWithRetry(ctx context.Context, req ctrl.Request, log logr.Logger) (reconcile.Result, error) {
	const maxRetries = 5
	const baseDelay = 50 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Re-fetch the instance to get the latest resourceVersion
		latestInstance := &redhatcopv1alpha1.NamespaceConfig{}
		err := r.GetClient().Get(ctx, req.NamespacedName, latestInstance)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Resource was deleted, no need to update status
				return reconcile.Result{}, nil
			}
			log.Error(err, "unable to re-fetch instance for status update", "attempt", attempt+1)
			return reconcile.Result{}, err
		}

		// Attempt to update status
		result, err := r.ManageSuccess(ctx, latestInstance)
		if err == nil {
			// Success!
			if attempt > 0 {
				log.V(1).Info("ManageSuccess succeeded after retry", "attempt", attempt+1, "namespaceconfig", latestInstance.Name)
			}
			return result, nil
		}

		// Check if this is a conflict error that we should retry
		if apierrors.IsConflict(err) {
			if attempt < maxRetries-1 {
				// Calculate exponential backoff delay
				delay := baseDelay * time.Duration(1<<uint(attempt))
				log.V(1).Info("retrying ManageSuccess due to conflict", "attempt", attempt+1, "maxRetries", maxRetries, "delay", delay, "error", err)
				time.Sleep(delay)
				continue
			}
			// Last attempt failed, return the error
			log.Error(err, "unable to update status after retries", "attempts", maxRetries)
			return reconcile.Result{}, err
		}

		// Not a conflict error, return immediately
		return result, err
	}

	// Should never reach here, but just in case
	return reconcile.Result{}, nil
}

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
			log.Info("resource deletion detected - resource not found, skipping reconciliation", "namespaceconfig", req.NamespacedName)
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
		log.Info("resource deletion detected - processing deletion cleanup", "namespaceconfig", instance.Name, "deletionTimestamp", instance.DeletionTimestamp)
		// Support all old finalizer variants for backward compatibility
		oldFinalizerVariants := []string{
			"namespaceconfig-controller",
			"namespaceconfig-controller.redhat.com",
			"namespaceconfig-controller.redhatcop.redhat.io",
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

		err := r.manageCleanUpLogic(instance)
		if err != nil {
			log.Error(err, "unable to delete instance", "instance", instance)
			return r.ManageError(context, instance, err)
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

		err = r.GetClient().Update(context, instance)
		if err != nil {
			// If the resource is already deleted (NotFound), that's fine - just return success
			if apierrors.IsNotFound(err) {
				log.Info("resource deletion completed - resource already deleted during finalizer removal", "namespaceconfig", instance.Name)
				return reconcile.Result{}, nil
			}
			log.Error(err, "unable to update instance", "instance", instance)
			return r.ManageError(context, instance, err)
		}
		log.Info("resource deletion completed successfully", "namespaceconfig", instance.Name)
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

	log.Info("resources processed successfully", "namespaceconfig", instance.Name, "namespaces", len(selectedNamespaces), "resources", len(lockedResources))

	// Use retry mechanism to handle optimistic concurrency conflicts
	// This re-fetches the instance before each retry to ensure we have the latest resourceVersion
	return r.manageSuccessWithRetry(context, req, log)
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

	// Migrate old finalizer to new finalizer (only if not being deleted)
	oldFinalizerName := "namespaceconfig-controller"
	if !util.IsBeingDeleted(instance) && util.HasFinalizer(instance, oldFinalizerName) {
		util.RemoveFinalizer(instance, oldFinalizerName)
		util.AddFinalizer(instance, r.controllerName)
		needsUpdate = false
	}

	// Only add/remove finalizers if not being deleted
	if !util.IsBeingDeleted(instance) {
		if len(instance.Spec.Templates) > 0 && !util.HasFinalizer(instance, r.controllerName) {
			util.AddFinalizer(instance, r.controllerName)
			needsUpdate = false
		}
		if len(instance.Spec.Templates) == 0 && util.HasFinalizer(instance, r.controllerName) {
			util.RemoveFinalizer(instance, r.controllerName)
			needsUpdate = false
		}
	}

	return needsUpdate
}

func (r *NamespaceConfigReconciler) getResourceList(instance *redhatcopv1alpha1.NamespaceConfig, namespaces []corev1.Namespace) ([]lockedresource.LockedResource, error) {
	lockedresources := []lockedresource.LockedResource{}
	for _, namespace := range namespaces {
		// Filter templates that are applicable to this namespace BEFORE processing
		applicableTemplates := r.filterApplicableTemplates(instance.Spec.Templates, namespace)

		// Only process templates that are actually applicable
		if len(applicableTemplates) > 0 {
			lrs, err := lockedresource.GetLockedResourcesFromTemplatesWithRestConfig(applicableTemplates, r.GetRestConfig(), namespace)
			if err != nil {
				r.Log.Error(err, "unable to process", "templates", applicableTemplates, "with param", namespace)
				return []lockedresource.LockedResource{}, err
			}
			lockedresources = append(lockedresources, lrs...)
		} else {
			// Namespace is being skipped because no templates in this NamespaceConfig match the namespace's pattern
			// This is logged at V(1) level to be visible but not too verbose
			r.Log.V(1).Info("skipping namespace - no NamespaceConfig templates match the namespace pattern",
				"namespace", namespace.Name,
				"namespaceconfig", instance.Name)
		}
	}
	return lockedresources, nil
}

// Dynamic template filtering based on extracted patterns from template content
func (r *NamespaceConfigReconciler) filterApplicableTemplates(templates []apis.LockedResourceTemplate, namespace corev1.Namespace) []apis.LockedResourceTemplate {
	applicableTemplates := []apis.LockedResourceTemplate{}

	for _, template := range templates {
		if r.isTemplateApplicableToNamespace(template, namespace) {
			applicableTemplates = append(applicableTemplates, template)
		}
	}

	return applicableTemplates
}

// Dynamically check if template is applicable by extracting patterns from template content
func (r *NamespaceConfigReconciler) isTemplateApplicableToNamespace(template apis.LockedResourceTemplate, namespace corev1.Namespace) bool {
	templateContent := template.ObjectTemplate
	namespaceName := namespace.Name

	// Extract both hasSuffix and contains patterns
	suffixPatterns := r.extractHasSuffixPatterns(templateContent)
	containsPatterns := r.extractContainsPatterns(templateContent)

	// Debug logging for template filtering (V(2) - only shown with --zap-log-level=2 or higher)
	// To enable: ./bin/manager --zap-log-level=2
	// Or set environment variable: ZAP_LOG_LEVEL=2
	r.Log.V(2).Info("checking template applicability",
		"namespace", namespaceName,
		"suffixPatterns", suffixPatterns,
		"containsPatterns", containsPatterns,
		"templatePreview", func() string {
			if len(templateContent) > 100 {
				return templateContent[:100] + "..."
			}
			return templateContent
		}())

	// If no conditional patterns found, template applies to all namespaces
	if len(suffixPatterns) == 0 && len(containsPatterns) == 0 {
		// Check for unrecognized conditional logic
		if strings.Contains(templateContent, "{{- if") || strings.Contains(templateContent, "{{ if") {
			r.Log.V(2).Info("template contains unrecognized conditional logic, applying to all namespaces (relying on template rendering)", "namespace", namespaceName)
			return true
		}
		r.Log.V(2).Info("template has no patterns, applying to all namespaces", "namespace", namespaceName)
		return true
	}

	// Detect if template uses AND logic (requires all conditions to match)
	// vs OR logic (requires any condition to match)
	// Look for "and" keyword in conditional statements
	usesAndLogic := strings.Contains(templateContent, "{{- if and") || strings.Contains(templateContent, "{{ if and")

	if usesAndLogic {
		// AND logic: ALL patterns must match
		allSuffixMatch := true
		if len(suffixPatterns) > 0 {
			for _, pattern := range suffixPatterns {
				if !strings.HasSuffix(namespaceName, pattern) {
					allSuffixMatch = false
					break
				}
			}
		} else {
			// If no suffix patterns are defined, they are considered to match if no other patterns are defined.
			// If there are contains patterns, this will be handled below.
			// If there are no patterns at all, it would have returned true earlier.
			allSuffixMatch = true
		}

		allContainsMatch := true
		if len(containsPatterns) > 0 {
			for _, pattern := range containsPatterns {
				if !strings.Contains(namespaceName, pattern) {
					allContainsMatch = false
					break
				}
			}
		} else {
			allContainsMatch = true
		}

		if allSuffixMatch && allContainsMatch {
			r.Log.V(2).Info("namespace matches all AND logic patterns", "namespace", namespaceName)
			return true
		}
		r.Log.V(2).Info("namespace does not match all AND logic patterns", "namespace", namespaceName)
		return false

	} else {
		// OR logic: ANY pattern can match (original behavior)
		// Check hasSuffix patterns
		for _, pattern := range suffixPatterns {
			if strings.HasSuffix(namespaceName, pattern) {
				r.Log.V(2).Info("namespace matches hasSuffix pattern",
					"namespace", namespaceName,
					"pattern", pattern)
				return true
			}
		}

		// Check contains patterns
		for _, pattern := range containsPatterns {
			if strings.Contains(namespaceName, pattern) {
				r.Log.V(2).Info("namespace matches contains pattern",
					"namespace", namespaceName,
					"pattern", pattern)
				return true
			}
		}
	}

	// Namespace doesn't match any patterns
	r.Log.V(2).Info("namespace does not match any template patterns",
		"namespace", namespaceName,
		"suffixPatterns", suffixPatterns,
		"containsPatterns", containsPatterns)
	return false
}

// Extract all hasSuffix patterns from template content
func (r *NamespaceConfigReconciler) extractHasSuffixPatterns(templateContent string) []string {
	patterns := []string{}

	// Regex to match: hasSuffix "some-pattern" or hasSuffix "-some-pattern"
	// Handles both: {{- if hasSuffix "-cluster-admin" .Name }} and similar patterns
	re := regexp.MustCompile(`hasSuffix\s+"([^"]+)"`)
	matches := re.FindAllStringSubmatch(templateContent, -1)

	for _, match := range matches {
		if len(match) > 1 {
			patterns = append(patterns, match[1])
		}
	}

	return patterns
}

// Extract contains patterns for templates using 'contains' instead of 'hasSuffix'
func (r *NamespaceConfigReconciler) extractContainsPatterns(templateContent string) []string {
	patterns := []string{}

	// Regex to match: contains "some-pattern" or contains "-some-pattern"
	re := regexp.MustCompile(`contains\s+"([^"]+)"`)
	matches := re.FindAllStringSubmatch(templateContent, -1)

	for _, match := range matches {
		if len(match) > 1 {
			patterns = append(patterns, match[1])
		}
	}

	return patterns
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

func (r *NamespaceConfigReconciler) findApplicableNameSpaceConfigs(ctx context.Context, namespace corev1.Namespace) ([]redhatcopv1alpha1.NamespaceConfig, error) {
	if !r.AllowSystemNamespaces && isProhibitedNamespaceName(namespace.GetName()) {
		return []redhatcopv1alpha1.NamespaceConfig{}, nil
	}
	//find all the namespaceconfig
	result := []redhatcopv1alpha1.NamespaceConfig{}
	ncl := redhatcopv1alpha1.NamespaceConfigList{}
	err := r.GetClient().List(ctx, &ncl, &client.ListOptions{})
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
	r.controllerName = "redhatcop.redhat.io/namespaceconfig-controller"
	return ctrl.NewControllerManagedBy(mgr).
		For(&redhatcopv1alpha1.NamespaceConfig{}, builder.WithPredicates(common.ResourceGenerationOrFinalizerOrDeletionTimestampChangedPredicate)).
		Watches(&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind: "Namespace",
			},
		}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
			res := []reconcile.Request{}
			ns := a.(*corev1.Namespace)
			ncl, err := r.findApplicableNameSpaceConfigs(ctx, *ns)
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
		WatchesRawSource(&source.Channel{Source: r.GetStatusChangeChannel()}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
