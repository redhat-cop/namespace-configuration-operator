# Issue #194 Fix Options

## Problem Summary

The operator does not remove fields with value `0` when conditionals change from true to false. The root cause is in the `lockedresource` library's comparison logic from `github.com/redhat-cop/operator-utils`.

## Root Cause

**Location**: `github.com/redhat-cop/operator-utils/pkg/util/lockedresourcecontroller/lockedresource`

**Issue**: When comparing expected (from template) vs actual (from cluster):
- **Expected**: Field missing (condition is false, template doesn't render the field)
- **Actual**: Field present with value `"0"`
- **Comparison**: Doesn't detect this as a difference requiring field removal

**Why**: The comparison logic likely:
1. Treats `"0"` as equivalent to missing/empty
2. Doesn't properly handle field removal in nested maps (`spec.hard`)
3. Uses JSON comparison that ignores zero values

## Fix Options

### Option 1: Fix in Upstream Library (Recommended - Long-term)

**Approach**: Fix the comparison logic in `github.com/redhat-cop/operator-utils`

**Pros**:
- ✅ Fixes the issue for all operators using the library
- ✅ Proper solution at the source
- ✅ Benefits the entire community
- ✅ No workarounds needed

**Cons**:
- ❌ Requires external dependency update
- ❌ May take time to get merged and released
- ❌ Need to coordinate with library maintainers

**Steps**:
1. Fork/clone `github.com/redhat-cop/operator-utils`
2. Locate comparison logic in `lockedresource` package
3. Fix comparison to properly detect field removal for zero values
4. Add test cases for this scenario
5. Submit PR to upstream repository
6. Update `go.mod` to use fixed version (or fork temporarily)

**Code Location** (estimated):
- Likely in: `pkg/util/lockedresourcecontroller/lockedresource/reconcile.go` or similar
- Function: Resource comparison/diff logic

**Implementation Strategy**:
```go
// Pseudo-code for fix
func compareResources(expected, actual *unstructured.Unstructured) bool {
    // Current logic might be:
    // if expectedValue == actualValue { return true }
    
    // Fixed logic should:
    // 1. Check if field exists in expected
    // 2. Check if field exists in actual
    // 3. If expected missing but actual present (even with "0"), return false (needs update)
    // 4. Properly handle nested maps (spec.hard)
}
```

---

### Option 2: Workaround in Operator Code (Short-term)

**Approach**: Post-process resources or use custom comparison

**Pros**:
- ✅ Can be implemented immediately
- ✅ No dependency on external fixes
- ✅ Works around the issue

**Cons**:
- ❌ Workaround, not a proper fix
- ❌ Adds complexity to operator code
- ❌ May need maintenance if library changes

**Implementation Options**:

#### 2a. Post-Process LockedResources

After getting resources from templates, manually check and remove fields that should be absent:

```go
func (r *NamespaceConfigReconciler) getResourceList(...) ([]lockedresource.LockedResource, error) {
    lockedresources := []lockedresource.LockedResource{}
    for _, namespace := range namespaces {
        applicableTemplates := r.filterApplicableTemplates(instance.Spec.Templates, namespace)
        if len(applicableTemplates) > 0 {
            lrs, err := lockedresource.GetLockedResourcesFromTemplatesWithRestConfig(...)
            if err != nil {
                return []lockedresource.LockedResource{}, err
            }
            
            // Post-process: Remove fields that should be absent
            for i := range lrs {
                lrs[i] = r.removeZeroValueFields(lrs[i], applicableTemplates, namespace)
            }
            
            lockedresources = append(lockedresources, lrs...)
        }
    }
    return lockedresources, nil
}

func (r *NamespaceConfigReconciler) removeZeroValueFields(
    lr lockedresource.LockedResource,
    templates []apis.LockedResourceTemplate,
    namespace corev1.Namespace,
) lockedresource.LockedResource {
    // Parse template to find conditional fields
    // If condition is false, ensure field is removed from Unstructured
    // This is complex and error-prone
}
```

**Challenges**:
- Need to re-parse templates to understand conditionals
- Complex logic to determine which fields should be absent
- Error-prone and hard to maintain

#### 2b. Custom Reconciliation Logic

Override the reconciliation to manually patch resources:

```go
func (r *NamespaceConfigReconciler) Reconcile(...) (ctrl.Result, error) {
    // ... existing code ...
    
    // After normal reconciliation, check for fields that should be removed
    err = r.cleanupZeroValueFields(context, instance, lockedResources)
    if err != nil {
        return r.ManageError(context, instance, err)
    }
    
    // ... rest of code ...
}

func (r *NamespaceConfigReconciler) cleanupZeroValueFields(
    ctx context.Context,
    instance *redhatcopv1alpha1.NamespaceConfig,
    lockedResources []lockedresource.LockedResource,
) error {
    // For each resource, check if it has fields that should be removed
    // Compare template-rendered vs actual resource
    // Manually patch to remove fields
}
```

**Challenges**:
- Need to re-render templates to compare
- Complex logic
- May conflict with lockedresource's own reconciliation

#### 2c. Use ExcludedPaths (Not Applicable)

**Note**: `ExcludedPaths` is for fields that should be ignored during comparison (like `.metadata`, `.status`). This doesn't help with fields that should be removed.

---

### Option 3: Fork operator-utils Library (Medium-term)

**Approach**: Fork the library, fix it, and use the fork

**Pros**:
- ✅ Can implement fix immediately
- ✅ Full control over the fix
- ✅ Can contribute back to upstream later

**Cons**:
- ❌ Need to maintain fork
- ❌ May diverge from upstream
- ❌ Need to update `go.mod` to use fork

**Steps**:
1. Fork `github.com/redhat-cop/operator-utils` on GitHub
2. Clone fork locally
3. Implement fix in comparison logic
4. Update `go.mod`:
   ```go
   replace github.com/redhat-cop/operator-utils => github.com/YOUR-ORG/operator-utils v1.3.8-fixed
   ```
5. Test thoroughly
6. Submit PR to upstream
7. Once merged, switch back to upstream

---

### Option 4: Use JSON Patch Strategy

**Approach**: After reconciliation, manually patch resources to remove fields

**Pros**:
- ✅ Can be implemented in operator code
- ✅ Works around the library limitation
- ✅ Relatively straightforward

**Cons**:
- ❌ Workaround, not a proper fix
- ❌ Need to track which fields should be removed
- ❌ May cause reconciliation loops

**Implementation**:
```go
func (r *NamespaceConfigReconciler) postReconcileCleanup(
    ctx context.Context,
    namespace corev1.Namespace,
) error {
    // Get the ResourceQuota
    quota := &corev1.ResourceQuota{}
    err := r.GetClient().Get(ctx, types.NamespacedName{
        Name:      "test-issue-194-quota",
        Namespace: namespace.Name,
    }, quota)
    
    // Check if annotation makes condition false
    if namespace.Annotations["allow-pvc"] == "true" {
        // Field should be removed
        if _, exists := quota.Spec.Hard["persistentvolumeclaims"]; exists {
            // Patch to remove field
            patch := client.MergeFrom(quota.DeepCopy())
            delete(quota.Spec.Hard, "persistentvolumeclaims")
            return r.GetClient().Patch(ctx, quota, patch)
        }
    }
    return nil
}
```

**Challenges**:
- Need to know which resources/fields to check
- Hardcoded logic for specific scenarios
- Not generic solution

---

## Recommended Approach

### Short-term (Immediate)
**Option 4**: Use JSON Patch Strategy for specific known cases
- Quick to implement
- Works around the issue
- Document as a known limitation

### Medium-term (Next Release)
**Option 3**: Fork operator-utils, implement fix, contribute back
- Proper fix
- Can be used immediately
- Contribute to upstream

### Long-term (Future)
**Option 1**: Upstream fix in operator-utils
- Proper solution
- Benefits all users
- Remove workarounds once merged

## Implementation Plan

### Phase 1: Investigation (1-2 days)
1. Clone `github.com/redhat-cop/operator-utils`
2. Locate comparison logic in `lockedresource` package
3. Understand how comparison works
4. Identify exact location of bug
5. Create minimal test case to reproduce

### Phase 2: Fix Development (3-5 days)
1. Implement fix in comparison logic
2. Add comprehensive test cases
3. Test with issue #194 scenario
4. Ensure no regressions

### Phase 3: Integration (2-3 days)
1. Fork operator-utils (or use replace directive)
2. Update operator to use fixed version
3. Test with real scenarios
4. Verify fix works

### Phase 4: Contribution (1-2 weeks)
1. Submit PR to upstream
2. Address review comments
3. Get merged
4. Update operator to use upstream version

## Testing Strategy

1. **Unit Tests**: Test comparison logic with zero values
2. **Integration Tests**: Test with issue #194 scenario
3. **Regression Tests**: Ensure other comparisons still work
4. **Real-world Tests**: Test with actual ResourceQuota scenarios

## Related Issues

- [GitHub Issue #194](https://github.com/redhat-cop/namespace-configuration-operator/issues/194)
- May affect other operators using `operator-utils`
- Consider reporting to `operator-utils` repository as well

## Next Steps

1. **Decide on approach** (recommend Option 3: Fork + Fix)
2. **Investigate** comparison logic in operator-utils
3. **Implement fix** with tests
4. **Test** with issue #194 scenario
5. **Contribute** back to upstream

---

## Code Investigation Checklist

To locate the bug in operator-utils:

- [ ] Clone `github.com/redhat-cop/operator-utils`
- [ ] Find `lockedresource` package
- [ ] Locate resource comparison function
- [ ] Understand comparison algorithm
- [ ] Identify where zero-value handling occurs
- [ ] Create test case reproducing the bug
- [ ] Implement fix
- [ ] Add tests
- [ ] Verify fix works

## Questions to Answer

1. How does the comparison function work?
2. Where is zero-value handling?
3. Why doesn't it detect field removal?
4. What's the best way to fix it?
5. Will the fix break other comparisons?
