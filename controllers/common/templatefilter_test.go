//go:build !integration
// +build !integration

package common

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"text/template"

	"github.com/go-logr/logr"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	utilstemplates "github.com/redhat-cop/operator-utils/pkg/util/templates"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const oudGroupLabel = "example.com/oud-group"

// The guard the openshift-rbac-automation chart emits for an oudGroup policy, verbatim in shape.
const chartPrefixGuard = `{{- if hasPrefix "app-bdp-rbac-spark-" (index .Labels "example.com/oud-group") }}
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: bdp-spark-job-submitter-role
  namespace: "{{ .Name }}"
{{- end }}
`

// The chart's other guard shape: groupPrefix unset, so only a non-empty label value qualifies.
const chartTruthinessGuard = `{{- if (index .Labels "example.com/oud-group") }}
kind: RoleBinding
metadata:
  name: "{{ index .Labels "example.com/oud-group" }}-rb"
{{- end }}
`

func ns(name string, labels map[string]string, annotations map[string]string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels, Annotations: annotations}}
}

func newTestFilter() *TemplateFilter {
	return NewTemplateFilter(logr.Discard(), nil)
}

func TestIsApplicable_ChartGuards(t *testing.T) {
	f := newTestFilter()
	tests := []struct {
		name     string
		template string
		ns       *corev1.Namespace
		want     bool
	}{
		{"prefix guard, label in the family", chartPrefixGuard, ns("bdp-spark-alpha-qa", map[string]string{oudGroupLabel: "app-bdp-rbac-spark-alpha"}, nil), true},
		{"prefix guard, label in another family", chartPrefixGuard, ns("bdp-trino-apps-qa", map[string]string{oudGroupLabel: "app-bdp-rbac-trino-apps"}, nil), false},
		{"prefix guard, exact prefix without a suffix is NOT a match of hasPrefix? it is", chartPrefixGuard, ns("x", map[string]string{oudGroupLabel: "app-bdp-rbac-spark-"}, nil), true},
		{"prefix guard, empty label value", chartPrefixGuard, ns("bdp-empty-qa", map[string]string{oudGroupLabel: ""}, nil), false},
		{"prefix guard, label absent", chartPrefixGuard, ns("plain", map[string]string{"other": "x"}, nil), false},
		{"prefix guard, nil labels", chartPrefixGuard, ns("plain", nil, nil), false},
		{"truthiness guard, value present", chartTruthinessGuard, ns("a", map[string]string{oudGroupLabel: "team-a"}, nil), true},
		{"truthiness guard, value empty", chartTruthinessGuard, ns("a", map[string]string{oudGroupLabel: ""}, nil), false},
		{"truthiness guard, label absent", chartTruthinessGuard, ns("a", nil, nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.IsApplicable(apis.LockedResourceTemplate{ObjectTemplate: tt.template}, tt.ns)
			if got != tt.want {
				t.Errorf("IsApplicable = %v, want %v", got, tt.want)
			}
		})
	}
}

// propertyTemplates is the grammar surface plus the shapes that must fall back to rendering. The
// property test below checks every one of them against every subject: the filter's answer must
// equal "the real render is non-blank" — that equivalence is the whole contract.
var propertyTemplates = []string{
	chartPrefixGuard,
	chartTruthinessGuard,
	"",
	"   \n\t\n",
	"kind: Role\nmetadata:\n  name: unconditional\n",
	"- kind: Role\n  metadata:\n    name: a\n- kind: Role\n  metadata:\n    name: b\n",
	// name-based guards, the pre-existing grammar
	`{{- if hasSuffix "-prod" .Name }}
kind: RoleBinding
{{- end }}`,
	`{{- if contains "monitoring" .Name }}
kind: Role
{{- end }}`,
	`{{- if hasPrefix "team-" .Name }}
kind: Role
{{- end }}`,
	// else-if chain and bare else
	`{{- if hasSuffix "-prod" .Name }}
prod
{{- else if contains "monitoring" .Name }}
monitoring
{{- end }}`,
	`{{- if hasSuffix "-prod" .Name }}
prod
{{- else }}
everything else
{{- end }}`,
	`{{- if hasSuffix "-prod" .Name }}
prod
{{- else }}
{{- end }}`,
	// boolean combinators
	`{{- if and (hasSuffix "-prod" .Name) (contains "my-app" .Name) }}
kind: RoleBinding
{{- end }}`,
	`{{- if or (hasSuffix "-prod" .Name) (contains "monitoring" .Name) }}
kind: RoleBinding
{{- end }}`,
	`{{- if not (hasSuffix "-prod" .Name) }}
kind: RoleBinding
{{- end }}`,
	`{{- if and (hasPrefix "app-" (index .Labels "example.com/oud-group")) (not (hasSuffix "-prod" .Name)) }}
kind: RoleBinding
{{- end }}`,
	`{{- if or (index .Labels "example.com/oud-group") (index .Annotations "allow-pvc") }}
kind: RoleBinding
{{- end }}`,
	// equality, including the operator's own documented example
	`{{- if eq .Name "admin" }}
kind: ConfigMap
{{- end }}`,
	`{{- if eq .Name "admin" "my-app-prod" }}
kind: ConfigMap
{{- end }}`,
	`{{- if ne (index .Annotations "allow-pvc") "true" }}
kind: ResourceQuota
{{- end }}`,
	`{{- if eq (index .Labels "example.com/oud-group") "team-a" }}
kind: Role
{{- end }}`,
	// label value as the needle, explicit ObjectMeta spelling, bare .Name truthiness
	`{{- if hasPrefix (index .Labels "example.com/oud-group") .Name }}
kind: Role
{{- end }}`,
	`{{- if hasPrefix "app-" (index .ObjectMeta.Labels "example.com/oud-group") }}
kind: Role
{{- end }}`,
	`{{- if .Name }}
kind: Role
{{- end }}`,
	// unconditional text around a guard: never blank
	`apiVersion: v1
kind: ConfigMap
{{- if hasSuffix "-prod" .Name }}
data:
  prod: "true"
{{- end }}`,
	// shapes outside the grammar: decided by rendering
	`{{- $g := index .Labels "example.com/oud-group" }}
{{- if hasPrefix "app-" $g }}
kind: Role
{{- end }}`,
	`{{- if .Name | hasPrefix "team-" }}
kind: Role
{{- end }}`,
	`{{- if .Spec.Finalizers }}
kind: Role
{{- end }}`,
	`{{- if hasSuffix "-prod" .Name }}
{{ .Name }}
{{- end }}`,
	`{{- if hasSuffix "-prod" .Name }}
a
{{- end }}
{{- if contains "monitoring" .Name }}
b
{{- end }}`,
	`{{- range .Spec.Finalizers }}
kind: Role
{{- end }}`,
	`{{- with (index .Labels "example.com/oud-group") }}
kind: Role
name: {{ . }}
{{- end }}`,
	// a label read as a field: a missing key is an error at render time, which the renderer must see
	`{{- if hasPrefix "app-" .Labels.team }}
kind: Role
{{- end }}`,
	// a function the renderer does not know: a parse error, which the renderer must see
	`{{- if bogus .Name }}
kind: Role
{{- end }}`,
}

var propertySubjects = []*corev1.Namespace{
	ns("my-app-prod", nil, nil),
	ns("my-app-prod", map[string]string{oudGroupLabel: "app-bdp-rbac-spark-alpha"}, map[string]string{"allow-pvc": "true"}),
	ns("user-workload-monitoring", map[string]string{oudGroupLabel: "team-a"}, map[string]string{"allow-pvc": "false"}),
	ns("team-x", map[string]string{oudGroupLabel: ""}, nil),
	ns("admin", map[string]string{"team": "app-1"}, nil),
	ns("plain", map[string]string{}, map[string]string{}),
	{ObjectMeta: metav1.ObjectMeta{Name: "finalized"}, Spec: corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{"kubernetes"}}},
}

// renderNonBlank is the oracle: what operator-utils would do with the template for this object,
// reduced to "would it produce an object". A render error maps to true because the filter must
// leave such a template to the renderer so the error is reported there.
func renderNonBlank(t *testing.T, funcs template.FuncMap, text string, obj *corev1.Namespace) bool {
	t.Helper()
	tmpl, err := template.New("oracle").Funcs(funcs).Parse(text)
	if err != nil {
		return true
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, obj); err != nil {
		return true
	}
	return len(bytes.TrimSpace(out.Bytes())) > 0
}

func TestIsApplicable_MatchesRenderer(t *testing.T) {
	f := newTestFilter()
	funcs := utilstemplates.AdvancedTemplateFuncMap(nil, logr.Discard())
	for i, text := range propertyTemplates {
		for j, obj := range propertySubjects {
			t.Run(fmt.Sprintf("template-%02d/subject-%d", i, j), func(t *testing.T) {
				want := renderNonBlank(t, funcs, text, obj)
				got := f.IsApplicable(apis.LockedResourceTemplate{ObjectTemplate: text}, obj)
				if got != want {
					t.Errorf("filter says %v, renderer says %v\nsubject: name=%q labels=%v annotations=%v\ntemplate:\n%s", got, want, obj.Name, obj.Labels, obj.Annotations, text)
				}
			})
		}
	}
}

// TestEvaluateStatically_Coverage pins WHICH path decides each shape. The property test proves the
// answers are right; this proves the cheap path is taken where it should be and the render fallback
// where it must be, so a grammar regression cannot hide behind the fallback.
func TestEvaluateStatically_Coverage(t *testing.T) {
	f := newTestFilter()
	staticallyDecided := map[string]bool{}
	for _, text := range propertyTemplates {
		pt := f.parse(text)
		if pt.err != nil {
			continue
		}
		decided, _ := evaluateStatically(pt.tmpl.Tree.Root, subject{name: "my-app-prod", labels: map[string]string{oudGroupLabel: "app-x"}, annotations: map[string]string{"allow-pvc": "true"}})
		staticallyDecided[text] = decided
	}
	mustBeStatic := []string{chartPrefixGuard, chartTruthinessGuard, "", "kind: Role\nmetadata:\n  name: unconditional\n"}
	for _, text := range mustBeStatic {
		if !staticallyDecided[text] {
			t.Errorf("expected the static evaluator to decide:\n%s", text)
		}
	}
	mustRender := []string{
		"{{- $g := index .Labels \"example.com/oud-group\" }}\n{{- if hasPrefix \"app-\" $g }}\nkind: Role\n{{- end }}",
		"{{- if .Name | hasPrefix \"team-\" }}\nkind: Role\n{{- end }}",
		"{{- if .Spec.Finalizers }}\nkind: Role\n{{- end }}",
		"{{- if hasSuffix \"-prod\" .Name }}\n{{ .Name }}\n{{- end }}",
		"{{- if hasPrefix \"app-\" .Labels.team }}\nkind: Role\n{{- end }}",
	}
	for _, text := range mustRender {
		if decided, present := staticallyDecided[text]; !present {
			t.Errorf("template missing from propertyTemplates:\n%s", text)
		} else if decided {
			t.Errorf("expected the render fallback to decide:\n%s", text)
		}
	}
	static, rendered := 0, 0
	for _, d := range staticallyDecided {
		if d {
			static++
		} else {
			rendered++
		}
	}
	t.Logf("statically decided: %d, decided by rendering: %d", static, rendered)
}

func TestFilterApplicable(t *testing.T) {
	f := newTestFilter()
	templates := []apis.LockedResourceTemplate{
		{ObjectTemplate: chartPrefixGuard},
		{ObjectTemplate: chartTruthinessGuard},
		{ObjectTemplate: "kind: Role\nmetadata:\n  name: unconditional\n"},
	}
	got := f.FilterApplicable(templates, ns("bdp-trino-qa", map[string]string{oudGroupLabel: "app-bdp-rbac-trino"}, nil))
	if len(got) != 2 {
		t.Fatalf("expected the truthiness guard and the unconditional template, got %d templates", len(got))
	}
	if got[0].ObjectTemplate != chartTruthinessGuard || got[1].ObjectTemplate != "kind: Role\nmetadata:\n  name: unconditional\n" {
		t.Errorf("wrong templates kept, or order not preserved")
	}
	if got := f.FilterApplicable(templates, ns("bdp-empty-qa", map[string]string{oudGroupLabel: ""}, nil)); len(got) != 1 {
		t.Errorf("expected only the unconditional template for an empty label value, got %d", len(got))
	}
}

func TestIsApplicable_ConcurrentUseSharesTheCache(t *testing.T) {
	f := newTestFilter()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			obj := ns(fmt.Sprintf("bdp-spark-%d-qa", i), map[string]string{oudGroupLabel: "app-bdp-rbac-spark-x"}, nil)
			if !f.IsApplicable(apis.LockedResourceTemplate{ObjectTemplate: chartPrefixGuard}, obj) {
				t.Errorf("worker %d: expected applicable", i)
			}
		}(i)
	}
	wg.Wait()
	entries := 0
	f.cache.Range(func(_, _ any) bool { entries++; return true })
	if entries != 1 {
		t.Errorf("expected one cached parse for one template text, got %d", entries)
	}
}

func TestIsApplicable_DoesNotReadGuardsFromComments(t *testing.T) {
	// A YAML comment is template TEXT, so a pattern-looking string inside it must not influence the
	// decision. The chart's own templates carry such comments.
	f := newTestFilter()
	text := "# guard would be: hasSuffix \"-prod\" .Name — but there is no guard here\nkind: Role\n"
	if !f.IsApplicable(apis.LockedResourceTemplate{ObjectTemplate: text}, ns("my-app-dev", nil, nil)) {
		t.Errorf("an unconditional template with a pattern in a comment must still apply")
	}
	if !strings.Contains(text, "hasSuffix") {
		t.Fatal("test setup: the comment must mention a guard function")
	}
}
