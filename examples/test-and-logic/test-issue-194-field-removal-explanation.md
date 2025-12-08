# Issue #194 Field Removal Test - Explanation

This document explains the test case for GitHub issue #194: **Operator does not differentiate between value 0 and missing field**.

## Problem Statement

When a field with value `0` is wrapped in a conditional template, and the condition changes from true to false, the operator should remove that field from the resource. However, the operator doesn't detect the difference and leaves the field in place.

### Example Scenario

1. **Initial State**: ResourceQuota has `persistentvolumeclaims: "0"` because condition `{{- if ne (index .Annotations "allow-pvc") "true" }}` evaluates to true (annotation doesn't exist or isn't "true")

2. **Change State**: Annotation `allow-pvc: "true"` is added to the namespace

3. **Expected Behavior**: The `persistentvolumeclaims` field should be **removed** from the ResourceQuota because the condition now evaluates to false

4. **Actual Behavior (Bug)**: The `persistentvolumeclaims: "0"` field **remains** in the ResourceQuota

5. **Root Cause**: The operator's resource comparison logic doesn't distinguish between:
   - A field with value `0` (should be removed)
   - A missing field (already removed)

## Test Configuration

### NamespaceConfig

**File**: `test-issue-194-field-removal-namespaceconfig.yaml`

The test uses a NamespaceConfig that:
- Matches namespaces with label `test-issue-194: "true"`
- Creates a ResourceQuota with a conditional field `persistentvolumeclaims: "0"`
- Condition: `{{- if ne (index .Annotations "allow-pvc") "true" }}`

### Template Structure

```yaml
spec:
  hard:
    pods: "4"
    requests.cpu: "1"
    requests.memory: 1Gi
    {{- if ne (index .Annotations "allow-pvc") "true" }}
    persistentvolumeclaims: "0"
    {{- end }}
    limits.cpu: "2"
    limits.memory: 2Gi
```

**Key Points**:
- `persistentvolumeclaims: "0"` is wrapped in a conditional
- When annotation `allow-pvc: "true"` is present, the condition is false
- The field should be removed from the rendered template
- Other fields (pods, requests.cpu, etc.) remain constant

## Test Steps

### Step 1: Create Test Namespace (Without Annotation)

```bash
# Create namespace with label but NO allow-pvc annotation
oc create namespace test-issue-194-ns
oc label namespace test-issue-194-ns test-issue-194=true
```

**Expected Result**: 
- ResourceQuota is created with `persistentvolumeclaims: "0"` field present

**Verification**:
```bash
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml
```

Should show:
```yaml
spec:
  hard:
    persistentvolumeclaims: "0"
    pods: "4"
    requests.cpu: "1"
    # ... other fields
```

### Step 2: Apply NamespaceConfig

```bash
oc apply -f examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml
```

**Expected Result**: 
- Operator reconciles and creates ResourceQuota
- ResourceQuota includes `persistentvolumeclaims: "0"` field

### Step 3: Add Annotation to Namespace

```bash
# Add annotation that makes the condition false
oc annotate namespace test-issue-194-ns allow-pvc=true
```

**Expected Result**: 
- Operator should detect the change and reconcile
- ResourceQuota should have `persistentvolumeclaims` field **removed**

**Verification**:
```bash
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml
```

**Expected (if bug is fixed)**:
```yaml
spec:
  hard:
    # persistentvolumeclaims field should be MISSING
    pods: "4"
    requests.cpu: "1"
    # ... other fields
```

**Actual (if bug exists)**:
```yaml
spec:
  hard:
    persistentvolumeclaims: "0"  # ❌ Field still present (BUG)
    pods: "4"
    requests.cpu: "1"
    # ... other fields
```

### Step 4: Remove Annotation (Reverse Test)

```bash
# Remove annotation to reverse the condition
oc annotate namespace test-issue-194-ns allow-pvc-
```

**Expected Result**: 
- Condition becomes true again
- `persistentvolumeclaims: "0"` field should be **added back**

**Verification**:
```bash
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml
```

Should show `persistentvolumeclaims: "0"` field is present again.

## Expected vs Actual Behavior

### Scenario 1: Annotation Added (Condition Becomes False)

| State | Expected | Actual (Bug) |
|-------|----------|--------------|
| **Before** | `persistentvolumeclaims: "0"` present | `persistentvolumeclaims: "0"` present |
| **After** | Field **removed** | Field **remains** ❌ |
| **Template Rendered** | Field not in template | Field not in template |
| **Resource Comparison** | Should detect difference | Doesn't detect difference |

### Scenario 2: Annotation Removed (Condition Becomes True)

| State | Expected | Actual |
|-------|----------|--------|
| **Before** | Field missing | Field missing |
| **After** | Field **added** | Field **added** ✅ |
| **Template Rendered** | Field in template | Field in template |
| **Resource Comparison** | Detects difference | Detects difference ✅ |

## Root Cause Analysis

The issue is likely in the `lockedresource` library's resource comparison logic:

1. **Template Rendering**: Works correctly - when condition is false, field is not in rendered template
2. **Resource Comparison**: Fails - doesn't detect that a field with value `0` should be removed
3. **Comparison Logic**: May treat `0` as equivalent to missing field, or may not properly compare nested fields

### Possible Causes

1. **JSON Comparison**: When comparing JSON, `0` might be treated as falsy and ignored
2. **Unstructured Comparison**: The `Unstructured` comparison might not handle field removal correctly
3. **Excluded Paths**: The field might be in an excluded path (but `spec.hard` shouldn't be excluded)
4. **Type Coercion**: String `"0"` vs integer `0` vs missing field might not be handled correctly

## Verification Commands

### Check ResourceQuota Before Annotation

```bash
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}'
# Expected: "0"
```

### Check ResourceQuota After Annotation

```bash
# Add annotation
oc annotate namespace test-issue-194-ns allow-pvc=true

# Wait for reconciliation (or trigger it)
oc get namespaceconfig test-issue-194-field-removal

# Check if field is removed
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o jsonpath='{.spec.hard.persistentvolumeclaims}'
# Expected (if fixed): "" (empty/missing)
# Actual (if bug exists): "0"
```

### Check All Fields

```bash
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml | grep -A 10 "spec:"
```

### Monitor Operator Logs

```bash
# Watch for reconciliation events
oc logs -f deployment/namespace-configuration-operator -n namespace-configuration-operator | grep -i "test-issue-194"
```

## Test Results

### If Bug Exists

- ✅ ResourceQuota is created correctly initially
- ✅ Field `persistentvolumeclaims: "0"` is present when condition is true
- ❌ Field `persistentvolumeclaims: "0"` **remains** when condition becomes false
- ✅ Field is **added back** when condition becomes true again

### If Bug is Fixed

- ✅ ResourceQuota is created correctly initially
- ✅ Field `persistentvolumeclaims: "0"` is present when condition is true
- ✅ Field `persistentvolumeclaims` is **removed** when condition becomes false
- ✅ Field is **added back** when condition becomes true again

## Related Resources

- **GitHub Issue**: [Issue #194](https://github.com/redhat-cop/namespace-configuration-operator/issues/194)
- **Test YAML**: `test-issue-194-field-removal-namespaceconfig.yaml`
- **Operator Library**: `github.com/redhat-cop/operator-utils` - `lockedresource` package

## Cleanup

To remove test resources:

```bash
# Delete NamespaceConfig
oc delete namespaceconfig test-issue-194-field-removal

# Delete test namespace (this will also delete the ResourceQuota)
oc delete namespace test-issue-194-ns
```

## Notes

- This test specifically targets the case where a field has value `0` (string `"0"` in YAML)
- The issue might also affect other "zero" values (integer `0`, boolean `false`, empty string `""`)
- The fix would need to be in the `lockedresource` library's comparison logic, not in the operator controllers
- This is a different issue from template filtering - it's about resource state comparison after templates are rendered
