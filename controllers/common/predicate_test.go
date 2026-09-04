//go:build !integration
// +build !integration

package common

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestSelectedObjectChangedPredicate(t *testing.T) {
	base := func() *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "n", ResourceVersion: "1", Labels: map[string]string{"team": "a"}, Annotations: map[string]string{"note": "x"}}}
	}
	cases := []struct {
		name string
		mut  func(n *corev1.Namespace)
		want bool
	}{
		{"resourceVersion only", func(n *corev1.Namespace) { n.ResourceVersion = "2" }, false},
		{"status only", func(n *corev1.Namespace) { n.Status.Phase = corev1.NamespaceTerminating }, false},
		{"spec only", func(n *corev1.Namespace) { n.Spec.Finalizers = nil }, false},
		{"label changed", func(n *corev1.Namespace) { n.Labels["team"] = "b" }, true},
		{"label added", func(n *corev1.Namespace) { n.Labels["x"] = "y" }, true},
		{"label removed", func(n *corev1.Namespace) { delete(n.Labels, "team") }, true},
		{"annotation changed", func(n *corev1.Namespace) { n.Annotations["note"] = "y" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old, updated := base(), base()
			tc.mut(updated)
			if got := SelectedObjectChangedPredicate.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated}); got != tc.want {
				t.Errorf("Update = %v, want %v", got, tc.want)
			}
		})
	}
	if !SelectedObjectChangedPredicate.Create(event.CreateEvent{Object: base()}) || !SelectedObjectChangedPredicate.Delete(event.DeleteEvent{Object: base()}) {
		t.Error("Create and Delete must always pass")
	}
}
