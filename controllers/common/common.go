package common

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	"github.com/scylladb/go-set/strset"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// DefaultExcludedPaths are unioned, in memory, into every template's excludedPaths when the locked
// resources are built (EffectiveExcludedPaths): the enforcer sets an excluded path when it creates
// the object and never enforces it again. `.status` is the server's; `.spec.replicas` belongs to an autoscaler once set.
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

// MetadataExcludedWarnings returns one message per template whose excludedPaths still carry `.metadata`.
// Such a template's labels and annotations are set once and never enforced; older builds wrote that
// entry into every CR, and nothing removes it but its author (spec.templates is one value to every
// writer, so no field manager can tell the author's entry from the old default; design record).
func MetadataExcludedWarnings(templates []apis.LockedResourceTemplate) []string {
	var out []string
	for i, t := range templates {
		if strset.New(t.ExcludedPaths...).Has(".metadata") {
			out = append(out, fmt.Sprintf("template %d excludes .metadata: its labels and annotations are set once and never enforced; remove .metadata from excludedPaths to enforce them", i))
		}
	}
	return out
}

// metadataExcludedWarned remembers, per process, the set of messages each CR (by UID) was last warned
// with, so a CR is warned once and again only when that set changes. client-go's event spam filter
// allows a source 25 events per OBJECT (all reasons together) and then one per five minutes; a
// warning on every reconcile would spend that budget and silence CleanupIncomplete on the very CRs
// this warning flags (review). The recorder's count was never a reliable signal past 25 either.
// One entry per UID, holding only the current set: an earlier shape keyed on UID plus message set
// kept every historical set forever (second review pass). The entry is dropped when the CR stops
// excluding .metadata and when it is deleted (ForgetMetadataExcluded), so the map is bounded by the
// CRs that exclude .metadata right now.
var metadataExcludedWarned sync.Map // map[types.UID]string

// WarnMetadataExcluded emits ONE Warning event on the CR (reason MetadataExcluded) naming every
// template that excludes .metadata, when that set differs from the last set emitted in this process:
// once on the first observation of a CR in a process, and again on every change of the set, a return
// to an earlier set included. One event per set, not one per template: a CR with 25 such templates
// would otherwise spend the whole per-object event burst by itself (third review pass). A nil
// recorder (tests without a manager) or a nil object emits nothing. Callers do not invoke it on the
// deletion path; they call ForgetMetadataExcluded there.
func WarnMetadataExcluded(recorder record.EventRecorder, object client.Object, templates []apis.LockedResourceTemplate) {
	if recorder == nil || isNilObject(object) {
		return
	}
	uid := object.GetUID()
	msgs := MetadataExcludedWarnings(templates)
	if len(msgs) == 0 {
		metadataExcludedWarned.Delete(uid)
		return
	}
	signature := strings.Join(msgs, "\x00")
	previous, loaded := metadataExcludedWarned.Swap(uid, signature)
	if loaded && previous == signature {
		return
	}
	recorder.Event(object, corev1.EventTypeWarning, "MetadataExcluded", metadataExcludedEventMessage(msgs, signature))
}

// metadataExcludedEventMessage joins the per-template messages into one event message, or, past the
// API server's 1024-byte limit on an event message, summarises them with a count and a short digest
// of the set so two different large sets stay distinguishable in the event list.
func metadataExcludedEventMessage(msgs []string, signature string) string {
	const limit = 1024
	joined := strings.Join(msgs, "; ")
	if len(joined) <= limit {
		return joined
	}
	sum := sha256.Sum256([]byte(signature))
	return fmt.Sprintf("%d templates exclude .metadata: their labels and annotations are set once and never enforced; remove .metadata from spec.templates[].excludedPaths to enforce them (set %x)", len(msgs), sum[:6])
}

// ForgetMetadataExcluded drops the CR's entry so a deleted CR leaves nothing behind in the process.
// A CR re-created under the same name has a new UID and is warned afresh regardless.
func ForgetMetadataExcluded(object client.Object) {
	if !isNilObject(object) {
		metadataExcludedWarned.Delete(object.GetUID())
	}
}

// isNilObject is true for a nil interface and for a typed nil pointer inside a non-nil interface,
// which `object == nil` misses and whose GetUID would panic.
func isNilObject(object client.Object) bool {
	if object == nil {
		return true
	}
	v := reflect.ValueOf(object)
	return v.Kind() == reflect.Pointer && v.IsNil()
}
