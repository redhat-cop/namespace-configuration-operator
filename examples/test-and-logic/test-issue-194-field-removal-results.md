# Issue #194 Field Removal Test - Results

## Test Date
2025-12-08

## Test Configuration
**Test NamespaceConfig**: `test-issue-194-field-removal`
**Test Namespace**: `test-issue-194-ns`
**Location**: `examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml`
**Fix Applied**: Using fork `github.com/ephico2real2/operator-utils@fix-issue-194-field-removal-zero-value`

## Test Summary

✅ **Fix Verified**: The fix successfully removes fields with value `0` when conditionals change from true to false.

## Test Steps and Results

### Step 1: Initial State (No Annotation)

**Action**:
```bash
oc create namespace test-issue-194-ns
oc label namespace test-issue-194-ns test-issue-194=true
oc apply -f examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml
```

**Result**: ✅ **PASSED**
- NamespaceConfig created successfully
- ResourceQuota created: `test-issue-194-quota`
- Field `persistentvolumeclaims: "0"` is **present** (expected - condition is true)

**Verification**:
```bash
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}'
# Output: 0
```

**ResourceQuota Spec**:
```yaml
spec:
  hard:
    limits.cpu: "2"
    limits.memory: 2Gi
    persistentvolumeclaims: "0"  ✅ Present (expected)
    pods: "4"
    requests.cpu: "1"
    requests.memory: 1Gi
```

---

### Step 2: Add Annotation (Condition Becomes False) - WITH FIX

**Action**:
```bash
oc annotate namespace test-issue-194-ns allow-pvc=true
```

**Expected Result**: 
- Field `persistentvolumeclaims` should be **removed** from ResourceQuota

**Actual Result**: ✅ **FIX WORKS!**
- Field `persistentvolumeclaims` **removed successfully** ✅
- Operator reconciled successfully (status: `LastReconcileCycleSucceded`)
- Annotation is correctly set: `allow-pvc: "true"`

**Verification**:
```bash
oc get namespace test-issue-194-ns -o jsonpath='{.metadata.annotations.allow-pvc}'
# Output: true

oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}'
# Output: (empty) ✅ Field removed!
```

**ResourceQuota Spec** (After Annotation - WITH FIX):
```yaml
spec:
  hard:
    limits.cpu: "2"
    limits.memory: 2Gi
    # persistentvolumeclaims field is MISSING ✅ (fix works!)
    pods: "4"
    requests.cpu: "1"
    requests.memory: 1Gi
```

**Analysis**:
- Template condition: `{{- if ne (index .Annotations "allow-pvc") "true" }}`
- When annotation is `allow-pvc: "true"`, condition evaluates to `false`
- Template renders WITHOUT `persistentvolumeclaims: "0"` field
- **Fix**: Operator detects the difference and removes the field ✅

---

### Step 3: Remove Annotation (Reverse Test - Condition Becomes True) - WITH FIX

**Action**:
```bash
oc annotate namespace test-issue-194-ns allow-pvc-
```

**Expected Result**: 
- Field `persistentvolumeclaims: "0"` should be **added back**

**Actual Result**: ✅ **WORKING**
- Field `persistentvolumeclaims: "0"` is **added back** (expected)
- Operator reconciled successfully
- Annotation is removed (empty)

**Verification**:
```bash
oc get namespace test-issue-194-ns -o jsonpath='{.metadata.annotations.allow-pvc}'
# Output: (empty)

oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}'
# Output: 0  ✅ Present (expected)
```

**ResourceQuota Spec** (After Removing Annotation - WITH FIX):
```yaml
spec:
  hard:
    limits.cpu: "2"
    limits.memory: 2Gi
    persistentvolumeclaims: "0"  ✅ Present (expected)
    pods: "4"
    requests.cpu: "1"
    requests.memory: 1Gi
```

**Analysis**:
- When annotation is removed, condition evaluates to `true`
- Template renders WITH `persistentvolumeclaims: "0"` field
- Field is successfully added back ✅

---

## Test Results Summary

| Test Step | Condition | Expected Field State | Actual Field State (WITH FIX) | Result |
|-----------|-----------|---------------------|-------------------------------|--------|
| **Step 1: Initial** | `true` (no annotation) | `persistentvolumeclaims: "0"` present | `persistentvolumeclaims: "0"` present | ✅ PASSED |
| **Step 2: Add Annotation** | `false` (annotation = "true") | Field **removed** | Field **removed** ✅ | ✅ **FIX WORKS** |
| **Step 3: Remove Annotation** | `true` (no annotation) | `persistentvolumeclaims: "0"` present | `persistentvolumeclaims: "0"` present | ✅ PASSED |

## Fix Verification

✅ **Issue #194 is FIXED**: The operator now correctly removes fields with value `0` when conditionals change from true to false.

### How the Fix Works

1. **Template Rendering**: ✅ Works correctly
   - When condition is `false`, template renders without the field
   - When condition is `true`, template renders with the field

2. **Resource Comparison**: ✅ **NOW WORKS**
   - `createPatchWithNullFields()` compares expected vs actual
   - `addNullFieldsForMissing()` sets missing fields to `null` in the patch
   - Merge patch with `null` values removes fields from the resource
   - Result: Field is successfully removed ✅

3. **Field Addition**: ✅ Works correctly
   - When field is added (condition becomes true), operator detects the change
   - Field is successfully added to the resource

### Fix Implementation

**Location**: `github.com/ephico2real2/operator-utils@fix-issue-194-field-removal-zero-value`

**Changes**:
- Added `createPatchWithNullFields()` method to `LockedResourceReconciler`
- Added `addNullFieldsForMissing()` helper function
- Modified patch creation to include `null` values for missing fields
- Ensures merge patches properly remove fields

**Code Flow**:
```
Operator Reconcile()
  ↓
isEqual() detects difference
  ↓
createPatchWithNullFields() [NEW - WITH FIX]
  ↓
  - Compares expected (from template) vs actual (from cluster)
  - Calls addNullFieldsForMissing() to set missing fields to null
  - Creates merge patch with null values
  ↓
MergePatchType with null values
  ↓
  - Kubernetes removes fields set to null
  - Result: Field removed ✅
```

## Comparison: Before vs After Fix

### Before Fix (Bug)
- Annotation: `allow-pvc: "true"` → Condition: `false`
- Expected: Field missing in template
- Actual: Field present with value `"0"`
- Result: Field **remains** ❌

### After Fix
- Annotation: `allow-pvc: "true"` → Condition: `false`
- Expected: Field missing in template
- Actual: Field present with value `"0"`
- Patch: Field set to `null`
- Result: Field **removed** ✅

## Operator Configuration

**go.mod replace directive**:
```go
replace github.com/redhat-cop/operator-utils => github.com/ephico2real2/operator-utils fix-issue-194-field-removal-zero-value
```

**Build**: ✅ Successful
**Operator**: ✅ Running with fix
**Test**: ✅ Verified working

## Verification Commands

### Check Field Value
```bash
# Check if persistentvolumeclaims field exists and its value
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}'
# Expected (with fix): (empty when annotation is true)
# Actual (with fix): (empty) ✅
```

### Check Annotation
```bash
# Check namespace annotation
oc get namespace test-issue-194-ns -o jsonpath='{.metadata.annotations.allow-pvc}'
```

### Check Full ResourceQuota
```bash
# View complete ResourceQuota
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml
```

### Check NamespaceConfig Status
```bash
# View NamespaceConfig reconciliation status
oc get namespaceconfig test-issue-194-field-removal -o yaml | grep -A 10 "status:"
```

## Impact

### Fixed Scenarios

This fix now correctly handles:
1. ✅ Fields with value `0` (string `"0"` in YAML)
2. ✅ Fields in nested structures (`spec.hard`)
3. ✅ Fields removed when conditionals change from true to false
4. ✅ Fields added back when conditionals change from false to true

### Production Ready

✅ **Fix Verified**: The fix is working correctly and ready for production use.

## Related Resources

- **GitHub Issue**: [Issue #194](https://github.com/redhat-cop/namespace-configuration-operator/issues/194)
- **Fix Branch**: `github.com/ephico2real2/operator-utils@fix-issue-194-field-removal-zero-value`
- **Test Configuration**: `test-issue-194-field-removal-namespaceconfig.yaml`
- **Test Explanation**: `test-issue-194-field-removal-explanation.md`
- **Root Cause Analysis**: `ISSUE-194-ROOT-CAUSE-ANALYSIS.md`

## Cleanup

To remove test resources:

```bash
# Delete NamespaceConfig
oc delete namespaceconfig test-issue-194-field-removal

# Delete test namespace (this will also delete the ResourceQuota)
oc delete namespace test-issue-194-ns
```

---

## Conclusion

✅ **Fix Verified and Working**: The fix successfully resolves issue #194. The operator now correctly removes fields with value `0` when conditionals change from true to false, and properly adds them back when conditionals change from false to true.

**Status**: ✅ **FIXED** - Ready for upstream contribution.
