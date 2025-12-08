//go:build !integration
// +build !integration

package controllers

import (
	"reflect"
	"testing"

	userv1 "github.com/openshift/api/user/v1"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractHasSuffixPatterns(t *testing.T) {
	reconciler := &GroupConfigReconciler{}

	tests := []struct {
		name            string
		templateContent string
		expected        []string
	}{
		{
			name: "single hasSuffix pattern",
			templateContent: `{{- if hasSuffix "-cluster-admin" .Name }}
kind: ClusterRoleBinding
{{- end }}`,
			expected: []string{"-cluster-admin"},
		},
		{
			name: "multiple hasSuffix patterns",
			templateContent: `{{- if hasSuffix "-cluster-admin" .Name }}
admin stuff
{{- else if hasSuffix "-cluster-audit" .Name }}
audit stuff
{{- end }}`,
			expected: []string{"-cluster-admin", "-cluster-audit"},
		},
		{
			name: "no hasSuffix patterns",
			templateContent: `kind: Role
metadata:
  name: basic-role`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := reconciler.extractHasSuffixPatterns(tt.templateContent)
			if !reflect.DeepEqual(patterns, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, patterns)
			}
		})
	}
}

func TestExtractContainsPatterns(t *testing.T) {
	reconciler := &GroupConfigReconciler{}

	tests := []struct {
		name            string
		templateContent string
		expected        []string
	}{
		{
			name: "single contains pattern",
			templateContent: `{{- if contains "monitoring" .Name }}
kind: Role
{{- end }}`,
			expected: []string{"monitoring"},
		},
		{
			name: "multiple contains patterns",
			templateContent: `{{- if contains "monitoring" .Name }}
monitoring role
{{- else if contains "developer" .Name }}
developer role
{{- end }}`,
			expected: []string{"monitoring", "developer"},
		},
		{
			name: "no contains patterns",
			templateContent: `kind: Role
metadata:
  name: basic-role`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := reconciler.extractContainsPatterns(tt.templateContent)
			if !reflect.DeepEqual(patterns, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, patterns)
			}
		})
	}
}

func TestIsTemplateApplicableToGroup(t *testing.T) {
	reconciler := &GroupConfigReconciler{}

	tests := []struct {
		name     string
		template apis.LockedResourceTemplate
		group    userv1.Group
		expected bool
	}{
		{
			name: "group matches hasSuffix pattern",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-cluster-admin" .Name }}
kind: ClusterRoleBinding
{{- end }}`,
			},
			group: userv1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-ocp-rbac-alpha-cluster-admin",
				},
			},
			expected: true,
		},
		{
			name: "group does not match hasSuffix pattern",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-cluster-admin" .Name }}
kind: ClusterRoleBinding
{{- end }}`,
			},
			group: userv1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-ocp-rbac-alpha-cluster-audit",
				},
			},
			expected: false,
		},
		{
			name: "group matches contains pattern",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if contains "monitoring" .Name }}
kind: Role
{{- end }}`,
			},
			group: userv1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user-workload-monitoring-admin",
				},
			},
			expected: true,
		},
		{
			name: "template with no patterns applies to all",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `kind: Role
metadata:
  name: basic-role`,
			},
			group: userv1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: "any-group-name",
				},
			},
			expected: true,
		},
		{
			name: "group matches multiple patterns (OR logic - any match)",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-cluster-admin" .Name }}
kind: ClusterRoleBinding
{{- else if contains "monitoring" .Name }}
kind: Role
{{- end }}`,
			},
			group: userv1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user-workload-monitoring-admin",
				},
			},
			expected: true, // Should match because contains "monitoring"
		},
		{
			name: "group matches one of multiple patterns",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-cluster-admin" .Name }}
admin
{{- else if hasSuffix "-cluster-audit" .Name }}
audit
{{- end }}`,
			},
			group: userv1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-ocp-rbac-alpha-cluster-admin",
				},
			},
			expected: true, // Should match hasSuffix "-cluster-admin"
		},
		{
			name: "AND logic - group matches all patterns",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if and (hasSuffix "-cluster-admin" .Name) (contains "app-ocp-rbac" .Name) }}
kind: ClusterRoleBinding
{{- end }}`,
			},
			group: userv1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-ocp-rbac-alpha-cluster-admin",
				},
			},
			expected: true, // Should match because BOTH conditions are true
		},
		{
			name: "AND logic - group matches only one pattern (should fail)",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if and (hasSuffix "-cluster-admin" .Name) (contains "monitoring" .Name) }}
kind: ClusterRoleBinding
{{- end }}`,
			},
			group: userv1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-ocp-rbac-alpha-cluster-admin",
				},
			},
			expected: false, // Should NOT match because only hasSuffix matches, but contains "monitoring" doesn't
		},
		{
			name: "AND logic - group matches none of the patterns (should fail)",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if and (hasSuffix "-cluster-audit" .Name) (contains "monitoring" .Name) }}
kind: ClusterRoleBinding
{{- end }}`,
			},
			group: userv1.Group{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-ocp-rbac-alpha-cluster-admin",
				},
			},
			expected: false, // Should NOT match because neither pattern matches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconciler.isTemplateApplicableToGroup(tt.template, tt.group)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilterApplicableTemplates(t *testing.T) {
	reconciler := &GroupConfigReconciler{}

	t.Run("filters templates based on group matching", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{
				ObjectTemplate: `{{- if hasSuffix "-cluster-admin" .Name }}
kind: ClusterRoleBinding
{{- end }}`,
			},
			{
				ObjectTemplate: `{{- if hasSuffix "-cluster-audit" .Name }}
kind: ClusterRoleBinding
{{- end }}`,
			},
			{
				ObjectTemplate: `kind: Role
metadata:
  name: basic-role`,
			},
		}

		group := userv1.Group{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-ocp-rbac-alpha-cluster-admin",
			},
		}

		filteredTemplates := reconciler.filterApplicableTemplates(templates, group)

		// Should return 2 templates: the matching hasSuffix one and the unconditional one
		if len(filteredTemplates) != 2 {
			t.Errorf("Expected 2 templates, got %d", len(filteredTemplates))
		}
	})

	t.Run("returns empty slice when no templates match", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{
				ObjectTemplate: `{{- if hasSuffix "-cluster-admin" .Name }}
kind: ClusterRoleBinding
{{- end }}`,
			},
			{
				ObjectTemplate: `{{- if contains "monitoring" .Name }}
kind: Role
{{- end }}`,
			},
		}

		group := userv1.Group{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-ocp-rbac-alpha-cluster-audit",
			},
		}

		filteredTemplates := reconciler.filterApplicableTemplates(templates, group)

		if len(filteredTemplates) != 0 {
			t.Errorf("Expected 0 templates, got %d", len(filteredTemplates))
		}
	})
}
