package common

import (
	"fmt"
	"sort"

	"github.com/scylladb/go-set/strset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// DefaultExcludedPaths are unioned into every template's excludedPaths (see IsInitialized in each
// controller): the enforcer sets an excluded path when it creates the object and never enforces it
// again. `.status` is the server's; `.spec.replicas` belongs to an autoscaler once set.
//
// `.metadata` is no longer here. It was excluded wholesale because the merge-patch enforcer could
// not tell a label the template rendered from one another actor added: every foreign label was a
// permanent difference and a patch on every sync. The enforcer now applies server-side and owns
// only what the template renders, so a rendered label or annotation is enforced (drift on it is
// corrected, issue #16) while a label added by anyone else is left alone. The server-populated
// metadata (uid, resourceVersion, creationTimestamp, managedFields) is never in a template, so it
// needs no exclusion. `.metadata.finalizers` is: a finalizer names the controller that owns that
// piece of lifecycle protocol, and a template that renders one (a copied `oc get -o yaml`) must not
// make this operator re-add it after that controller removed it (review of PR #40). A CR that still
// lists `.metadata` keeps the old behaviour for its objects.
//
// These are applied IN MEMORY when the locked resources are built (EffectiveExcludedPaths); the CR's
// spec is never rewritten to include them. It used to be: IsInitialized unioned them into
// spec.templates[].excludedPaths and wrote the CR, which made every CR differ from what its author
// or their Git declared, and with a GitOps controller healing the spec back, a rewrite loop
// (recorded in the chart that deploys this operator, 0.21.1). The author's list is the author's.
var DefaultExcludedPaths = []string{".metadata.finalizers", ".status", ".spec.replicas"}

// DefaultExcludedPathsSet represents paths that are excluded by default in all resources
var DefaultExcludedPathsSet = strset.New(DefaultExcludedPaths...)

// EffectiveExcludedPaths is what the enforcer is handed for a template: the author's excluded paths
// unioned with the defaults, sorted so the same input always gives the same list (a locked
// resource's identity includes its excluded paths; an unstable order would look like a change and
// restart its reconciler for nothing).
func EffectiveExcludedPaths(declared []string) []string {
	paths := strset.Union(DefaultExcludedPathsSet, strset.New(declared...)).List()
	sort.Strings(paths)
	return paths
}

// ResourceGenerationOrFinalizerOrDeletionTimestampChangedPredicate is a predicate that triggers reconciliation when:
// 1. Resource generation changes (spec updates)
// 2. Finalizers change (added or removed)
// 3. Deletion timestamp changes (resource marked for deletion or deletion timestamp removed)
//
// This is an extension of ResourceGenerationOrFinalizerChangedPredicate that also handles
// deletion timestamp changes, which is critical for proper cleanup of resources stuck in deletion.
var ResourceGenerationOrFinalizerOrDeletionTimestampChangedPredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		// Check if generation changed (spec update)
		if e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
			return true
		}

		// Check if finalizers changed
		oldFinalizers := e.ObjectOld.GetFinalizers()
		newFinalizers := e.ObjectNew.GetFinalizers()
		if len(oldFinalizers) != len(newFinalizers) {
			return true
		}
		for i := range oldFinalizers {
			if oldFinalizers[i] != newFinalizers[i] {
				return true
			}
		}

		// Check if deletion timestamp changed
		oldDeletionTimestamp := e.ObjectOld.GetDeletionTimestamp()
		newDeletionTimestamp := e.ObjectNew.GetDeletionTimestamp()

		// Deletion timestamp was set (resource marked for deletion)
		if oldDeletionTimestamp == nil && newDeletionTimestamp != nil {
			return true
		}

		// Deletion timestamp was removed (resource deletion cancelled)
		if oldDeletionTimestamp != nil && newDeletionTimestamp == nil {
			return true
		}

		// Deletion timestamp value changed (shouldn't normally happen, but handle it)
		if oldDeletionTimestamp != nil && newDeletionTimestamp != nil &&
			!oldDeletionTimestamp.Equal(newDeletionTimestamp) {
			return true
		}

		return false
	},
	CreateFunc: func(e event.CreateEvent) bool {
		return true
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return true
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return true
	},
}

// ValidateSelectors compiles every selector a CR carries (label, annotation and, for UserConfig,
// the identity extra-field selector) and returns the first error, so callers can tell "this spec
// cannot select anything" from an API failure while listing. Names are for the error message.
func ValidateSelectors(selectors ...NamedSelector) error {
	for _, s := range selectors {
		if _, err := metav1.LabelSelectorAsSelector(&s.Selector); err != nil {
			return fmt.Errorf("%s does not compile: %w", s.Name, err)
		}
	}
	return nil
}

// NamedSelector pairs a selector with the spec field it came from.
type NamedSelector struct {
	Name     string
	Selector metav1.LabelSelector
}

// SelectedObjectChangedPredicate gates the NAMESPACE watch only. A Namespace's contract with a
// NamespaceConfig is its labels and annotations: selection reads nothing else, and the shipped
// policies render from labels. Without the gate every status or resourceVersion bump on any
// namespace listed every CR, re-rendered every matching one and rewrote its status (one API write
// per event per CR). Create and Delete events still pass, as does any label or annotation change.
//
// It is deliberately NOT applied to the Group and User watches: Group.users and User.identities are
// top-level fields, not labels, and a GroupConfig template can legitimately read `.Users`
// (membership changes must re-render). Measured in review: with the gate on those watches a
// membership change was dropped.
//
// KNOWN LIMIT for namespaces, on purpose: a template that reads `.Spec` or `.Status` of the
// namespace through the render fallback is not re-rendered when only those change.
var SelectedObjectChangedPredicate = predicate.Or(predicate.LabelChangedPredicate{}, predicate.AnnotationChangedPredicate{})
