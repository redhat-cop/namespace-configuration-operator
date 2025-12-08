//go:build !integration
// +build !integration

package controllers

import (
	"reflect"
	"testing"

	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespaceExtractHasSuffixPatterns(t *testing.T) {
	reconciler := &NamespaceConfigReconciler{}

	tests := []struct {
		name            string
		templateContent string
		expected        []string
	}{
		{
			name: "single hasSuffix pattern",
			templateContent: `{{- if hasSuffix "-prod" .Name }}
kind: RoleBinding
{{- end }}`,
			expected: []string{"-prod"},
		},
		{
			name: "multiple hasSuffix patterns",
			templateContent: `{{- if hasSuffix "-prod" .Name }}
prod stuff
{{- else if hasSuffix "-dev" .Name }}
dev stuff
{{- end }}`,
			expected: []string{"-prod", "-dev"},
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

func TestNamespaceExtractContainsPatterns(t *testing.T) {
	reconciler := &NamespaceConfigReconciler{}

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
{{- else if contains "logging" .Name }}
logging role
{{- end }}`,
			expected: []string{"monitoring", "logging"},
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

func TestIsTemplateApplicableToNamespace(t *testing.T) {
	reconciler := &NamespaceConfigReconciler{}

	tests := []struct {
		name      string
		template  apis.LockedResourceTemplate
		namespace corev1.Namespace
		expected  bool
	}{
		{
			name: "namespace matches hasSuffix pattern",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-prod" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-app-prod",
				},
			},
			expected: true,
		},
		{
			name: "namespace does not match hasSuffix pattern",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-prod" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-app-dev",
				},
			},
			expected: false,
		},
		{
			name: "namespace matches contains pattern",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if contains "monitoring" .Name }}
kind: Role
{{- end }}`,
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user-workload-monitoring",
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
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "any-namespace-name",
				},
			},
			expected: true,
		},
		{
			name: "namespace matches multiple patterns (OR logic - any match)",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-prod" .Name }}
kind: RoleBinding
{{- else if contains "monitoring" .Name }}
kind: Role
{{- end }}`,
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user-workload-monitoring",
				},
			},
			expected: true, // Should match because contains "monitoring"
		},
		{
			name: "namespace matches one of multiple patterns",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-prod" .Name }}
prod
{{- else if hasSuffix "-dev" .Name }}
dev
{{- end }}`,
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-app-prod",
				},
			},
			expected: true, // Should match hasSuffix "-prod"
		},
		{
			name: "AND logic - namespace matches all patterns",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if and (hasSuffix "-prod" .Name) (contains "my-app" .Name) }}
kind: RoleBinding
{{- end }}`,
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-app-prod",
				},
			},
			expected: true, // Should match because BOTH conditions are true
		},
		{
			name: "AND logic - namespace matches only one pattern (should fail)",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if and (hasSuffix "-prod" .Name) (contains "monitoring" .Name) }}
kind: RoleBinding
{{- end }}`,
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-app-prod",
				},
			},
			expected: false, // Should NOT match because only hasSuffix matches, but contains "monitoring" doesn't
		},
		{
			name: "AND logic - namespace matches none of the patterns (should fail)",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if and (hasSuffix "-dev" .Name) (contains "monitoring" .Name) }}
kind: RoleBinding
{{- end }}`,
			},
			namespace: corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-app-prod",
				},
			},
			expected: false, // Should NOT match because neither pattern matches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconciler.isTemplateApplicableToNamespace(tt.template, tt.namespace)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNamespaceFilterApplicableTemplates(t *testing.T) {
	reconciler := &NamespaceConfigReconciler{}

	t.Run("filters templates based on namespace matching", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{
				ObjectTemplate: `{{- if hasSuffix "-prod" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			{
				ObjectTemplate: `{{- if hasSuffix "-dev" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			{
				ObjectTemplate: `kind: Role
metadata:
  name: basic-role`,
			},
		}

		namespace := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-app-prod",
			},
		}

		filteredTemplates := reconciler.filterApplicableTemplates(templates, namespace)

		// Should return 2 templates: the matching hasSuffix one and the unconditional one
		if len(filteredTemplates) != 2 {
			t.Errorf("Expected 2 templates, got %d", len(filteredTemplates))
		}
	})

	t.Run("returns empty slice when no templates match", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{
				ObjectTemplate: `{{- if hasSuffix "-prod" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			{
				ObjectTemplate: `{{- if contains "monitoring" .Name }}
kind: Role
{{- end }}`,
			},
		}

		namespace := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-app-dev",
			},
		}

		filteredTemplates := reconciler.filterApplicableTemplates(templates, namespace)

		if len(filteredTemplates) != 0 {
			t.Errorf("Expected 0 templates, got %d", len(filteredTemplates))
		}
	})
}
