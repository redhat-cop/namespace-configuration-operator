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

package common

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	utilsapi "github.com/redhat-cop/operator-utils/api/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util/apis"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// EnforcingReconcilerInterface defines the interface that reconcilers must implement
// to use the centralized helper functions. This interface is satisfied by any struct
// that embeds lockedresourcecontroller.EnforcingReconciler.
type EnforcingReconcilerInterface interface {
	GetClient() client.Client
	ManageSuccess(ctx context.Context, obj client.Object) (reconcile.Result, error)
}

// LogReconcilingStarted logs the "reconciling started" message naming the resource.
func LogReconcilingStarted(log logr.Logger, resourceTypeName string, namespacedName types.NamespacedName) {
	log.Info("reconciling started", resourceTypeName, namespacedName.Name)
}

// LogResourcesProcessedSuccessfully logs the "resources processed successfully" message
// with resource type name, instance name, selected items count, resources count, and selected items label.
func LogResourcesProcessedSuccessfully(log logr.Logger, resourceTypeName string, instanceName string, selectedItemsCount int, resourcesCount int, selectedItemsLabel string) {
	log.Info("resources processed successfully", resourceTypeName, instanceName, selectedItemsLabel, selectedItemsCount, "resources", resourcesCount)
}

// ManageSuccessWithRetry attempts to call ManageSuccess with retry logic to handle
// optimistic concurrency conflicts. It re-fetches the instance before each retry
// to ensure we have the latest resourceVersion.
//
// This is a generic function that works with any controller type (GroupConfig, NamespaceConfig, UserConfig)
// by using Go generics. The resourceTypeName parameter ensures proper logging for each controller type.
//
// Parameters:
//   - reconciler: A reconciler that implements EnforcingReconcilerInterface (embeds lockedresourcecontroller.EnforcingReconciler)
//   - ctx: Context for the operation
//   - req: Controller request with the resource's namespaced name
//   - log: Logger instance
//   - resourceTypeName: The resource type name for logging (e.g., "groupconfig", "namespaceconfig", "userconfig")
//   - reconciledGeneration: the generation that was actually processed; if the object has moved on
//     since, no success is written for it (the newer generation's reconcile is already queued)
//   - newInstance: Factory function that creates a new instance of type T
//
// Returns:
//   - reconcile.Result and error from ManageSuccess, or error from retry logic
func ManageSuccessWithRetry[T client.Object](
	reconciler EnforcingReconcilerInterface,
	ctx context.Context,
	req ctrl.Request,
	log logr.Logger,
	resourceTypeName string,
	reconciledGeneration int64,
	newInstance func() T,
) (reconcile.Result, error) {
	const maxRetries = 5
	const baseDelay = 50 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Re-fetch the instance to get the latest resourceVersion
		latestInstance := newInstance()
		err := reconciler.GetClient().Get(ctx, req.NamespacedName, latestInstance)
		if err != nil {
			if errors.IsNotFound(err) {
				// Resource was deleted, no need to update status
				return reconcile.Result{}, nil
			}
			log.Error(err, "unable to re-fetch instance for status update", "attempt", attempt+1)
			return reconcile.Result{}, err
		}

		// The library stamps ObservedGeneration from the object it is handed. If the spec changed
		// between render and this write, writing success now would claim the NEW generation was
		// processed. Its reconcile is already queued by the generation predicate; let it report.
		if latestInstance.GetGeneration() != reconciledGeneration {
			// No Requeue here. The update that moved the generation passed the generation predicate
			// and is already in the workqueue (as a dirty mark if it arrived mid-reconcile, so Done
			// re-adds it at once). A Requeue on top is an AddRateLimited that Forget does not cancel:
			// measured in review, it turned two reconciles into three and wrote the same status twice.
			log.V(1).Info("spec changed during reconcile; leaving status to the next cycle", resourceTypeName, latestInstance.GetName(), "reconciled", reconciledGeneration, "current", latestInstance.GetGeneration())
			return reconcile.Result{}, nil
		}

		// A previous cycle may have left ReconcileError=True. The library's ManageSuccess only adds or
		// replaces ReconcileSuccess, so without this the CR would read as both succeeded and failed
		// forever after a recovered error. Mark the error condition False; ManageSuccess carries it.
		clearReconcileError(latestInstance)

		// Attempt to update status
		result, err := reconciler.ManageSuccess(ctx, latestInstance)
		if err == nil {
			// Success!
			if attempt > 0 {
				log.V(1).Info("ManageSuccess succeeded after retry", "attempt", attempt+1, resourceTypeName, latestInstance.GetName())
			}
			return result, nil
		}

		// Check if this is a conflict error that we should retry
		if errors.IsConflict(err) {
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

// clearReconcileError flips a standing ReconcileError=True condition to False on the instance's
// status (in memory; the caller's status update persists it). Objects without the enforcing status
// are left alone. The condition is kept rather than removed so its history stays visible, as the
// Kubernetes conditions convention expects.
func clearReconcileError(obj client.Object) {
	aware, ok := obj.(utilsapi.EnforcingReconcileStatusAware)
	if !ok {
		return
	}
	status := aware.GetEnforcingReconcileStatus()
	for _, c := range status.Conditions {
		if c.Type == apis.ReconcileError && c.Status == metav1.ConditionTrue {
			status.Conditions = apis.AddOrReplaceCondition(metav1.Condition{
				Type:               apis.ReconcileError,
				Status:             metav1.ConditionFalse,
				Reason:             apis.ReconcileSuccessReason,
				Message:            "",
				ObservedGeneration: obj.GetGeneration(),
				LastTransitionTime: metav1.Now(),
			}, status.Conditions)
			aware.SetEnforcingReconcileStatus(status)
			return
		}
	}
}
