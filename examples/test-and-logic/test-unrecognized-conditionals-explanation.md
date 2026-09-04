# Unrecognized Conditional Logic Test

This example demonstrates the **unrecognized conditional logic detection** feature in the GroupConfig controller.

## Overview

The GroupConfig controller now detects when templates use conditional logic that it cannot extract patterns from (like `eq`, `hasPrefix`, `ne`, etc.). When such conditionals are detected, the operator logs a specific message indicating that it's relying on template rendering to handle the logic.

### Recognized vs Unrecognized Conditionals

- **Recognized**: `hasSuffix` and `contains` - The operator can extract patterns and filter templates before processing
- **Unrecognized**: `eq`, `hasPrefix`, `ne`, `gt`, `lt`, etc. - The operator cannot extract patterns, so it applies the template to all groups and relies on the template renderer to evaluate the conditionals

## Test Cases

### Test Case 1: `eq` Function (Equality Check)

**Template**:
```yaml
{{- if eq .Name "app-ocp-rbac-alpha-cluster-admin" }}
```

**Behavior**: 
- Uses `eq` which is NOT recognized by pattern extraction
- Operator will log: `"template contains unrecognized conditional logic, applying to all groups (relying on template rendering)"`
- Template renderer will evaluate the condition and only create resources for matching groups

**Example Matching Groups**:
- ✅ `app-ocp-rbac-alpha-cluster-admin` (exact match)

**Example Non-Matching Groups**:
- ❌ `app-ocp-rbac-alpha-cluster-developer` (doesn't match exactly)
- ❌ `app-ocp-rbac-demo-cluster-admin` (different name)

### Test Case 2: `hasPrefix` Function (Prefix Check)

**Template**:
```yaml
{{- if hasPrefix "app-ocp-rbac-alpha" .Name }}
```

**Behavior**: 
- Uses `hasPrefix` which is NOT recognized by pattern extraction
- Operator will log: `"template contains unrecognized conditional logic, applying to all groups (relying on template rendering)"`
- Template renderer will evaluate the condition and only create resources for matching groups

**Example Matching Groups**:
- ✅ `app-ocp-rbac-alpha-cluster-admin` (starts with "app-ocp-rbac-alpha")
- ✅ `app-ocp-rbac-alpha-cluster-developer` (starts with "app-ocp-rbac-alpha")
- ✅ `app-ocp-rbac-alpha-ns-developer` (starts with "app-ocp-rbac-alpha")

**Example Non-Matching Groups**:
- ❌ `app-ocp-rbac-demo-cluster-admin` (starts with "app-ocp-rbac-demo")
- ❌ `app-ocp-rbac-platform-cluster-admin` (starts with "app-ocp-rbac-platform")

### Test Case 3: `ne` Function (Not Equal Check)

**Template**:
```yaml
{{- if ne .Name "app-ocp-rbac-alpha-cluster-admin" }}
```

**Behavior**: 
- Uses `ne` which is NOT recognized by pattern extraction
- Operator will log: `"template contains unrecognized conditional logic, applying to all groups (relying on template rendering)"`
- Template renderer will evaluate the condition and create resources for all groups EXCEPT the specified one

**Example Matching Groups**:
- ✅ `app-ocp-rbac-alpha-cluster-developer` (not equal to "app-ocp-rbac-alpha-cluster-admin")
- ✅ `app-ocp-rbac-demo-cluster-admin` (not equal to "app-ocp-rbac-alpha-cluster-admin")

**Example Non-Matching Groups**:
- ❌ `app-ocp-rbac-alpha-cluster-admin` (exactly matches the excluded name)

### Test Case 4: `and` with Unrecognized Functions

**Template**:
```yaml
{{- if and (eq .Name "app-ocp-rbac-demo-cluster-admin") (hasPrefix "app-ocp-rbac-demo" .Name) }}
```

**Behavior**: 
- Uses `and` with `eq` and `hasPrefix` which are NOT recognized by pattern extraction
- Operator will log: `"template contains unrecognized conditional logic, applying to all groups (relying on template rendering)"`
- Template renderer will evaluate BOTH conditions and only create resources if both match

**Example Matching Groups**:
- ✅ `app-ocp-rbac-demo-cluster-admin` (matches both: exact name AND prefix)

**Example Non-Matching Groups**:
- ❌ `app-ocp-rbac-demo-cluster-developer` (wrong suffix, doesn't match exact name)
- ❌ `app-ocp-rbac-alpha-cluster-admin` (wrong prefix)

### Test Case 5: No Conditionals (Universal Template)

**Template**:
```yaml
# No conditionals at all - just a plain template
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
...
```

**Behavior**: 
- Has NO conditionals - truly universal template
- Operator will log: `"template has no patterns, applying to all groups"`
- Template will be applied to ALL groups

**Example Matching Groups**:
- ✅ ALL groups (universal template)

## Usage

### Apply the Test GroupConfig

```bash
oc apply -f examples/test-and-logic/test-unrecognized-conditionals-groupconfig.yaml
```

### Check Operator Logs

With log level set to 2 (debug), you should see messages like:

```json
{
  "level": "info",
  "ts": "...",
  "msg": "template contains unrecognized conditional logic, applying to all groups (relying on template rendering)",
  "group": "app-ocp-rbac-alpha-cluster-admin"
}
```

For universal templates (no conditionals):

```json
{
  "level": "info",
  "ts": "...",
  "msg": "template has no patterns, applying to all groups",
  "group": "app-ocp-rbac-alpha-cluster-admin"
}
```

### Verify Results

Check ClusterRoleBindings created for each test case:

```bash
# Test Case 1: eq function
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-eq

# Test Case 2: hasPrefix function
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-hasprefix

# Test Case 3: ne function
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-ne

# Test Case 4: and with unrecognized functions
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-and

# Test Case 5: Universal template
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-universal
```

### Run Operator with Debug Logging

To see the template filtering debug messages:

```bash
# Using run-go.sh
./run-go.sh --log-level 2

# Or using environment variable
ZAP_LOG_LEVEL=2 ./run-go.sh
```

## Expected Behavior

1. **Unrecognized Conditionals**: Templates using `eq`, `hasPrefix`, `ne`, etc. will:
   - Be detected as having unrecognized conditional logic
   - Be logged with the specific message
   - Still be processed (applied to all groups initially)
   - Have their conditionals evaluated by the template renderer
   - Only create resources for groups that actually match the conditions

2. **Universal Templates**: Templates with no conditionals will:
   - Be detected as having no patterns
   - Be logged with the "no patterns" message
   - Be applied to ALL groups

## Implementation Details

The unrecognized conditional detection works by:

1. **Pattern Extraction**: Attempts to extract `hasSuffix` and `contains` patterns from template content
2. **Conditional Detection**: If no patterns are found, checks if template contains `{{- if` or `{{ if`
3. **Logging**: 
   - If conditionals found but no patterns extracted → "unrecognized conditional logic"
   - If no conditionals found → "no patterns, applying to all"
4. **Processing**: Returns `true` in both cases, allowing template renderer to handle evaluation

### Code Location

- Implementation: `controllers/groupconfig_controller.go` - `isTemplateApplicableToGroup()` function
- Tests: `controllers/unrecognized_conditionals_test.go` - `TestUnrecognizedConditionals()` function

## Cleanup

To remove test resources:

```bash
# Delete the GroupConfig
oc delete groupconfig test-unrecognized-conditionals-groupconfig

# Delete created ClusterRoleBindings
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-eq
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-hasprefix
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-ne
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-and
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-universal
```

## Related Documentation

- [README.md](README.md) - Main test documentation
- [test-and-logic-groupconfig-explanation.md](test-and-logic-groupconfig-explanation.md) - AND logic explanation
- [test-or-logic-groupconfig-explanation.md](test-or-logic-groupconfig-explanation.md) - OR logic explanation
