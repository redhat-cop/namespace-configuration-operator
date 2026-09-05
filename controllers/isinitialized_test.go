//go:build !integration
// +build !integration

package controllers

import (
	"encoding/json"
	"strings"
	"testing"

	redhatcopv1alpha1 "github.com/redhat-cop/namespace-configuration-operator/api/v1alpha1"
	"github.com/redhat-cop/namespace-configuration-operator/controllers/common"
	apis "github.com/redhat-cop/operator-utils/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIsInitialized(t *testing.T) {
	r := &NamespaceConfigReconciler{controllerName: "redhatcop.redhat.io/namespaceconfig-controller"}
	fresh := func() *redhatcopv1alpha1.NamespaceConfig {
		return &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc"}, Spec: redhatcopv1alpha1.NamespaceConfigSpec{
			Templates: []apis.LockedResourceTemplate{{ObjectTemplate: "kind: Role", ExcludedPaths: []string{".author.path"}}},
		}}
	}

	nc := fresh()
	if r.IsInitialized(nc) {
		t.Fatal("a fresh CR needs its finalizer, so it is not initialized")
	}
	// the spec is the author's: no default is written into it (issue #16); only the finalizer is added
	if got := nc.Spec.Templates[0].ExcludedPaths; len(got) != 1 || got[0] != ".author.path" {
		t.Errorf("the author's excludedPaths must be left exactly as declared, got %v", got)
	}
	if len(nc.Finalizers) != 1 {
		t.Errorf("expected the finalizer, got %v", nc.Finalizers)
	}
	if !r.IsInitialized(nc) {
		t.Error("the second pass must find nothing to change")
	}

	deleting := fresh()
	now := metav1.Now()
	deleting.DeletionTimestamp = &now
	deleting.Finalizers = []string{r.controllerName}
	if !r.IsInitialized(deleting) {
		t.Error("a CR being deleted must not be rewritten")
	}
	if len(deleting.Spec.Templates[0].ExcludedPaths) != 1 {
		t.Errorf("excludedPaths must be left alone on a deleting CR, got %v", deleting.Spec.Templates[0].ExcludedPaths)
	}
}

// The finalizer write is a merge patch from the pre-mutation copy: only metadata.finalizers crosses
// the wire, never the spec (a whole-object Update wrote `annotationSelector: {}`; measured in review).
func TestFinalizerPatchTouchesOnlyMetadata(t *testing.T) {
	r := &NamespaceConfigReconciler{controllerName: "redhatcop.redhat.io/namespaceconfig-controller"}
	nc := &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc"}, Spec: redhatcopv1alpha1.NamespaceConfigSpec{
		Templates: []apis.LockedResourceTemplate{{ObjectTemplate: "kind: Role"}},
	}}
	original := nc.DeepCopy()
	if r.IsInitialized(nc) {
		t.Fatal("a fresh CR needs its finalizer")
	}
	data, err := client.MergeFrom(original).Data(nc)
	if err != nil {
		t.Fatal(err)
	}
	var patch map[string]interface{}
	if err := json.Unmarshal(data, &patch); err != nil {
		t.Fatal(err)
	}
	if _, hasSpec := patch["spec"]; hasSpec || len(patch) != 1 || patch["metadata"] == nil {
		t.Fatalf("the finalizer patch must touch metadata only, got %s", data)
	}
}

func TestMetadataExcludedWarnings(t *testing.T) {
	msgs := common.MetadataExcludedWarnings([]apis.LockedResourceTemplate{
		{ObjectTemplate: "a", ExcludedPaths: []string{".status"}},
		{ObjectTemplate: "b", ExcludedPaths: []string{".metadata", ".status"}},
		{ObjectTemplate: "c"},
	})
	if len(msgs) != 1 || !strings.Contains(msgs[0], "template 1 excludes .metadata") {
		t.Fatalf("expected one warning for template 1, got %v", msgs)
	}
	rec := record.NewFakeRecorder(4)
	common.WarnMetadataExcluded(rec, &redhatcopv1alpha1.NamespaceConfig{ObjectMeta: metav1.ObjectMeta{Name: "nc"}}, []apis.LockedResourceTemplate{{ExcludedPaths: []string{".metadata"}}})
	select {
	case ev := <-rec.Events:
		if !strings.Contains(ev, "Warning MetadataExcluded") {
			t.Fatalf("unexpected event %q", ev)
		}
	default:
		t.Fatal("expected a Warning event")
	}
	common.WarnMetadataExcluded(nil, nil, nil) // a nil recorder is tolerated (tests without a manager)
}
