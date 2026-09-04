# Issue #194 — Root Cause Summary

Problem
- When a conditional stops rendering a field (e.g., ResourceQuota.spec.hard.persistentvolumeclaims), the operator failed to remove the field if its last value was "0". The field lingered with value "0" instead of being deleted.

How this was reproduced locally
1. Applied the test NamespaceConfig: examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml (matches namespaces labeled test-issue-194=true and conditionally includes persistentvolumeclaims: "0").
2. Verified initial state (no annotation): the field is present and equals 0.
3. Added annotation allow-pvc=true on the test namespace, which makes the template condition false and should remove the field.
4. Observed that the field remained with value "0" (bug).

How the dependency was identified as the culprit
- Grep showed UpdateLockedResources is not implemented in the operator controllers (no matches in controllers/).
- go doc confirmed UpdateLockedResources is a method of operator-utils’ lockedresourcecontroller.EnforcingReconciler.
- The operator embeds EnforcingReconciler and calls UpdateLockedResources during reconciliation.
- Therefore, the comparison/patch generation that decides whether to add/remove fields lives in the dependency (operator-utils), not in this operator.

Key findings
- Template rendering in the operator was correct: when allow-pvc=true, the template no longer contained persistentvolumeclaims.
- Despite the field disappearing from the rendered template (expected), the dependency did not emit a deletion for the field that already existed in the live object.
- Net effect: the field persisted with value "0" in the cluster because the patch did not instruct Kubernetes to remove it.

Minimal test signal
- With annotation:
  - oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}' → expected: empty; observed (bug): 0.
- Without annotation:
  - The same jsonpath returns 0 as expected.

Conclusion
- The bug was not in this operator’s template/rendering path.
- The bug was in operator-utils’ comparison/patch logic: it did not produce removals for fields present in actual but missing in expected, particularly when the stale value was "0".

Key commands used to identify the right module
```bash
# 1) Prove the operator does not implement UpdateLockedResources
grep -r "func.*UpdateLockedResources" controllers/

# 2) Show UpdateLockedResources lives in operator-utils
go doc github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller.EnforcingReconciler.UpdateLockedResources

# 3) Show the operator embeds EnforcingReconciler
grep -A 5 "type NamespaceConfigReconciler struct" controllers/namespaceconfig_controller.go

# 4) Show the operator calls UpdateLockedResources
grep -B 2 -A 2 "UpdateLockedResources" controllers/namespaceconfig_controller.go

# 5) Confirm which operator-utils version is in use
grep "github.com/redhat-cop/operator-utils" go.mod

# 6) List available versions and confirm current selection
go list -m -versions github.com/redhat-cop/operator-utils
go list -m github.com/redhat-cop/operator-utils
```

Minimal reproduction commands (symptom)
```bash
# With annotation (condition false) — field should be removed but wasn’t pre-fix
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}' && echo

# Inspect YAML to see lingering field
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml | grep -A 10 "spec:" | head -15
```
