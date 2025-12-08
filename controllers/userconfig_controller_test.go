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

func TestUserExtractHasSuffixPatterns(t *testing.T) {
	reconciler := &UserConfigReconciler{}

	tests := []struct {
		name            string
		templateContent string
		expected        []string
	}{
		{
			name: "single hasSuffix pattern",
			templateContent: `{{- if hasSuffix "-admin" .Name }}
kind: RoleBinding
{{- end }}`,
			expected: []string{"-admin"},
		},
		{
			name: "multiple hasSuffix patterns",
			templateContent: `{{- if hasSuffix "-admin" .Name }}
admin stuff
{{- else if hasSuffix "-view" .Name }}
view stuff
{{- end }}`,
			expected: []string{"-admin", "-view"},
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

func TestUserExtractContainsPatterns(t *testing.T) {
	reconciler := &UserConfigReconciler{}

	tests := []struct {
		name            string
		templateContent string
		expected        []string
	}{
		{
			name: "single contains pattern",
			templateContent: `{{- if contains "jdoe" .Name }}
kind: Role
{{- end }}`,
			expected: []string{"jdoe"},
		},
		{
			name: "multiple contains patterns",
			templateContent: `{{- if contains "jdoe" .Name }}
jdoe role
{{- else if contains "smith" .Name }}
smith role
{{- end }}`,
			expected: []string{"jdoe", "smith"},
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

func TestIsTemplateApplicableToUser(t *testing.T) {
	reconciler := &UserConfigReconciler{}

	tests := []struct {
		name     string
		template apis.LockedResourceTemplate
		user     userv1.User
		expected bool
	}{
		{
			name: "user matches hasSuffix pattern",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-admin" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			user: userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user-admin",
				},
			},
			expected: true,
		},
		{
			name: "user does not match hasSuffix pattern",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-admin" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			user: userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user-view",
				},
			},
			expected: false,
		},
		{
			name: "user matches contains pattern",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if contains "jdoe" .Name }}
kind: Role
{{- end }}`,
			},
			user: userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "jdoe-user",
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
			user: userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "any-user-name",
				},
			},
			expected: true,
		},
		{
			name: "user matches multiple patterns (OR logic - any match)",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if hasSuffix "-admin" .Name }}
kind: RoleBinding
{{- else if contains "jdoe" .Name }}
kind: Role
{{- end }}`,
			},
			user: userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "jdoe-user",
				},
			},
			expected: true, // Should match because contains "jdoe"
		},
		{
			name: "AND logic - user matches all patterns",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if and (hasSuffix "-admin" .Name) (contains "super" .Name) }}
kind: RoleBinding
{{- end }}`,
			},
			user: userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "super-user-admin",
				},
			},
			expected: true, // Should match because BOTH conditions are true
		},
		{
			name: "AND logic - user matches only one pattern (should fail)",
			template: apis.LockedResourceTemplate{
				ObjectTemplate: `{{- if and (hasSuffix "-admin" .Name) (contains "super" .Name) }}
kind: RoleBinding
{{- end }}`,
			},
			user: userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "regular-user-admin",
				},
			},
			expected: false, // Should NOT match because only hasSuffix matches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reconciler.isTemplateApplicableToUser(tt.template, tt.user)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestUserFilterApplicableTemplates(t *testing.T) {
	reconciler := &UserConfigReconciler{}

	t.Run("filters templates based on user matching", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{
				ObjectTemplate: `{{- if hasSuffix "-admin" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			{
				ObjectTemplate: `{{- if hasSuffix "-view" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			{
				ObjectTemplate: `kind: Role
metadata:
  name: basic-role`,
			},
		}

		user := userv1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: "user-admin",
			},
		}

		filteredTemplates := reconciler.filterApplicableTemplates(templates, user)

		// Should return 2 templates: the matching hasSuffix one and the unconditional one
		if len(filteredTemplates) != 2 {
			t.Errorf("Expected 2 templates, got %d", len(filteredTemplates))
		}
	})

	t.Run("returns empty slice when no templates match", func(t *testing.T) {
		templates := []apis.LockedResourceTemplate{
			{
				ObjectTemplate: `{{- if hasSuffix "-admin" .Name }}
kind: RoleBinding
{{- end }}`,
			},
			{
				ObjectTemplate: `{{- if contains "jdoe" .Name }}
kind: Role
{{- end }}`,
			},
		}

		user := userv1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: "other-user",
			},
		}

		filteredTemplates := reconciler.filterApplicableTemplates(templates, user)

		if len(filteredTemplates) != 0 {
			t.Errorf("Expected 0 templates, got %d", len(filteredTemplates))
		}
	})
}
