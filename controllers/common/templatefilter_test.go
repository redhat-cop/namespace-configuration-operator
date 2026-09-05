//go:build !integration
// +build !integration

package common

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"text/template"

	"github.com/go-logr/logr"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	utilstemplates "github.com/redhat-cop/operator-utils/pkg/util/templates"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
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
	// literal shapes the renderer sees as null, or not, measured in review (second pass), guarded,
	// unguarded, and outside a guard
	"# comment only\n",
	"---\n",
	"--- # note\n",
	"null\n",
	"~\n",
	"# header comment\n{{- if hasPrefix \"team-\" .Name }}\nkind: Role\n{{- end }}",
	"# header comment\n{{- if hasPrefix \"nomatch-\" .Name }}\nkind: Role\n{{- end }}",
	"---\n{{- if hasPrefix \"nomatch-\" .Name }}\nkind: Role\n{{- end }}",
	"kind: Role\n{{- if hasPrefix \"nomatch-\" .Name }}\n---\n{{- end }}",
	"{{- if .Name }}\n--- # note\n{{- end }}",
	"{{- if .Name }}\nnull\n{{- end }}",
	"{{- if .Name }}\n~\n{{- end }}",
	"{{- if .Name }}\n&anchor\n{{- end }}",
	"{{- if .Name }}\n# comment\n---\nkind: Role\n{{- end }}",
	"{{- if .Name }}\n%YAML 1.1\n---\nkind: Role\n{{- end }}",
	"{{- if .Name }}\nkind: Role\r\n{{- end }}",
	"{{- if .Name }}\n...\nkind: Role\n{{- end }}",
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
	// a pointer-receiver method: resolves on a pointer, fails on the value the renderer uses
	`{{- if hasPrefix "team" .GetName }}
kind: Role
{{- end }}`,
	`{{- if hasSuffix "-prod" .Name }}
kind: Role
metadata:
  name: {{ .GetName }}
{{- end }}`,
	// taken branches that are text but not an object
	`{{- if hasSuffix "-prod" .Name }}
# only a comment
{{- end }}`,
	`{{- if hasSuffix "-prod" .Name }}
---
{{- end }}`,
	`{{- if .Name }}
# comment first
kind: Role
{{- end }}`,
	`{{- if (index .Labels "example.com/oud-group") }}
{{/* a template comment renders nothing */}}
{{- end }}`,
	// review shapes: the renderer parses only the FIRST YAML document
	`{{- if .Name }}
---
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: never-seen
{{- end }}`,
	`{{- if .Name }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: first-doc
{{- end }}`,
	"---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n",
	// review shapes: outside the grammar, decided by rendering
	`{{- range .Labels }}
kind: Role
{{- end }}`,
	`{{- define "cm" }}apiVersion: v1
kind: ConfigMap
metadata:
  name: x
{{- end }}{{ template "cm" . }}`,
	`{{ define "r" }}kind: Role{{ end }}`,
	`{{- with (index .Annotations "missing") }}
kind: Role
{{- end }}`,
	`{{- if .Name }}
data:
{{- end }}`,
	`{{- if eq .Name 5 }}
kind: Role
{{- end }}`,
	`{{- if eq (index .Annotations "k") "v" }}
kind: Role
{{- end }}`,
	`{{ if index .Annotations "enabled" }}
kind: ConfigMap
{{ end }}`,
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

// renderNonBlank is the oracle: exactly what operator-utils' renderer does with the template for
// this object, by value. The contract it encodes, precisely: the filter says "applicable" iff the
// renderer would produce at least one object, OR would fail with anything other than the
// null-output failure. A render whose only failure is "Object 'Kind' is missing in 'null'" (blank,
// comment-only, `---`-only, an empty first document) is NOT applicable: preventing exactly that
// failure is why the filter exists. Any other parse or execution error maps to true so the renderer
// reports it and the reconcile fails visibly.
func renderNonBlank(t *testing.T, funcs template.FuncMap, text string, obj *corev1.Namespace) bool {
	t.Helper()
	tmpl, err := template.New("oracle").Funcs(funcs).Parse(text)
	if err != nil {
		return true
	}
	objs, err := utilstemplates.ProcessTemplateArray(log.IntoContext(context.Background(), logr.Discard()), *obj, tmpl)
	if err != nil {
		var out bytes.Buffer
		if execErr := tmpl.Execute(&out, *obj); execErr != nil {
			return true // execution error: the renderer reports it
		}
		// Executed but produced no object (null, or a YAML error): the renderer would error on
		// this; the filter must therefore have said "not applicable" for a null-only render, or
		// "applicable" for a real YAML error. Distinguish exactly as rendersAnObject does.
		return rendersAnObject(out.Bytes())
	}
	return len(objs) > 0
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

// Render is the controllers' path into the renderer. Its one non-negotiable property, the reason it
// exists, is that a failure is RETURNED: operator-utils' equivalent returns an empty list with a nil
// error, which the enforcer then treats as "delete everything this batch used to own".
func TestRender_ReturnsErrorsInsteadOfAnEmptyBatch(t *testing.T) {
	f := newTestFilter()
	good := apis.LockedResourceTemplate{ObjectTemplate: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: good\n  namespace: {{ .Name }}\n", ExcludedPaths: []string{".metadata"}}
	subject := ns("team-a", map[string]string{oudGroupLabel: "app-x"}, nil)

	cases := []struct {
		name string
		bad  string
		want string // substring the error must carry
	}{
		{"execution error", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Nope }}\n", "failed to render for team-a"},
		{"required label missing", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ required \"team label\" (index .Labels \"team\") }}\n", "team label"},
		{"invalid yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata: [\n", "failed to render"},
		{"parse error", "{{- if bogus .Name }}\nkind: ConfigMap\n{{- end }}", "does not parse"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := f.Render(context.Background(), []apis.LockedResourceTemplate{good, {ObjectTemplate: tc.bad}}, subject)
			if err == nil {
				t.Fatalf("expected an error, got %d resources", len(out))
			}
			if out != nil {
				t.Errorf("a failed render must not return a partial batch, got %d resources", len(out))
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q should mention %q", err.Error(), tc.want)
			}
		})
	}
}

func TestRender_HappyPathCarriesExcludedPathsAndSkipsRejected(t *testing.T) {
	f := newTestFilter()
	templates := []apis.LockedResourceTemplate{
		{ObjectTemplate: chartPrefixGuard, ExcludedPaths: []string{".metadata", ".status"}},
		{ObjectTemplate: chartTruthinessGuard},
		{ObjectTemplate: "- apiVersion: v1\n  kind: ConfigMap\n  metadata:\n    name: a\n- apiVersion: v1\n  kind: ConfigMap\n  metadata:\n    name: b\n"},
	}
	out, err := f.Render(context.Background(), templates, ns("bdp-spark-alpha", map[string]string{oudGroupLabel: "app-bdp-rbac-spark-alpha"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 {
		t.Fatalf("expected Role + RoleBinding + two ConfigMaps, got %d", len(out))
	}
	// the template's two paths plus the defaults, as the enforcer receives them (sorted)
	if out[0].GetKind() != "Role" || !reflect.DeepEqual(out[0].ExcludedPaths, []string{".metadata", ".metadata.finalizers", ".spec.replicas", ".status"}) {
		t.Errorf("first resource should be the Role with its excludedPaths plus the defaults, got %s %v", out[0].GetKind(), out[0].ExcludedPaths)
	}
	if got := out[0].GetNamespace(); got != "bdp-spark-alpha" {
		t.Errorf("the render must see the object's fields, namespace = %q", got)
	}
	// The prefix guard rejects this one, so only the truthiness-guarded binding and the array remain.
	out, err = f.Render(context.Background(), templates, ns("bdp-trino", map[string]string{oudGroupLabel: "app-bdp-rbac-trino"}, nil))
	if err != nil || len(out) != 3 {
		t.Fatalf("expected 3 resources for a rejected prefix, got %d err=%v", len(out), err)
	}
	if out, err := f.Render(context.Background(), templates[:2], ns("plain", nil, nil)); err != nil || len(out) != 0 {
		t.Errorf("no applicable template must give an empty list and no error, got %d err=%v", len(out), err)
	}
	// A taken branch that is only a comment renders to `null`; it is skipped, not rendered into an error.
	commentOnly := apis.LockedResourceTemplate{ObjectTemplate: "{{- if .Name }}\n# nothing but a comment\n{{- end }}\n"}
	out, err = f.Render(context.Background(), []apis.LockedResourceTemplate{templates[2], commentOnly}, ns("plain", nil, nil))
	if err != nil || len(out) != 2 {
		t.Errorf("a comment-only branch must be skipped silently, got %d err=%v", len(out), err)
	}
}

// The renderer has always received the object by VALUE. Pointer-receiver methods therefore must not
// resolve in Render either, or a template that only ever worked in the filter would pass and then
// fail in the real render.
func TestRender_ExecutesAgainstTheValueLikeTheRenderer(t *testing.T) {
	f := newTestFilter()
	tpl := apis.LockedResourceTemplate{ObjectTemplate: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .GetName }}\n"}
	_, err := f.Render(context.Background(), []apis.LockedResourceTemplate{tpl}, ns("team-a", nil, nil))
	if err == nil || !strings.Contains(err.Error(), "GetName") {
		t.Errorf("a pointer-receiver method must fail like it does in the renderer, got err=%v", err)
	}
}

func TestOwnedResources_BestEffortAcrossObjects(t *testing.T) {
	f := newTestFilter()
	templates := []apis.LockedResourceTemplate{{ObjectTemplate: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ required \"team\" (index .Labels \"team\") }}\n  namespace: {{ .Name }}\n"}}
	objs := []metav1.Object{
		ns("a", map[string]string{"team": "x"}, nil),
		ns("b", nil, nil), // cannot render
		ns("c", map[string]string{"team": "y"}, nil),
	}
	owned, failures := f.OwnedResources(context.Background(), templates, objs)
	if len(owned) != 2 || owned[0].GetNamespace() != "a" || owned[1].GetNamespace() != "c" {
		t.Errorf("expected the two renderable objects in order, got %d", len(owned))
	}
	if len(failures) != 1 || !strings.Contains(failures[0].Error(), "for b") {
		t.Errorf("expected exactly one failure naming b, got %v", failures)
	}
}

// The static path judges a literal branch with the renderer's oracle. Every shape here was measured
// against sigs.k8s.io/yaml in review: an empty first document (`---` then `---`), a bare `--- # note`,
// a literal `null` or `~`, and an anchor with no node are `null` to the renderer; a comment before
// the first `---`, a `---` with trailing spaces or a trailing comment, a `%YAML` directive and CRLF
// line endings are not. A `...` document-end marker is a parse error, which is "applicable" so the
// renderer reports it. Three of these were claimed the other way in review; the measurements won.
func TestIsApplicable_FirstDocumentRule(t *testing.T) {
	f := newTestFilter()
	subject := ns("team-a", nil, nil)
	cases := []struct {
		text string
		want bool
	}{
		{"{{- if .Name }}\n---\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n{{- end }}", false},
		{"{{- if .Name }}\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n{{- end }}", true},
		{"{{- if .Name }}\n# comment\n---\nkind: Role\n{{- end }}", true},
		{"{{- if .Name }}\n# only a comment\n{{- end }}", false},
		{"{{- if .Name }}\n--- # note\n{{- end }}", false},
		{"{{- if .Name }}\n--- # note\nkind: Role\n{{- end }}", true},
		{"{{- if .Name }}\n---   \nkind: Role\n{{- end }}", true},
		{"{{- if .Name }}\nnull\n{{- end }}", false},
		{"{{- if .Name }}\n~\n{{- end }}", false},
		{"{{- if .Name }}\n&anchor\n{{- end }}", false},
		{"{{- if .Name }}\n%YAML 1.1\n---\nkind: Role\n{{- end }}", true},
		{"{{- if .Name }}\nkind: Role\r\n{{- end }}", true},
		{"{{- if .Name }}\n...\nkind: Role\n{{- end }}", true},
	}
	for _, tc := range cases {
		if got := f.IsApplicable(apis.LockedResourceTemplate{ObjectTemplate: tc.text}, subject); got != tc.want {
			t.Errorf("IsApplicable = %v, want %v for\n%s", got, tc.want, tc.text)
		}
	}
}

// An unguarded literal template, and literal text around a guard, are judged by the renderer's
// oracle, never by "is there a non-blank line". Measured in review (second pass): a comment-only
// template and a header comment above a guard that is false were "applicable", and the render then
// failed the reconcile for that namespace with "Object 'Kind' is missing in 'null'".
func TestIsApplicable_UnconditionalLiteralUsesYAMLOracle(t *testing.T) {
	f := newTestFilter()
	subject := ns("team-a", nil, nil)
	for _, text := range []string{"", "   \n", "# comment only\n", "---\n", "--- # note\n", "null\n", "~\n", "kind: Role\n", "{not valid YAML"} {
		if got, want := f.IsApplicable(apis.LockedResourceTemplate{ObjectTemplate: text}, subject), rendersAnObject([]byte(text)); got != want {
			t.Errorf("IsApplicable = %v, oracle = %v for %q", got, want, text)
		}
	}
	// a header comment above a guard: the guard's truth decides, as the renderer would
	if f.IsApplicable(apis.LockedResourceTemplate{ObjectTemplate: "# header\n{{- if hasPrefix \"nomatch-\" .Name }}\nkind: Role\n{{- end }}"}, subject) {
		t.Error("a header comment above a false guard renders only the comment: not applicable")
	}
	if !f.IsApplicable(apis.LockedResourceTemplate{ObjectTemplate: "# header\n{{- if hasPrefix \"team-\" .Name }}\nkind: Role\n{{- end }}"}, subject) {
		t.Error("a header comment above a true guard renders the object: applicable")
	}
}

// The enforcer is handed the author's paths plus the defaults, sorted; the template's own list is
// not touched (the CR spec is the author's, issue #16).
func TestRender_CarriesEffectiveExcludedPaths(t *testing.T) {
	f := newTestFilter()
	subject := ns("team-a", nil, nil)
	tmpl := apis.LockedResourceTemplate{ObjectTemplate: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n  namespace: {{ .Name }}\n", ExcludedPaths: []string{".data.b", ".status"}}
	out, err := f.Render(context.Background(), []apis.LockedResourceTemplate{tmpl}, subject)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".data.b", ".metadata.finalizers", ".spec.replicas", ".status"}
	if len(out) != 1 || !reflect.DeepEqual(out[0].ExcludedPaths, want) {
		t.Fatalf("effective excluded paths = %v, want %v", out[0].ExcludedPaths, want)
	}
	if !reflect.DeepEqual(tmpl.ExcludedPaths, []string{".data.b", ".status"}) {
		t.Fatalf("the template's own list must be untouched, got %v", tmpl.ExcludedPaths)
	}
	if reflect.DeepEqual(EffectiveExcludedPaths([]string{".status", ".data.b"}), EffectiveExcludedPaths([]string{".data.b"})) == false {
		t.Fatal("the effective list is a set: order and duplicates in the author's list must not matter")
	}
}

// The documentation states the defaults the code applies; a change to one without the other fails
// here (review of PR #40: README and CSV still promised .metadata after the code dropped it).
func TestDefaultExcludedPathsAreDocumented(t *testing.T) {
	var lines []string
	for i, p := range DefaultExcludedPaths {
		lines = append(lines, fmt.Sprintf("%d. `%s`", i+1, p))
	}
	want := "The following paths are always included:\n\n" + strings.Join(lines, "\n")
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), want) {
		t.Fatalf("README.md does not document the current defaults:\n%s", want)
	}
	raw, err := os.ReadFile("../../config/manifests/bases/namespace-configuration-operator.clusterserviceversion.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var csv struct {
		Spec struct {
			Description string `json:"description"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal(raw, &csv); err != nil {
		t.Fatal(err)
	}
	for _, p := range DefaultExcludedPaths {
		if !strings.Contains(csv.Spec.Description, "`"+p+"`") {
			t.Fatalf("the CSV description does not mention default excluded path %s", p)
		}
	}
	if strings.Contains(csv.Spec.Description, "1. `.metadata`") {
		t.Fatal("the CSV description still lists .metadata as always included")
	}
	features, err := os.ReadFile("../../docs/FEATURES_AND_ISSUES_RESOLUTION.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(features), "`DefaultExcludedPaths` is now `.status` and `.spec.replicas`") {
		t.Fatal("FEATURES_AND_ISSUES_RESOLUTION.md still states a previous default set")
	}
	design, err := os.ReadFile("../../docs/DESIGN_excludedPaths.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(design), "count grows per\nreconcile)") {
		t.Fatal("DESIGN_excludedPaths.md still describes the MetadataExcluded event as emitted every reconcile")
	}
}
