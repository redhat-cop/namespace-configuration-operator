//go:build !integration
// +build !integration

package controllers

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	userv1 "github.com/openshift/api/user/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Deleting a CR must remove what it owns even when the in-memory enforcer knows nothing about it,
// which is the state after an operator restart or after a failed Terminate. The reconciler here has
// never started an enforcer for the CR, exactly that state.
func TestManageCleanUpLogic_DeletesOwnedObjectsWithoutAStartedEnforcer(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, redhatcopv1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cm := func(ns, name string) *corev1.ConfigMap {
		return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}
	}
	instance := &redhatcopv1alpha1.NamespaceConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "nc"},
		Spec: redhatcopv1alpha1.NamespaceConfigSpec{
			LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"selected": "yes"}},
			Templates:     []apis.LockedResourceTemplate{{ObjectTemplate: requiredLabelTemplate}},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		instance,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"selected": "yes", "team": "a"}}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-b", Labels: map[string]string{"selected": "yes"}}}, // no team label: its template cannot render
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "unselected", Labels: map[string]string{"team": "z"}}},
		cm("team-a", "a"),        // owned: rendered from the template for team-a
		cm("team-a", "not-ours"), // decoy in an owned namespace
		cm("unselected", "z"),    // would match the template, but the namespace is not selected
	).Build()
	recorder := record.NewFakeRecorder(10)
	r := &NamespaceConfigReconciler{
		EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(c, scheme, nil, c, recorder, true, true),
		Log:                 logr.Discard(),
	}

	if err := r.manageCleanUpLogic(context.Background(), instance); err != nil {
		t.Fatalf("cleanup must not fail because one namespace cannot render: %v", err)
	}
	exists := func(ns, name string) bool {
		err := c.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: name}, &corev1.ConfigMap{})
		if err != nil && !apierrors.IsNotFound(err) {
			t.Fatal(err)
		}
		return err == nil
	}
	if exists("team-a", "a") {
		t.Errorf("the owned object in team-a must be deleted")
	}
	if !exists("team-a", "not-ours") || !exists("unselected", "z") {
		t.Errorf("objects the CR does not own must survive")
	}
	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, "CleanupIncomplete") || !strings.Contains(ev, "team-b") {
			t.Errorf("expected a CleanupIncomplete warning naming team-b, got %q", ev)
		}
	default:
		t.Errorf("a namespace whose template cannot render must be reported as a Warning event")
	}
}

// A listing failure must keep the finalizer: cleanup cannot claim success without knowing the set.
func TestManageCleanUpLogic_ListFailureKeepsTheFinalizer(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := redhatcopv1alpha1.AddToScheme(scheme); err != nil { // corev1 deliberately absent: listing namespaces fails
		t.Fatal(err)
	}
	instance := &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc"}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	r := &NamespaceConfigReconciler{
		EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(c, scheme, nil, c, record.NewFakeRecorder(1), true, true),
		Log:                 logr.Discard(),
	}
	if err := r.manageCleanUpLogic(context.Background(), instance); err == nil {
		t.Fatal("expected an error when the selected namespaces cannot be listed")
	}
}

// A CR whose selector does not compile never selected (or created) anything under that spec. Its
// deletion must complete, with the gap reported, rather than hang on a finalizer that can never clear.
func TestManageCleanUpLogic_MalformedSelectorFinalizesWithAWarning(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := redhatcopv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	instance := &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc"}, Spec: redhatcopv1alpha1.NamespaceConfigSpec{LabelSelector: malformedSelector}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	recorder := record.NewFakeRecorder(1)
	r := &NamespaceConfigReconciler{
		EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(c, scheme, nil, c, recorder, true, true),
		Log:                 logr.Discard(),
	}
	if err := r.manageCleanUpLogic(context.Background(), instance); err != nil {
		t.Fatalf("a malformed selector must not block deletion: %v", err)
	}
	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, "CleanupIncomplete") || !strings.Contains(ev, "labelSelector does not compile") {
			t.Errorf("expected a CleanupIncomplete warning about the selector, got %q", ev)
		}
	default:
		t.Error("expected a CleanupIncomplete warning event")
	}
}

// A UserConfig's identityExtraFieldSelector is a selector like the other two; a malformed one must
// be reported as the reason cleanup could not recompute, not silently produce an empty owned set.
func TestUserConfigCleanup_MalformedExtraSelectorIsReported(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, userv1.AddToScheme, redhatcopv1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	instance := &redhatcopv1alpha1.UserConfig{ObjectMeta: metav1.ObjectMeta{Name: "uc"}, Spec: redhatcopv1alpha1.UserConfigSpec{IdentityExtraFieldSelector: malformedSelector}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	recorder := record.NewFakeRecorder(1)
	r := &UserConfigReconciler{
		EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(c, scheme, nil, c, recorder, true, true),
		Log:                 logr.Discard(),
	}
	if err := r.manageCleanUpLogic(context.Background(), instance); err != nil {
		t.Fatalf("a malformed selector must not block deletion: %v", err)
	}
	select {
	case ev := <-recorder.Events:
		if !strings.Contains(ev, "identityExtraFieldSelector does not compile") {
			t.Errorf("the warning must name the identityExtraFieldSelector, got %q", ev)
		}
	default:
		t.Error("expected a CleanupIncomplete warning event")
	}
}
