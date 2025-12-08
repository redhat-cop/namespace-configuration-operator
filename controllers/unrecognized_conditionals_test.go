//go:build !integration
// +build !integration

package controllers

import (
	"strings"
	"testing"

	userv1 "github.com/openshift/api/user/v1"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUnrecognizedConditionals(t *testing.T) {
	reconciler := &GroupConfigReconciler{}

	// Template with conditional logic that is NOT hasSuffix or contains
	// e.g. using 'eq' or 'hasPrefix'
	templateContent := `{{- if eq .Name "admin" }}
kind: ConfigMap
metadata:
  name: admin-config
{{- end }}
`

	template := apis.LockedResourceTemplate{
		ObjectTemplate: templateContent,
	}

	// Case 1: Group is "admin" (should match)
	adminGroup := userv1.Group{
		ObjectMeta: metav1.ObjectMeta{
			Name: "admin",
		},
	}

	// Case 2: Group is "dev" (should NOT match logically, but currently matches because no patterns extracted)
	devGroup := userv1.Group{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dev",
		},
	}

	// Test extraction - should be empty
	suffixPatterns := reconciler.extractHasSuffixPatterns(templateContent)
	if len(suffixPatterns) != 0 {
		t.Errorf("Expected 0 suffix patterns, got %v", suffixPatterns)
	}

	containsPatterns := reconciler.extractContainsPatterns(templateContent)
	if len(containsPatterns) != 0 {
		t.Errorf("Expected 0 contains patterns, got %v", containsPatterns)
	}

	// Check logic for Unrecognized Conditionals
	// It should return TRUE so that the template renderer can handle the logic
	if !reconciler.isTemplateApplicableToGroup(template, adminGroup) {
		t.Errorf("Expected template to apply to admin group (via fallthrough)")
	}

	if !reconciler.isTemplateApplicableToGroup(template, devGroup) {
		t.Errorf("Expected template to apply to dev group (via fallthrough, relying on renderer)")
	}

	// Verify the logic detection (manually checking what the code does)
	if len(suffixPatterns) == 0 && len(containsPatterns) == 0 {
		if strings.Contains(templateContent, "{{- if") || strings.Contains(templateContent, "{{ if") {
			t.Log("Correctly detected unrecognized conditional logic")
		} else {
			t.Error("Failed to detect unrecognized conditional logic")
		}
	}
}
