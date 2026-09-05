//go:build !integration
// +build !integration

package common

import (
	"sync"
	"testing"

	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

func cacheEntries() int {
	n := 0
	metadataExcludedWarned.Range(func(_, _ any) bool { n++; return true })
	return n
}

// The cache holds one entry per CR, the set it was last warned with; it is dropped when the CR stops
// excluding .metadata. The earlier shape kept every historical set forever.
func TestMetadataExcludedCacheRetainsOnlyCurrentSet(t *testing.T) {
	metadataExcludedWarned = sync.Map{}
	t.Cleanup(func() { metadataExcludedWarned = sync.Map{} })

	rec := record.NewFakeRecorder(8)
	cr := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cr", UID: "warning-cache-test"}}
	one := []apis.LockedResourceTemplate{{ExcludedPaths: []string{".metadata"}}}
	two := append(one, apis.LockedResourceTemplate{ExcludedPaths: []string{".metadata"}})

	WarnMetadataExcluded(rec, cr, one)
	<-rec.Events
	WarnMetadataExcluded(rec, cr, two)
	<-rec.Events
	<-rec.Events
	if got := cacheEntries(); got != 1 {
		t.Fatalf("cache entries for one CR after two different sets = %d, want 1", got)
	}

	// back to the first set: a transition, so it is emitted again (the set is what changed)
	WarnMetadataExcluded(rec, cr, one)
	select {
	case <-rec.Events:
	default:
		t.Fatal("returning to an earlier set must emit again")
	}

	// the CR stops excluding .metadata: nothing emitted, entry dropped
	WarnMetadataExcluded(rec, cr, []apis.LockedResourceTemplate{{ExcludedPaths: []string{".status"}}})
	select {
	case ev := <-rec.Events:
		t.Fatalf("a CR without .metadata must not emit, got %q", ev)
	default:
	}
	if got := cacheEntries(); got != 0 {
		t.Fatalf("cache entries after the warnings disappeared = %d, want 0", got)
	}

	// deletion drops the entry too
	WarnMetadataExcluded(rec, cr, one)
	<-rec.Events
	ForgetMetadataExcluded(cr)
	if got := cacheEntries(); got != 0 {
		t.Fatalf("cache entries after ForgetMetadataExcluded = %d, want 0", got)
	}
	ForgetMetadataExcluded(nil)
}

// A nil object with a live recorder must not panic (the exported helper is callable outside the three
// controllers, which always pass a typed CR).
func TestWarnMetadataExcludedToleratesNilObject(t *testing.T) {
	rec := record.NewFakeRecorder(1)
	WarnMetadataExcluded(rec, nil, []apis.LockedResourceTemplate{{ExcludedPaths: []string{".metadata"}}})
	select {
	case ev := <-rec.Events:
		t.Fatalf("nil object emitted event %q", ev)
	default:
	}
}
