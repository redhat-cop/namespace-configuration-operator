//go:build !integration
// +build !integration

package controllers

import (
	"context"
	"strings"
	"testing"

	userv1 "github.com/openshift/api/user/v1"
	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A render failure for ONE selected object must surface as an error from getResourceList in every
// controller, so the reconcile ends in ManageError instead of handing the enforcer a batch that is
// missing that object's resources (which it would delete). The templates use `required`, the way a
// policy guards a label it cannot do without.
const requiredLabelTemplate = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ required \"team label is required\" (index .Labels \"team\") }}\n  namespace: {{ .Name }}\n"

func TestGetResourceList_RenderFailureIsAnError(t *testing.T) {
	templates := []apis.LockedResourceTemplate{{ObjectTemplate: requiredLabelTemplate}}
	labelled := metav1.ObjectMeta{Name: "with-label", Labels: map[string]string{"team": "a"}}
	unlabelled := metav1.ObjectMeta{Name: "without-label"}

	t.Run("namespaceconfig", func(t *testing.T) {
		r := &NamespaceConfigReconciler{}
		instance := &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc"}, Spec: redhatcopv1alpha1.NamespaceConfigSpec{Templates: templates}}
		lrs, err := r.getResourceList(context.Background(), instance, []corev1.Namespace{{ObjectMeta: labelled}})
		if err != nil || len(lrs) != 1 {
			t.Fatalf("happy path: want 1 resource, got %d err=%v", len(lrs), err)
		}
		lrs, err = r.getResourceList(context.Background(), instance, []corev1.Namespace{{ObjectMeta: labelled}, {ObjectMeta: unlabelled}})
		if err == nil || lrs != nil {
			t.Fatalf("want an error and no partial batch, got %d resources err=%v", len(lrs), err)
		}
		for _, want := range []string{"namespaceconfig nc", "without-label", "team label is required"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q should name %q", err.Error(), want)
			}
		}
	})
	t.Run("groupconfig", func(t *testing.T) {
		r := &GroupConfigReconciler{}
		instance := &redhatcopv1alpha1.GroupConfig{ObjectMeta: metav1.ObjectMeta{Name: "gc"}, Spec: redhatcopv1alpha1.GroupConfigSpec{Templates: templates}}
		if _, err := r.getResourceList(context.Background(), instance, []userv1.Group{{ObjectMeta: labelled}, {ObjectMeta: unlabelled}}); err == nil || !strings.Contains(err.Error(), "groupconfig gc") {
			t.Fatalf("want an error naming the GroupConfig, got %v", err)
		}
	})
	t.Run("userconfig", func(t *testing.T) {
		r := &UserConfigReconciler{}
		instance := &redhatcopv1alpha1.UserConfig{ObjectMeta: metav1.ObjectMeta{Name: "uc"}, Spec: redhatcopv1alpha1.UserConfigSpec{Templates: templates}}
		if _, err := r.getResourceList(context.Background(), instance, []userv1.User{{ObjectMeta: labelled}, {ObjectMeta: unlabelled}}); err == nil || !strings.Contains(err.Error(), "userconfig uc") {
			t.Fatalf("want an error naming the UserConfig, got %v", err)
		}
	})
}
