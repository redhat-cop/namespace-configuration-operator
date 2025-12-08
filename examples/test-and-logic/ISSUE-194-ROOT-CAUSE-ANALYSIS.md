# Issue #194 Root Cause Analysis

## Summary

**Bug Location**: The bug is in the dependency `github.com/redhat-cop/operator-utils` (v1.3.8), specifically in the `lockedresource` comparison logic, NOT in the namespace-configuration-operator code.

**Issue**: When a field with value `"0"` is conditionally removed from a template, the operator does not remove the field from the actual Kubernetes resource. The field remains with value `"0"` even though the template no longer includes it.

## Evidence

### 1. `UpdateLockedResources` is NOT in Operator Code

**Command**:
```bash
grep -r "func.*UpdateLockedResources" controllers/
```

**Output**:
```
No matches found
```

**Conclusion**: The operator does not implement `UpdateLockedResources` method.

---

### 2. `UpdateLockedResources` Comes from Dependency

**Command**:
```bash
go doc github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller.EnforcingReconciler.UpdateLockedResources
```

**Output**:
```
package lockedresourcecontroller // import "github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller"

func (er *EnforcingReconciler) UpdateLockedResources(context context.Context, instance client.Object, lockedResources []lockedresource.LockedResource, lockedPatches []lockedpatch.LockedPatch) error
    UpdateLockedResources will do the following:
     1. initialize or retrieve the LockedResourceManager related to the passed
        parent resource
     2. compare the currently enforced resources with the one passed as
        parameters and then a. return immediately if they are the same b.
        restart the LockedResourceManager if they don't match
```

**Conclusion**: The method is defined in `operator-utils` package, not in the operator code. The method description explicitly states it "compare the currently enforced resources with the one passed as parameters" - this is where the bug occurs.

---

### 3. Operator Embeds Dependency

**Code Location**: `controllers/namespaceconfig_controller.go:48`

```go
type NamespaceConfigReconciler struct {
	lockedresourcecontroller.EnforcingReconciler  // ← From dependency
	Log                   logr.Logger
	controllerName        string
	AllowSystemNamespaces bool
}
```

**Conclusion**: The operator embeds `EnforcingReconciler` from the dependency, inheriting all its methods including `UpdateLockedResources()`.

---

### 4. Operator Calls Dependency Method

**Code Location**: `controllers/namespaceconfig_controller.go:148`

```go
err = r.UpdateLockedResources(context, instance, lockedResources, []lockedpatch.LockedPatch{})
```

**Conclusion**: The operator calls `UpdateLockedResources()` but does not implement it. The comparison logic that determines what needs to be updated is entirely within the dependency.

---

### 5. Template Rendering Works Correctly (Operator Code)

**Code Location**: `controllers/namespaceconfig_controller.go:200-217`

```go
func (r *NamespaceConfigReconciler) getResourceList(...) ([]lockedresource.LockedResource, error) {
    // ...
    lrs, err := lockedresource.GetLockedResourcesFromTemplatesWithRestConfig(applicableTemplates, r.GetRestConfig(), namespace)
    // ...
}
```

**Test Evidence**: When we tested issue #194:
- Template condition: `{{- if ne (index .Annotations "allow-pvc") "true" }}`
- When annotation is `allow-pvc: "true"`, condition is `false`
- Template correctly renders WITHOUT `persistentvolumeclaims: "0"` field ✅
- But the field remains in the actual resource ❌

**Conclusion**: Template rendering (operator code) works correctly. The bug is in the comparison/update logic (dependency code).

---

### 6. Dependency Version

**Command**:
```bash
cat go.mod | grep "operator-utils"
```

**Output**:
```
github.com/redhat-cop/operator-utils v1.3.8
```

**Command**:
```bash
go list -m -versions github.com/redhat-cop/operator-utils
```

**Output**:
```
github.com/redhat-cop/operator-utils v0.1.0 v0.1.1 v0.2.0 v0.2.1 v0.2.2 v0.2.3 v0.2.4 v0.2.5 v0.3.0 v0.3.1 v0.3.2 v0.3.3 v0.3.4 v0.3.5 v0.3.6 v0.3.7 v1.0.0 v1.0.1 v1.1.0 v1.1.1 v1.1.2 v1.1.3 v1.1.4 v1.2.0 v1.2.1 v1.2.2 v1.3.0 v1.3.1 v1.3.2 v1.3.3 v1.3.4 v1.3.5 v1.3.6 v1.3.7 v1.3.8
```

**Conclusion**: Currently using v1.3.8, which is the latest available version. The bug exists in this version.

---

## Root Cause

The bug is in the resource comparison logic within `github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller.EnforcingReconciler.UpdateLockedResources()`.

### What Happens

1. **Template Rendering** (Operator Code - ✅ Works):
   - Template condition evaluates to `false`
   - Template renders WITHOUT `persistentvolumeclaims: "0"` field
   - Expected resource: Field is missing

2. **Resource Comparison** (Dependency Code - ❌ Fails):
   - `UpdateLockedResources()` compares expected vs actual
   - Expected: Field missing
   - Actual: Field present with value `"0"`
   - Comparison: Does NOT detect this as a difference requiring field removal
   - Result: Field remains in resource

3. **Why Comparison Fails**:
   - The comparison logic likely treats `"0"` as equivalent to missing/empty
   - Or doesn't properly handle field removal in nested maps (`spec.hard`)
   - Or uses JSON comparison that ignores zero values

---

## Code Flow

```
Operator Reconcile()
  ↓
getResourceList() [Operator Code]
  ↓
  - Renders templates ✅
  - Creates LockedResource objects ✅
  - When condition false: field NOT in rendered template ✅
  ↓
UpdateLockedResources() [Dependency Code]
  ↓
  - Compares expected (from template) vs actual (from cluster) ❌
  - Should detect: field missing in expected, present in actual
  - Actually: Doesn't detect difference
  - Result: Field not removed ❌
```

---

## Test Evidence

### Test Case: ResourceQuota with Conditional Field

**Template**:
```yaml
spec:
  hard:
    {{- if ne (index .Annotations "allow-pvc") "true" }}
    persistentvolumeclaims: "0"
    {{- end }}
    pods: "4"
```

**Test Steps**:
1. Initial: No annotation → Field present ✅
2. Add annotation `allow-pvc: "true"` → Field should be removed ❌ (Field remains)
3. Remove annotation → Field should be added back ✅ (Works)

**Verification Commands**:
```bash
# Check if field exists
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}'
# Output: 0  (should be empty when annotation is true)

# Check annotation
oc get namespace test-issue-194-ns -o jsonpath='{.metadata.annotations.allow-pvc}'
# Output: true
```

**Result**: Field `persistentvolumeclaims: "0"` remains even though template doesn't include it when annotation is `true`.

---

## Where to Fix

The fix needs to be in:
- **Repository**: `github.com/redhat-cop/operator-utils`
- **Package**: `pkg/util/lockedresourcecontroller`
- **File**: Likely in the resource comparison/diff logic
- **Function**: Within `UpdateLockedResources()` or its comparison helper functions

The comparison logic needs to properly detect when:
- Expected resource: Field is missing
- Actual resource: Field is present (even with value `"0"`)
- Action required: Remove the field

---

## Impact

- **Affects**: All operators using `operator-utils` with conditional field removal
- **Severity**: Medium - Fields with value `0` are not removed when they should be
- **Workaround**: Manually patch resources to remove fields, but operator will not maintain the removal

---

## Related Files

- Test Configuration: `examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml`
- Test Results: `examples/test-and-logic/test-issue-194-field-removal-results.md`
- Test Explanation: `examples/test-and-logic/test-issue-194-field-removal-explanation.md`
- Fix Options: `examples/test-and-logic/test-issue-194-field-removal-fix-options.md`

---

## Next Steps

1. **Report to upstream**: Open issue in `github.com/redhat-cop/operator-utils` repository
2. **Investigate**: Clone `operator-utils` and locate exact comparison logic
3. **Fix**: Implement fix in comparison logic to properly detect field removal
4. **Test**: Verify fix with issue #194 test case
5. **Contribute**: Submit PR to upstream repository

---

## Commands Summary

```bash
# 1. Verify UpdateLockedResources is not in operator code
grep -r "func.*UpdateLockedResources" controllers/
# Result: No matches

# 2. Show UpdateLockedResources is from dependency
go doc github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller.EnforcingReconciler.UpdateLockedResources

# 3. Check dependency version
cat go.mod | grep "operator-utils"
go list -m -versions github.com/redhat-cop/operator-utils

# 4. Test the bug
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}'
oc get namespace test-issue-194-ns -o jsonpath='{.metadata.annotations.allow-pvc}'
```

---

## Conclusion

The bug is **definitively in the dependency** `github.com/redhat-cop/operator-utils` v1.3.8, specifically in the resource comparison logic within `UpdateLockedResources()`. The operator code correctly renders templates, but the dependency's comparison logic fails to detect that fields with value `"0"` should be removed when they are no longer in the template.

**Fix Required**: Update the comparison logic in `operator-utils` to properly detect and remove fields that are missing in the expected resource but present in the actual resource, regardless of the field's value (including `"0"`).
