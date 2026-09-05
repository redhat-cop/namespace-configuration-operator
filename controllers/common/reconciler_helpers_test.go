//go:build !integration
// +build !integration

package common

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	utilsapi "github.com/redhat-cop/operator-utils/api/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// recordingReconciler stands in for the library: ManageSuccess sees the instance exactly as the
// helper hands it over, which is what the library then serialises into the status update.
type recordingReconciler struct {
	c    client.Client
	seen []metav1.Condition
}

func (r *recordingReconciler) GetClient() client.Client { return r.c }
func (r *recordingReconciler) ManageSuccess(_ context.Context, obj client.Object) (reconcile.Result, error) {
	r.seen = obj.(utilsapi.EnforcingReconcileStatusAware).GetEnforcingReconcileStatus().Conditions
	return reconcile.Result{}, nil
}

func TestManageSuccessWithRetry_ClearsAStandingReconcileError(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := redhatcopv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	nc := &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc", Generation: 3}}
	nc.SetEnforcingReconcileStatus(utilsapi.EnforcingReconcileStatus{Conditions: []metav1.Condition{
		{Type: apis.ReconcileError, Status: metav1.ConditionTrue, Reason: apis.ReconcileErrorReason, Message: "template 0 failed", LastTransitionTime: metav1.Now()},
	}})
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nc).Build()
	r := &recordingReconciler{c: c}

	_, err := ManageSuccessWithRetry(r, context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "nc"}}, logr.Discard(), "namespaceconfig", 3,
		func() *redhatcopv1alpha1.NamespaceConfig { return &redhatcopv1alpha1.NamespaceConfig{} })
	if err != nil {
		t.Fatal(err)
	}
	var errCond *metav1.Condition
	for i := range r.seen {
		if r.seen[i].Type == apis.ReconcileError {
			errCond = &r.seen[i]
		}
	}
	if errCond == nil {
		t.Fatal("the ReconcileError condition must be kept (as False), not dropped")
	}
	if errCond.Status != metav1.ConditionFalse || errCond.Reason != apis.ReconcileSuccessReason || errCond.Message != "" {
		t.Errorf("ReconcileError must read False/%s with an empty message after success, got %s/%s %q", apis.ReconcileSuccessReason, errCond.Status, errCond.Reason, errCond.Message)
	}
	if errCond.ObservedGeneration != 3 {
		t.Errorf("observedGeneration should follow the re-fetched object, got %d", errCond.ObservedGeneration)
	}
}

func TestManageSuccessWithRetry_LeavesConditionsAloneWithoutAnError(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := redhatcopv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	nc := &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc"}}
	nc.SetEnforcingReconcileStatus(utilsapi.EnforcingReconcileStatus{Conditions: []metav1.Condition{
		{Type: apis.ReconcileSuccess, Status: metav1.ConditionTrue, Reason: apis.ReconcileSuccessReason, LastTransitionTime: metav1.Now()},
	}})
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nc).Build()
	r := &recordingReconciler{c: c}
	if _, err := ManageSuccessWithRetry(r, context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "nc"}}, logr.Discard(), "namespaceconfig", 0,
		func() *redhatcopv1alpha1.NamespaceConfig { return &redhatcopv1alpha1.NamespaceConfig{} }); err != nil {
		t.Fatal(err)
	}
	if len(r.seen) != 1 || r.seen[0].Type != apis.ReconcileSuccess {
		t.Errorf("no ReconcileError must be invented, got %+v", r.seen)
	}
}

func TestManageSuccessWithRetry_SkipsAStaleGeneration(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := redhatcopv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	nc := &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc", Generation: 5}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nc).Build()
	r := &recordingReconciler{c: c}
	res, err := ManageSuccessWithRetry(r, context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "nc"}}, logr.Discard(), "namespaceconfig", 4,
		func() *redhatcopv1alpha1.NamespaceConfig { return &redhatcopv1alpha1.NamespaceConfig{} })
	if err != nil {
		t.Fatal(err)
	}
	if r.seen != nil {
		t.Errorf("success must not be written for generation 4 when the object is at 5, but ManageSuccess was called")
	}
	if res.Requeue || res.RequeueAfter != 0 {
		t.Errorf("a moved generation must not requeue: its update is already queued by the generation predicate, and a requeue is an AddRateLimited that Forget does not cancel (measured: 3 reconciles instead of 2), got %+v", res)
	}
}
