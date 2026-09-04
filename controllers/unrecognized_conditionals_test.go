//go:build !integration
// +build !integration

package controllers

import (
	"testing"

	userv1 "github.com/openshift/api/user/v1"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A guard outside the statically understood grammar used to make the template "apply to every
// object, relying on the renderer" — and the renderer cannot represent an empty render, so every
// rejected object produced "Object 'Kind' is missing in 'null'" at error level and dropped the rest
// of that object's batch. Such guards are now decided by rendering the template, so the answer is
// exactly what the renderer would produce, without the error.
func TestUnrecognizedConditionals(t *testing.T) {
	reconciler := &GroupConfigReconciler{}

	t.Run("a guard on a field outside the grammar is decided by rendering", func(t *testing.T) {
		// .Users is a Group field the static evaluator does not know.
		template := apis.LockedResourceTemplate{ObjectTemplate: `{{- if .Users }}
kind: ClusterRoleBinding
metadata:
  name: {{ .Name }}-members
{{- end }}
`}
		withMembers := userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "admins"}, Users: []string{"alice"}}
		empty := userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "nobody"}}

		if !reconciler.isTemplateApplicableToGroup(template, withMembers) {
			t.Errorf("expected the template to apply to a group with members")
		}
		if reconciler.isTemplateApplicableToGroup(template, empty) {
			t.Errorf("expected the template to be skipped for a group without members: it renders nothing")
		}
	})

	t.Run("a template that does not parse is left to the renderer", func(t *testing.T) {
		template := apis.LockedResourceTemplate{ObjectTemplate: `{{- if bogus .Name }}
kind: ConfigMap
{{- end }}
`}
		group := userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "admins"}}
		if !reconciler.isTemplateApplicableToGroup(template, group) {
			t.Errorf("a template with a parse error must reach the renderer so the error is reported there")
		}
	})
}
