//go:build !integration
// +build !integration

package controllers

import (
	"testing"

	userv1 "github.com/openshift/api/user/v1"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The decision logic itself is tested exhaustively in controllers/common (templatefilter_test.go),
// including a property test that checks every guard shape against the real renderer. These tests
// pin the reconciler's wiring: the group it passes is the one the guard reads, name, labels and
// annotations included.

func GroupSubject(name string, labels map[string]string, annotations map[string]string) userv1.Group {
	return userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels, Annotations: annotations}}
}

func TestGroupIsTemplateApplicable(t *testing.T) {
	reconciler := &GroupConfigReconciler{}

	tests := []struct {
		name     string
		template string
		subject  userv1.Group
		expected bool
	}{
		{
			name:     "matches hasSuffix pattern",
			template: "{{- if hasSuffix \"-cluster-admin\" .Name }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("my-app-cluster-admin", nil, nil),
			expected: true,
		},
		{
			name:     "does not match hasSuffix pattern",
			template: "{{- if hasSuffix \"-cluster-admin\" .Name }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("my-app-cluster-audit", nil, nil),
			expected: false,
		},
		{
			name:     "matches contains pattern",
			template: "{{- if contains \"developer\" .Name }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("team-developer-x", nil, nil),
			expected: true,
		},
		{
			name:     "template with no guard applies to all",
			template: "kind: ClusterRoleBinding\nmetadata:\n  name: basic",
			subject:  GroupSubject("any-name", nil, nil),
			expected: true,
		},
		{
			name:     "else-if chain: any branch qualifies",
			template: "{{- if hasSuffix \"-cluster-admin\" .Name }}\nkind: ClusterRoleBinding\n{{- else if contains \"developer\" .Name }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("team-developer-x", nil, nil),
			expected: true,
		},
		{
			name:     "and: all conditions hold",
			template: "{{- if and (hasSuffix \"-cluster-admin\" .Name) (contains \"my-app\" .Name) }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("my-app-cluster-admin", nil, nil),
			expected: true,
		},
		{
			name:     "and: only one condition holds",
			template: "{{- if and (hasSuffix \"-cluster-admin\" .Name) (contains \"developer\" .Name) }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("my-app-cluster-admin", nil, nil),
			expected: false,
		},
		{
			name:     "and: no condition holds",
			template: "{{- if and (hasSuffix \"-cluster-audit\" .Name) (contains \"developer\" .Name) }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("my-app-cluster-admin", nil, nil),
			expected: false,
		},
		{
			name:     "hasPrefix on a label value: in the family",
			template: "{{- if hasPrefix \"app-bdp-rbac-spark-\" (index .Labels \"example.com/oud-group\") }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("bdp-spark-alpha", map[string]string{"example.com/oud-group": "app-bdp-rbac-spark-alpha"}, nil),
			expected: true,
		},
		{
			name:     "hasPrefix on a label value: another family",
			template: "{{- if hasPrefix \"app-bdp-rbac-spark-\" (index .Labels \"example.com/oud-group\") }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("bdp-trino-apps", map[string]string{"example.com/oud-group": "app-bdp-rbac-trino-apps"}, nil),
			expected: false,
		},
		{
			name:     "hasPrefix on a label value: label absent",
			template: "{{- if hasPrefix \"app-bdp-rbac-spark-\" (index .Labels \"example.com/oud-group\") }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("plain", nil, nil),
			expected: false,
		},
		{
			name:     "truthiness of a label value: empty value does not qualify",
			template: "{{- if (index .Labels \"example.com/oud-group\") }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("bdp-empty", map[string]string{"example.com/oud-group": ""}, nil),
			expected: false,
		},
		{
			name:     "ne on an annotation value",
			template: "{{- if ne (index .Annotations \"allow-pvc\") \"true\" }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("quota", nil, map[string]string{"allow-pvc": "true"}),
			expected: false,
		},
		{
			name:     "ne on an annotation value: annotation absent",
			template: "{{- if ne (index .Annotations \"allow-pvc\") \"true\" }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("quota", nil, nil),
			expected: true,
		},
		{
			name:     "eq on the name, the documented unrecognized example",
			template: "{{- if eq .Name \"admin\" }}\nkind: ClusterRoleBinding\n{{- end }}",
			subject:  GroupSubject("dev", nil, nil),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconciler.isTemplateApplicableToGroup(apis.LockedResourceTemplate{ObjectTemplate: tt.template}, tt.subject)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGroupFilterApplicableTemplates(t *testing.T) {
	reconciler := &GroupConfigReconciler{}

	t.Run("keeps the matching guarded template and the unconditional one", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{ObjectTemplate: "{{- if hasSuffix \"-cluster-admin\" .Name }}\nkind: ClusterRoleBinding\n{{- end }}"},
			{ObjectTemplate: "{{- if hasSuffix \"-cluster-audit\" .Name }}\nkind: ClusterRoleBinding\n{{- end }}"},
			{ObjectTemplate: "kind: ClusterRoleBinding\nmetadata:\n  name: basic"},
		}
		filtered := reconciler.filterApplicableTemplates(templates, GroupSubject("my-app-cluster-admin", nil, nil))
		if len(filtered) != 2 {
			t.Errorf("Expected 2 templates, got %d", len(filtered))
		}
	})

	t.Run("returns an empty slice when no template matches", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{ObjectTemplate: "{{- if hasSuffix \"-cluster-admin\" .Name }}\nkind: ClusterRoleBinding\n{{- end }}"},
			{ObjectTemplate: "{{- if contains \"developer\" .Name }}\nkind: ClusterRoleBinding\n{{- end }}"},
		}
		filtered := reconciler.filterApplicableTemplates(templates, GroupSubject("my-app-cluster-audit", nil, nil))
		if len(filtered) != 0 {
			t.Errorf("Expected 0 templates, got %d", len(filtered))
		}
	})

	t.Run("a label-guarded template is skipped for a group outside its family", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{ObjectTemplate: "{{- if hasPrefix \"app-bdp-rbac-spark-\" (index .Labels \"example.com/oud-group\") }}\nkind: ClusterRoleBinding\n{{- end }}"},
		}
		filtered := reconciler.filterApplicableTemplates(templates, GroupSubject("bdp-trino-apps", map[string]string{"example.com/oud-group": "app-bdp-rbac-trino-apps"}, nil))
		if len(filtered) != 0 {
			t.Errorf("Expected 0 templates, got %d", len(filtered))
		}
	})
}
