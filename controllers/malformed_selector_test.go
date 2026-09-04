//go:build !integration
// +build !integration

package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	userv1 "github.com/openshift/api/user/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// The API server accepts a selector with an unknown operator (the CRD has no enum), so one such CR
// is a realistic state. It must not stop the watch map funcs from enqueuing every OTHER CR.
var malformedSelector = metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "k", Operator: "Foo"}}}

func TestFindApplicableNameSpaceConfigs_SkipsMalformedSelectors(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := redhatcopv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "bad"}, Spec: redhatcopv1alpha1.NamespaceConfigSpec{LabelSelector: malformedSelector}},
		&redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "bad-annotations"}, Spec: redhatcopv1alpha1.NamespaceConfigSpec{AnnotationSelector: malformedSelector}},
		&redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "good"}, Spec: redhatcopv1alpha1.NamespaceConfigSpec{LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"team": "a"}}}},
	).Build()
	r := &NamespaceConfigReconciler{EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(c, scheme, nil, c, record.NewFakeRecorder(1), true, true), Log: logr.Discard()}

	got, err := r.findApplicableNameSpaceConfigs(context.Background(), corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"team": "a"}}})
	if err != nil {
		t.Fatalf("a malformed CR must not fail the lookup for the others: %v", err)
	}
	if len(got) != 1 || got[0].Name != "good" {
		t.Errorf("expected exactly the good CR, got %d", len(got))
	}
}

func TestFindApplicableGroupConfigsFromGroup_SkipsMalformedSelectors(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{redhatcopv1alpha1.AddToScheme, userv1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&redhatcopv1alpha1.GroupConfig{ObjectMeta: metav1.ObjectMeta{Name: "bad"}, Spec: redhatcopv1alpha1.GroupConfigSpec{LabelSelector: malformedSelector}},
		&redhatcopv1alpha1.GroupConfig{ObjectMeta: metav1.ObjectMeta{Name: "good"}, Spec: redhatcopv1alpha1.GroupConfigSpec{LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"synced": "yes"}}}},
	).Build()
	r := &GroupConfigReconciler{EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(c, scheme, nil, c, record.NewFakeRecorder(1), true, true), Log: logr.Discard()}

	got, err := r.findApplicableGroupConfigsFromGroup(context.Background(), userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "g", Labels: map[string]string{"synced": "yes"}}})
	if err != nil {
		t.Fatalf("a malformed CR must not fail the lookup for the others: %v", err)
	}
	if len(got) != 1 || got[0].Name != "good" {
		t.Errorf("expected exactly the good CR, got %d", len(got))
	}
}
