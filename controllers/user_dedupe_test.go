//go:build !integration
// +build !integration

package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	userv1 "github.com/openshift/api/user/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// A user with two matching identities is one user: selected once, rendered once, enqueued once.
func TestUserSelection_OnePerUserAcrossIdentities(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{userv1.AddToScheme, redhatcopv1alpha1.AddToScheme, corev1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	user := &userv1.User{ObjectMeta: metav1.ObjectMeta{Name: "jdoe", UID: types.UID("u-1")}}
	identities := []userv1.Identity{
		{ObjectMeta: metav1.ObjectMeta{Name: "ldap:jdoe"}, ProviderName: "ldap", User: corev1.ObjectReference{Name: "jdoe", UID: "u-1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "github:jdoe"}, ProviderName: "github", User: corev1.ObjectReference{Name: "jdoe", UID: "u-1"}},
	}
	uc := &redhatcopv1alpha1.UserConfig{ObjectMeta: metav1.ObjectMeta{Name: "uc"}, Spec: redhatcopv1alpha1.UserConfigSpec{
		Templates: []apis.LockedResourceTemplate{{ObjectTemplate: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: {{ .Name }}-sandbox\n"}},
	}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(user, &identities[0], &identities[1], uc).Build()
	r := &UserConfigReconciler{EnforcingReconciler: lockedresourcecontroller.NewEnforcingReconciler(c, scheme, nil, c, record.NewFakeRecorder(1), true, true), Log: logr.Discard()}

	selected, err := r.getSelectedUsers(context.Background(), uc)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 {
		t.Fatalf("expected the user once, got %d", len(selected))
	}
	lrs, err := r.getResourceList(context.Background(), uc, selected)
	if err != nil || len(lrs) != 1 {
		t.Fatalf("expected one rendered resource, got %d err=%v", len(lrs), err)
	}
	applicable, err := r.findApplicableUserConfigsFromIdentities(context.Background(), user, identities)
	if err != nil || len(applicable) != 1 {
		t.Fatalf("expected the UserConfig once, got %d err=%v", len(applicable), err)
	}
	// Selecting on one provider still finds the user through the matching identity alone.
	uc.Spec.ProviderName = "github"
	if selected, err = r.getSelectedUsers(context.Background(), uc); err != nil || len(selected) != 1 {
		t.Fatalf("provider-scoped selection: expected 1, got %d err=%v", len(selected), err)
	}
	uc.Spec.ProviderName = "okta"
	if selected, err = r.getSelectedUsers(context.Background(), uc); err != nil || len(selected) != 0 {
		t.Fatalf("no identity for the provider: expected 0, got %d err=%v", len(selected), err)
	}
}
