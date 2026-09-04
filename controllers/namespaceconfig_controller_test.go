//go:build !integration
// +build !integration

package controllers

import (
	"testing"

	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The decision logic itself is tested exhaustively in controllers/common (templatefilter_test.go),
// including a property test that checks every guard shape against the real renderer. These tests
// pin the reconciler's wiring: the namespace it passes is the one the guard reads, name, labels and
// annotations included.

func NamespaceSubject(name string, labels map[string]string, annotations map[string]string) corev1.Namespace {
	return corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels, Annotations: annotations}}
}

func TestNamespaceIsTemplateApplicable(t *testing.T) {
	reconciler := &NamespaceConfigReconciler{}

	tests := []struct {
		name     string
		template string
		subject  corev1.Namespace
		expected bool
	}{
		{
			name:     "matches hasSuffix pattern",
			template: "{{- if hasSuffix \"-prod\" .Name }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("my-app-prod", nil, nil),
			expected: true,
		},
		{
			name:     "does not match hasSuffix pattern",
			template: "{{- if hasSuffix \"-prod\" .Name }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("my-app-dev", nil, nil),
			expected: false,
		},
		{
			name:     "matches contains pattern",
			template: "{{- if contains \"monitoring\" .Name }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("team-monitoring-x", nil, nil),
			expected: true,
		},
		{
			name:     "template with no guard applies to all",
			template: "kind: RoleBinding\nmetadata:\n  name: basic",
			subject:  NamespaceSubject("any-name", nil, nil),
			expected: true,
		},
		{
			name:     "else-if chain: any branch qualifies",
			template: "{{- if hasSuffix \"-prod\" .Name }}\nkind: RoleBinding\n{{- else if contains \"monitoring\" .Name }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("team-monitoring-x", nil, nil),
			expected: true,
		},
		{
			name:     "and: all conditions hold",
			template: "{{- if and (hasSuffix \"-prod\" .Name) (contains \"my-app\" .Name) }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("my-app-prod", nil, nil),
			expected: true,
		},
		{
			name:     "and: only one condition holds",
			template: "{{- if and (hasSuffix \"-prod\" .Name) (contains \"monitoring\" .Name) }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("my-app-prod", nil, nil),
			expected: false,
		},
		{
			name:     "and: no condition holds",
			template: "{{- if and (hasSuffix \"-dev\" .Name) (contains \"monitoring\" .Name) }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("my-app-prod", nil, nil),
			expected: false,
		},
		{
			name:     "hasPrefix on a label value: in the family",
			template: "{{- if hasPrefix \"app-bdp-rbac-spark-\" (index .Labels \"example.com/oud-group\") }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("bdp-spark-alpha", map[string]string{"example.com/oud-group": "app-bdp-rbac-spark-alpha"}, nil),
			expected: true,
		},
		{
			name:     "hasPrefix on a label value: another family",
			template: "{{- if hasPrefix \"app-bdp-rbac-spark-\" (index .Labels \"example.com/oud-group\") }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("bdp-trino-apps", map[string]string{"example.com/oud-group": "app-bdp-rbac-trino-apps"}, nil),
			expected: false,
		},
		{
			name:     "hasPrefix on a label value: label absent",
			template: "{{- if hasPrefix \"app-bdp-rbac-spark-\" (index .Labels \"example.com/oud-group\") }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("plain", nil, nil),
			expected: false,
		},
		{
			name:     "truthiness of a label value: empty value does not qualify",
			template: "{{- if (index .Labels \"example.com/oud-group\") }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("bdp-empty", map[string]string{"example.com/oud-group": ""}, nil),
			expected: false,
		},
		{
			name:     "ne on an annotation value",
			template: "{{- if ne (index .Annotations \"allow-pvc\") \"true\" }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("quota", nil, map[string]string{"allow-pvc": "true"}),
			expected: false,
		},
		{
			name:     "ne on an annotation value: annotation absent",
			template: "{{- if ne (index .Annotations \"allow-pvc\") \"true\" }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("quota", nil, nil),
			expected: true,
		},
		{
			name:     "eq on the name, the documented unrecognized example",
			template: "{{- if eq .Name \"admin\" }}\nkind: RoleBinding\n{{- end }}",
			subject:  NamespaceSubject("dev", nil, nil),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconciler.isTemplateApplicableToNamespace(apis.LockedResourceTemplate{ObjectTemplate: tt.template}, tt.subject)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNamespaceFilterApplicableTemplates(t *testing.T) {
	reconciler := &NamespaceConfigReconciler{}

	t.Run("keeps the matching guarded template and the unconditional one", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{ObjectTemplate: "{{- if hasSuffix \"-prod\" .Name }}\nkind: RoleBinding\n{{- end }}"},
			{ObjectTemplate: "{{- if hasSuffix \"-dev\" .Name }}\nkind: RoleBinding\n{{- end }}"},
			{ObjectTemplate: "kind: RoleBinding\nmetadata:\n  name: basic"},
		}
		filtered := reconciler.filterApplicableTemplates(templates, NamespaceSubject("my-app-prod", nil, nil))
		if len(filtered) != 2 {
			t.Errorf("Expected 2 templates, got %d", len(filtered))
		}
	})

	t.Run("returns an empty slice when no template matches", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{ObjectTemplate: "{{- if hasSuffix \"-prod\" .Name }}\nkind: RoleBinding\n{{- end }}"},
			{ObjectTemplate: "{{- if contains \"monitoring\" .Name }}\nkind: RoleBinding\n{{- end }}"},
		}
		filtered := reconciler.filterApplicableTemplates(templates, NamespaceSubject("my-app-dev", nil, nil))
		if len(filtered) != 0 {
			t.Errorf("Expected 0 templates, got %d", len(filtered))
		}
	})

	t.Run("a label-guarded template is skipped for a namespace outside its family", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{ObjectTemplate: "{{- if hasPrefix \"app-bdp-rbac-spark-\" (index .Labels \"example.com/oud-group\") }}\nkind: RoleBinding\n{{- end }}"},
		}
		filtered := reconciler.filterApplicableTemplates(templates, NamespaceSubject("bdp-trino-apps", map[string]string{"example.com/oud-group": "app-bdp-rbac-trino-apps"}, nil))
		if len(filtered) != 0 {
			t.Errorf("Expected 0 templates, got %d", len(filtered))
		}
	})
}
