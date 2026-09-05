//go:build !integration
// +build !integration

package controllers

import (
	"testing"

	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsInitialized(t *testing.T) {
	r := &NamespaceConfigReconciler{controllerName: "redhatcop.redhat.io/namespaceconfig-controller"}
	fresh := func() *redhatcopv1alpha1.NamespaceConfig {
		return &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc"}, Spec: redhatcopv1alpha1.NamespaceConfigSpec{
			Templates: []apis.LockedResourceTemplate{{ObjectTemplate: "kind: Role", ExcludedPaths: []string{".spec.replicas"}}},
		}}
	}

	nc := fresh()
	if r.IsInitialized(nc) {
		t.Fatal("a fresh CR needs its excludedPaths union and finalizer, so it is not initialized")
	}
	// the CR's own path plus the two defaults; `.metadata` is no longer added (issue #16)
	if len(nc.Spec.Templates[0].ExcludedPaths) != 2 || len(nc.Finalizers) != 1 {
		t.Errorf("expected the CR's path unioned with the default excludedPaths, and the finalizer, got %v %v", nc.Spec.Templates[0].ExcludedPaths, nc.Finalizers)
	}
	if !r.IsInitialized(nc) {
		t.Error("the second pass must find nothing to change")
	}

	deleting := fresh()
	now := metav1.Now()
	deleting.DeletionTimestamp = &now
	deleting.Finalizers = []string{r.controllerName}
	if !r.IsInitialized(deleting) {
		t.Error("a CR being deleted must not be rewritten")
	}
	if len(deleting.Spec.Templates[0].ExcludedPaths) != 1 {
		t.Errorf("excludedPaths must be left alone on a deleting CR, got %v", deleting.Spec.Templates[0].ExcludedPaths)
	}
}
