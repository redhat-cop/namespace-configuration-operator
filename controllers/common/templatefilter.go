package common

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"

	"github.com/go-logr/logr"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	"github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedresource"
	utilstemplates "github.com/redhat-cop/operator-utils/pkg/util/templates"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

// TemplateFilter decides, per selected object (Namespace, Group or User), whether an objectTemplate
// would render anything for it, so the reconcilers can skip it BEFORE handing it to operator-utils.
//
// WHY THE SKIP MATTERS. A guarded template such as
//
//	{{- if hasPrefix "team-a-" (index .Labels "example.com/team") }} ... {{- end }}
//
// renders to an empty string for every object the guard rejects. operator-utils turns that empty
// string into the JSON literal `null`, fails it with "Object 'Kind' is missing in 'null'", logs the
// failure at error level, and then DROPS every object it had already rendered for that param while
// returning a nil error — so the CR reports success, the log fills with an error per rejected object
// per reconcile, and any other template in the same batch silently loses its objects. Deciding
// applicability here, with the same semantics the renderer applies, is what prevents all of that.
//
// HOW IT DECIDES. The template is parsed once (cached by its text) with the same function map the
// renderer uses, and its syntax tree is inspected:
//
//   - A template whose top level is a single if / else-if / else chain over the recognised guard
//     functions is evaluated statically against the object's name, labels and annotations. The
//     recognised guard grammar is: hasPrefix, hasSuffix, contains, eq, ne, and, or, not, the bare
//     truthiness of `.Name` or of `(index .Labels "key")` / `(index .Annotations "key")`, and
//     parenthesised nesting of those. Operands are string literals, `.Name`, or an index into
//     .Labels / .Annotations. This is the cheap path and it covers every guard this operator's
//     own policies use.
//   - Anything the static evaluator does not fully understand — a variable declaration at the top
//     level, a pipeline, a `range`, a lookup, `.Spec` access, a function outside the grammar — is
//     decided by RENDERING the template and checking whether the output is blank. That is always
//     correct because it is exactly what the renderer will see; it merely costs a second render for
//     that template.
//   - A template that does not parse is left to the renderer, which reports the parse error with
//     the context it already has.
//
// Unconditional templates (any non-blank text outside a top-level guard) always apply, exactly as
// before this filter existed.
type TemplateFilter struct {
	log   logr.Logger
	funcs template.FuncMap
	// cache maps template text to *parsedTemplate. Templates are immutable once read from a CR, and
	// text/template values are safe for concurrent Execute, so one parse serves every reconcile.
	cache sync.Map
}

type parsedTemplate struct {
	tmpl *template.Template
	err  error
}

// NewTemplateFilter builds a filter whose render fallback uses the same function map as the real
// renderer. restConfig may be nil (unit tests); only the API-lookup functions need it, and they are
// only reached by the render fallback.
func NewTemplateFilter(log logr.Logger, restConfig *rest.Config) *TemplateFilter {
	return &TemplateFilter{
		log:   log,
		funcs: utilstemplates.AdvancedTemplateFuncMap(restConfig, log),
	}
}

// FilterApplicable returns the templates that would render at least one object for obj.
func (f *TemplateFilter) FilterApplicable(templates []apis.LockedResourceTemplate, obj metav1.Object) []apis.LockedResourceTemplate {
	applicable := []apis.LockedResourceTemplate{}
	for _, t := range templates {
		if f.IsApplicable(t, obj) {
			applicable = append(applicable, t)
		}
	}
	return applicable
}

// IsApplicable reports whether tpl would render something for obj. obj is also the data the render
// fallback executes the template against, so it must be the same object the renderer receives.
func (f *TemplateFilter) IsApplicable(tpl apis.LockedResourceTemplate, obj metav1.Object) bool {
	pt := f.parse(tpl.ObjectTemplate)
	if pt.err != nil {
		// The renderer will fail on the same text and log it with the resource attached; deciding
		// "not applicable" here would hide that failure instead.
		f.log.V(1).Info("template does not parse, leaving it to the renderer", "object", obj.GetName(), "error", pt.err.Error())
		return true
	}

	s := subject{name: obj.GetName(), labels: obj.GetLabels(), annotations: obj.GetAnnotations()}
	if decided, applicable := evaluateStatically(pt.tmpl.Tree.Root, s); decided {
		f.log.V(2).Info("template applicability decided statically", "object", obj.GetName(), "applicable", applicable, "template", preview(tpl.ObjectTemplate))
		return applicable
	}

	// The fallback executes against the same VALUE the renderer receives (renderData), never the
	// pointer: a pointer would resolve pointer-receiver methods such as {{ .GetName }} that the real
	// render then fails on, and the two paths must agree. An execution error is left to the renderer
	// so it is reported there, with the CR attached.
	var out bytes.Buffer
	if err := pt.tmpl.Execute(&out, renderData(obj)); err != nil {
		f.log.V(1).Info("template applicability could not be decided by rendering, leaving it to the renderer", "object", obj.GetName(), "error", err.Error())
		return true
	}
	applicable := rendersAnObject(out.Bytes())
	f.log.V(2).Info("template applicability decided by rendering", "object", obj.GetName(), "applicable", applicable, "template", preview(tpl.ObjectTemplate))
	return applicable
}

// rendersAnObject decides what the renderer will make of a render. "Non-blank" is not enough: a
// YAML comment or a bare `---` is non-blank text that YAMLToJSON turns into the literal `null`,
// which the renderer rejects with "Object 'Kind' is missing in 'null'". Output that does not parse
// is left to the renderer (true), so the parse error is reported there.
func rendersAnObject(out []byte) bool {
	if len(bytes.TrimSpace(out)) == 0 {
		return false
	}
	j, err := yaml.YAMLToJSON(out)
	if err != nil {
		return true
	}
	return !bytes.Equal(bytes.TrimSpace(j), []byte("null"))
}

// Render turns the templates that apply to obj into LockedResources, using the renderer's own
// ProcessTemplateArray on the filter's cached parse, and RETURNS every error.
//
// WHY THE CONTROLLERS RENDER HERE RATHER THAN THROUGH operator-utils' GetLockedResourcesFromTemplates*.
// That function logs a parse or render failure and then returns an EMPTY list with a nil error, so
// a caller cannot tell "nothing wanted" from "something went wrong". The enforcing reconciler then
// treats the empty list as the desired state and deletes every object it was enforcing for that
// batch, while the CR reports ReconcileSuccess (measured: a `required` label removed from one
// namespace deleted that namespace's RoleBinding under a green status). Returning the error lets the
// reconcile end in ManageError, which records the failure on the CR and never reaches the enforcer.
// It also keeps the controllers off that function's unsynchronised package-global template map.
//
// The template is executed against the VALUE obj points to, which is what the renderer has always
// received: a pointer would additionally resolve pointer-receiver methods that a later real render
// would not, and the two must agree.
func (f *TemplateFilter) Render(ctx context.Context, templates []apis.LockedResourceTemplate, obj metav1.Object) ([]lockedresource.LockedResource, error) {
	ctx = log.IntoContext(ctx, f.log)
	data := renderData(obj)
	out := []lockedresource.LockedResource{}
	for i, t := range templates {
		if !f.IsApplicable(t, obj) {
			continue
		}
		pt := f.parse(t.ObjectTemplate)
		if pt.err != nil {
			return nil, fmt.Errorf("template %d does not parse: %w (template starts %q)", i, pt.err, preview(t.ObjectTemplate))
		}
		objs, err := utilstemplates.ProcessTemplateArray(ctx, data, pt.tmpl)
		if err != nil {
			return nil, fmt.Errorf("template %d failed to render for %s: %w (template starts %q)", i, obj.GetName(), err, preview(t.ObjectTemplate))
		}
		for _, o := range objs {
			out = append(out, lockedresource.LockedResource{Unstructured: o, ExcludedPaths: EffectiveExcludedPaths(t.ExcludedPaths)})
		}
	}
	return out, nil
}

// renderData is the value the renderer executes against: the object itself, not a pointer to it.
func renderData(obj metav1.Object) any {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		return v.Elem().Interface()
	}
	return obj
}

func (f *TemplateFilter) parse(text string) *parsedTemplate {
	if cached, ok := f.cache.Load(text); ok {
		return cached.(*parsedTemplate)
	}
	tmpl, err := template.New("objectTemplate").Funcs(f.funcs).Parse(text)
	pt := &parsedTemplate{tmpl: tmpl, err: err}
	actual, _ := f.cache.LoadOrStore(text, pt)
	return actual.(*parsedTemplate)
}

func preview(text string) string {
	if len(text) > 100 {
		return text[:100] + "..."
	}
	return text
}

// subject is the string view of an object that the recognised guards can read.
type subject struct {
	name        string
	labels      map[string]string
	annotations map[string]string
}

// evaluateStatically inspects the template's top level. It returns decided=false whenever the
// shape or the guard expression is outside the recognised grammar, so the caller renders instead.
func evaluateStatically(root *parse.ListNode, s subject) (decided bool, applicable bool) {
	var guard *parse.IfNode
	var outside []byte
	for _, n := range root.Nodes {
		switch n := n.(type) {
		case *parse.TextNode:
			outside = append(outside, n.Text...)
		case *parse.IfNode:
			if guard != nil {
				return false, false
			}
			guard = n
		default:
			// A declaration, an action, a range, a with, a template call: the render decides.
			return false, false
		}
	}
	if guard == nil {
		// Literal only: the renderer's oracle decides exactly. Measured in review (second pass): the
		// previous "any non-blank text is content" rule declared `# comment`, `---`, `null` and `~`
		// applicable, and the render then failed the whole reconcile with "Kind is missing in null".
		return true, rendersAnObject(outside)
	}
	if len(bytes.TrimSpace(outside)) > 0 {
		// Text outside the guard (a header comment, a `---`) can itself be null to the renderer, or
		// combine with document markers the branch prints; only the render can tell.
		return false, false
	}
	return evaluateGuardChain(guard, s)
}

// evaluateGuardChain walks an if / else if / else chain. text/template parses `{{ else if }}` as an
// else list holding exactly one IfNode, which is also the shape of `{{ else }}{{ if }}…{{ end }}` and
// means the same thing, so both are followed.
func evaluateGuardChain(n *parse.IfNode, s subject) (decided bool, applicable bool) {
	for {
		cond, ok := evalBoolPipe(n.Pipe, s)
		if !ok {
			return false, false
		}
		if cond {
			return listHasContent(n.List)
		}
		if n.ElseList == nil {
			return true, false
		}
		if len(n.ElseList.Nodes) == 1 {
			if next, isIf := n.ElseList.Nodes[0].(*parse.IfNode); isIf {
				n = next
				continue
			}
		}
		return listHasContent(n.ElseList)
	}
}

// listHasContent reports whether a taken branch renders an object. The branch's literal text is
// judged by the same oracle the render fallback uses, rendersAnObject: YAMLToJSON of the text is not
// `null`. That is exactly what operator-utils' renderer sees (sigs.k8s.io/yaml parses only the FIRST
// YAML document), so a comment-only branch, a `---` followed by another `---`, a bare `--- # note`,
// a literal `null` or `~`, or an anchor with no node are all "no object", as measured in review. A
// branch made only of actions (no literal text) is left to the render fallback, since an action can
// legitimately print nothing.
func listHasContent(list *parse.ListNode) (decided bool, content bool) {
	if list == nil {
		return true, false
	}
	var text []byte
	actions := false
	for _, n := range list.Nodes {
		if t, isText := n.(*parse.TextNode); isText {
			text = append(text, t.Text...)
			continue
		}
		actions = true
	}
	if rendersAnObject(text) {
		return true, true
	}
	if actions {
		return false, false
	}
	return true, false
}

func evalBoolPipe(p *parse.PipeNode, s subject) (value bool, ok bool) {
	if p == nil || len(p.Decl) != 0 || len(p.Cmds) != 1 {
		return false, false
	}
	return evalBoolCmd(p.Cmds[0], s)
}

// evalBoolCmd evaluates one command in boolean context, with Go template truthiness: every operand
// this grammar produces is a string, and a string is true when non-empty.
func evalBoolCmd(c *parse.CommandNode, s subject) (value bool, ok bool) {
	if len(c.Args) == 0 {
		return false, false
	}
	ident, isIdent := c.Args[0].(*parse.IdentifierNode)
	if !isIdent {
		if len(c.Args) != 1 {
			return false, false
		}
		return evalBoolNode(c.Args[0], s)
	}
	args := c.Args[1:]
	switch ident.Ident {
	case "hasPrefix", "hasSuffix", "contains":
		// Sprig argument order: (needle, haystack).
		if len(args) != 2 {
			return false, false
		}
		needle, ok1 := evalStringNode(args[0], s)
		haystack, ok2 := evalStringNode(args[1], s)
		if !ok1 || !ok2 {
			return false, false
		}
		switch ident.Ident {
		case "hasPrefix":
			return strings.HasPrefix(haystack, needle), true
		case "hasSuffix":
			return strings.HasSuffix(haystack, needle), true
		default:
			return strings.Contains(haystack, needle), true
		}
	case "eq":
		// eq arg1 arg2 arg3… is true when arg1 equals any of the others.
		if len(args) < 2 {
			return false, false
		}
		first, ok := evalStringNode(args[0], s)
		if !ok {
			return false, false
		}
		result := false
		for _, a := range args[1:] {
			v, ok := evalStringNode(a, s)
			if !ok {
				return false, false
			}
			result = result || v == first
		}
		return result, true
	case "ne":
		if len(args) != 2 {
			return false, false
		}
		a, ok1 := evalStringNode(args[0], s)
		b, ok2 := evalStringNode(args[1], s)
		if !ok1 || !ok2 {
			return false, false
		}
		return a != b, true
	case "and", "or":
		if len(args) == 0 {
			return false, false
		}
		for _, a := range args {
			v, ok := evalBoolNode(a, s)
			if !ok {
				return false, false
			}
			if ident.Ident == "and" && !v {
				return false, true
			}
			if ident.Ident == "or" && v {
				return true, true
			}
		}
		return ident.Ident == "and", true
	case "not":
		if len(args) != 1 {
			return false, false
		}
		v, ok := evalBoolNode(args[0], s)
		return !v, ok
	case "index":
		v, ok := evalStringCmd(c, s)
		return v != "", ok
	}
	return false, false
}

func evalBoolNode(n parse.Node, s subject) (value bool, ok bool) {
	switch n := n.(type) {
	case *parse.PipeNode:
		return evalBoolPipe(n, s)
	default:
		v, ok := evalStringNode(n, s)
		return v != "", ok
	}
}

// evalStringNode resolves an operand: a string literal, `.Name`, or a parenthesised
// `(index .Labels "key")` / `(index .Annotations "key")`.
func evalStringNode(n parse.Node, s subject) (value string, ok bool) {
	switch n := n.(type) {
	case *parse.StringNode:
		return n.Text, true
	case *parse.FieldNode:
		if isField(n, "Name") {
			return s.name, true
		}
		return "", false
	case *parse.PipeNode:
		if len(n.Decl) != 0 || len(n.Cmds) != 1 {
			return "", false
		}
		return evalStringCmd(n.Cmds[0], s)
	}
	return "", false
}

// evalStringCmd resolves `index .Labels "key"` and `index .Annotations "key"`, or a lone operand.
// A missing key yields "", which is what `index` returns for a map[string]string.
func evalStringCmd(c *parse.CommandNode, s subject) (value string, ok bool) {
	if len(c.Args) == 1 {
		return evalStringNode(c.Args[0], s)
	}
	if len(c.Args) != 3 {
		return "", false
	}
	ident, isIdent := c.Args[0].(*parse.IdentifierNode)
	field, isField := c.Args[1].(*parse.FieldNode)
	key, isKey := c.Args[2].(*parse.StringNode)
	if !isIdent || ident.Ident != "index" || !isField || !isKey {
		return "", false
	}
	switch {
	case isMetaField(field, "Labels"):
		return s.labels[key.Text], true
	case isMetaField(field, "Annotations"):
		return s.annotations[key.Text], true
	}
	return "", false
}

// isField matches `.Field`.
func isField(f *parse.FieldNode, name string) bool {
	return len(f.Ident) == 1 && f.Ident[0] == name
}

// isMetaField matches `.Field` and its explicit spelling `.ObjectMeta.Field`.
func isMetaField(f *parse.FieldNode, name string) bool {
	if isField(f, name) {
		return true
	}
	return len(f.Ident) == 2 && f.Ident[0] == "ObjectMeta" && f.Ident[1] == name
}

// OwnedResources renders what a CR owns for each selected object, for deletion. It is best effort by
// design: an object whose templates no longer render (a label removed since the object was created,
// a template edited into a parse error) must not leave a CR stuck in Terminating forever, so its
// failure is returned alongside whatever did render and the caller decides how loudly to say so.
//
// WHY THE CALLER CANNOT RELY ON THE ENFORCER FOR THIS. The enforcer only deletes what its in-memory
// manager was started with. That map is empty after an operator restart, and a failed Terminate
// removes the entry, so a CR being deleted in either state would finalize with every managed object
// left behind. Recomputing the owned set from the spec is the only source that survives both.
func (f *TemplateFilter) OwnedResources(ctx context.Context, templates []apis.LockedResourceTemplate, objs []metav1.Object) (owned []unstructured.Unstructured, failures []error) {
	for _, obj := range objs {
		lrs, err := f.Render(ctx, templates, obj)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		for _, lr := range lrs {
			owned = append(owned, lr.Unstructured)
		}
	}
	return owned, failures
}
