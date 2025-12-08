# Template AND Logic Test

This example demonstrates the **AND logic fix** for template filtering in the GroupConfig controller.

## Overview

The GroupConfig controller now supports **AND logic** in template conditionals, allowing you to require multiple conditions to match before applying a template.

### AND Logic vs OR Logic

- **AND Logic**: Requires ALL patterns to match (uses `{{- if and ... }}`)
- **OR Logic**: Requires ANY pattern to match (default behavior, uses `{{- if ... }}` or `{{- else if ... }}`)

## Files

- `test-and-logic-groupconfig.yaml` - Test GroupConfig demonstrating both AND and OR logic
- `test-or-logic-groupconfig.yaml` - **Dedicated OR logic test with multiple test cases**
- `test-and-logic-groupconfig-explanation.md` - **Detailed stanza-by-stanza explanation of the AND logic YAML**
- `test-or-logic-groupconfig-explanation.md` - **Detailed stanza-by-stanza explanation of the OR logic YAML**
- `test-and-logic-results.md` - AND logic test results and verification
- `test-or-logic-results.md` - OR logic test results and verification

## Test Scenarios

### Test Case 1: AND Logic (Both Conditions Required)

**Template**:
```yaml
{{- if and (hasSuffix "-cluster-admin" .Name) (contains "app-ocp-rbac" .Name) }}
```

**Behavior**: 
- Template applies ONLY to groups that match BOTH conditions:
  1. Has suffix `-cluster-admin`
  2. Contains `app-ocp-rbac` in the name

**Example Matching Groups**:
- ✅ `app-ocp-rbac-alpha-cluster-admin` (matches both)
- ✅ `app-ocp-rbac-demo-cluster-admin` (matches both)

**Example Non-Matching Groups**:
- ❌ `custom-cluster-admin` (missing "app-ocp-rbac")
- ❌ `app-ocp-rbac-alpha-cluster-audit` (wrong suffix)

### Test Case 2: OR Logic (Any Condition Matches)

**Template**:
```yaml
{{- if hasSuffix "-cluster-developer" .Name }}
{{- else if contains "monitoring" .Name }}
```

**Behavior**: 
- Template applies to groups that match EITHER condition:
  1. Has suffix `-cluster-developer` OR
  2. Contains `monitoring` in the name

**Example Matching Groups**:
- ✅ `app-ocp-rbac-alpha-cluster-developer` (matches suffix)
- ✅ `user-workload-monitoring-admin` (contains "monitoring")

## Usage

### Apply the Test GroupConfig

```bash
oc apply -f examples/test-and-logic/test-and-logic-groupconfig.yaml
```

### Verify AND Logic Results

Check ClusterRoleBindings created for groups matching BOTH conditions:

```bash
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-and-logic
```

Expected: ClusterRoleBindings only for groups with suffix `-cluster-admin` AND containing `app-ocp-rbac`.

### Verify OR Logic Results

Check ClusterRoleBindings created for groups matching ANY condition:

```bash
# From test-and-logic-groupconfig.yaml (simple OR test)
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic

# From test-or-logic-groupconfig.yaml (comprehensive OR tests)
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-suffix
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-contains
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-mixed
```

Expected: ClusterRoleBindings for groups matching ANY of the specified conditions.

### Apply Dedicated OR Logic Test

For comprehensive OR logic testing:

```bash
oc apply -f examples/test-and-logic/test-or-logic-groupconfig.yaml
```

This includes three test cases:
1. **OR with hasSuffix patterns**: `-cluster-developer` OR `-cluster-audit` OR `-ns-developer`
2. **OR with contains patterns**: `monitoring` OR `platform` OR `devops`
3. **OR with mixed patterns**: `-cluster-admin` OR `finance` OR `test`

### Check Groups

List groups that should match AND logic:

```bash
oc get groups | grep -E "app-ocp-rbac.*-cluster-admin"
```

## Test Results

See `test-and-logic-results.md` for detailed test results from a production cluster.

**Summary**:
- ✅ AND Logic: 6 ClusterRoleBindings created (all groups matched both conditions)
- ✅ OR Logic: 4 ClusterRoleBindings created (groups matched at least one condition)

## Cleanup

To remove test resources:

```bash
# Delete the GroupConfig
oc delete groupconfig test-and-logic-groupconfig

# Delete created ClusterRoleBindings
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-and-logic
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-suffix
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-contains
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-mixed

# Delete OR logic test GroupConfig
oc delete groupconfig test-or-logic-groupconfig
```

## Implementation Details

The AND logic detection works by:

1. **Pattern Detection**: Extracts `hasSuffix` and `contains` patterns from template content
2. **Logic Detection**: Checks for `{{- if and` or `{{ if and` keywords
3. **AND Evaluation**: When AND logic is detected, requires ALL patterns to match
4. **OR Fallback**: When no `and` keyword is found, uses OR logic (any match)

### Code Location

- Implementation: `controllers/groupconfig_controller.go` - `isTemplateApplicableToGroup()` function
- Tests: `controllers/groupconfig_controller_test.go` - `TestIsTemplateApplicableToGroup()` function

## Related Documentation

- **[test-and-logic-groupconfig-explanation.md](test-and-logic-groupconfig-explanation.md)** - Complete stanza-by-stanza explanation of the AND logic YAML
- **[test-or-logic-groupconfig-explanation.md](test-or-logic-groupconfig-explanation.md)** - Complete stanza-by-stanza explanation of the OR logic YAML
- **[test-or-logic-results.md](test-or-logic-results.md)** - OR logic test results from production cluster
- [Issues and Resolution](../issues-and-resolution.md) - Issue 1: Template Filtering Fix
- [Work in Progress](../work-in-progress.md) - Bug 3: AND Logic Fix

