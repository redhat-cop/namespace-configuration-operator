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
- `test-unrecognized-conditionals-groupconfig.yaml` - **Test for unrecognized conditional logic detection (eq, hasPrefix, ne, etc.)**
- `test-issue-194-field-removal-namespaceconfig.yaml` - **Test for GitHub issue #194 - Field removal with value 0 in conditionals**
- `test-and-logic-groupconfig-explanation.md` - **Detailed stanza-by-stanza explanation of the AND logic YAML**
- `test-or-logic-groupconfig-explanation.md` - **Detailed stanza-by-stanza explanation of the OR logic YAML**
- `test-unrecognized-conditionals-explanation.md` - **Detailed explanation of unrecognized conditional logic detection**
- `test-issue-194-field-removal-explanation.md` - **Detailed explanation of issue #194 field removal test**
- `test-issue-194-field-removal-results.md` - **Issue #194 test results and bug confirmation**
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

### Apply Unrecognized Conditionals Test

To test unrecognized conditional logic detection:

```bash
oc apply -f examples/test-and-logic/test-unrecognized-conditionals-groupconfig.yaml
```

**Important**: Run the operator with log level 2 to see the debug messages:

```bash
./run-go.sh --log-level 2
# or
ZAP_LOG_LEVEL=2 ./run-go.sh
```

This includes five test cases:
1. **eq function**: Exact equality check (unrecognized)
2. **hasPrefix function**: Prefix check (unrecognized)
3. **ne function**: Not equal check (unrecognized)
4. **and with unrecognized functions**: AND logic with eq/hasPrefix (unrecognized)
5. **No conditionals**: Universal template (no patterns)

You should see log messages like:
- `"template contains unrecognized conditional logic, applying to all groups (relying on template rendering)"` for test cases 1-4
- `"template has no patterns, applying to all groups"` for test case 5

### Apply Issue #194 Field Removal Test

To test GitHub issue #194 (field removal with value 0):

```bash
# Create test namespace
oc create namespace test-issue-194-ns
oc label namespace test-issue-194-ns test-issue-194=true

# Apply NamespaceConfig
oc apply -f examples/test-and-logic/test-issue-194-field-removal-namespaceconfig.yaml

# Verify ResourceQuota is created with persistentvolumeclaims: "0"
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml

# Add annotation to make condition false
oc annotate namespace test-issue-194-ns allow-pvc=true

# Verify if persistentvolumeclaims field is removed (should be removed if bug is fixed)
oc get resourcequota test-issue-194-quota -n test-issue-194-ns -o yaml
```

**Expected Behavior** (if bug is fixed):
- Initially: `persistentvolumeclaims: "0"` field is present
- After annotation: `persistentvolumeclaims` field is **removed**

**Actual Behavior** (if bug exists):
- Initially: `persistentvolumeclaims: "0"` field is present
- After annotation: `persistentvolumeclaims: "0"` field **remains** ❌

See [test-issue-194-field-removal-explanation.md](test-issue-194-field-removal-explanation.md) for detailed test steps and analysis.

**Test Results**: See [test-issue-194-field-removal-results.md](test-issue-194-field-removal-results.md) for actual test execution results.

**Status**: ✅ **Bug Confirmed** - The operator does NOT remove fields with value `0` when conditionals change from true to false.

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

# Delete unrecognized conditionals test GroupConfig
oc delete groupconfig test-unrecognized-conditionals-groupconfig
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-eq
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-hasprefix
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-ne
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-and
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-unrecognized-universal

# Delete issue #194 test NamespaceConfig
oc delete namespaceconfig test-issue-194-field-removal
oc delete namespace test-issue-194-ns
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
- **[test-unrecognized-conditionals-explanation.md](test-unrecognized-conditionals-explanation.md)** - Complete explanation of unrecognized conditional logic detection
- **[test-issue-194-field-removal-explanation.md](test-issue-194-field-removal-explanation.md)** - Complete explanation of issue #194 field removal test
- **[test-issue-194-field-removal-results.md](test-issue-194-field-removal-results.md)** - Issue #194 test results and bug confirmation
- **[test-issue-194-field-removal-fix-options.md](test-issue-194-field-removal-fix-options.md)** - Fix options and implementation plan for issue #194
- **[ISSUE-194-ROOT-CAUSE-ANALYSIS.md](ISSUE-194-ROOT-CAUSE-ANALYSIS.md)** - **Root cause analysis proving bug is in dependency (for GitHub issue)**
- **[ISSUE-194-COMMAND-VERIFICATION.md](ISSUE-194-COMMAND-VERIFICATION.md)** - **All commands executed with actual outputs (verification)**
- **[ISSUE-194-GITHUB-ISSUE-TEXT.md](ISSUE-194-GITHUB-ISSUE-TEXT.md)** - **Formatted text ready to post in GitHub issue**
- **[test-or-logic-results.md](test-or-logic-results.md)** - OR logic test results from production cluster
- [Issues and Resolution](../issues-and-resolution.md) - Issue 1: Template Filtering Fix
- [Resolved Issues Tracker](../resolved-issues-tracker/resolved-issues-tracker.md) - Bug 3: AND Logic Fix
- [GitHub Issue #194](https://github.com/redhat-cop/namespace-configuration-operator/issues/194) - Field removal with value 0 in conditionals

## Issue #194 Root Cause

**Important Finding**: The bug in issue #194 is **NOT in the namespace-configuration-operator code**, but in the dependency `github.com/redhat-cop/operator-utils` v1.3.8.

See **[ISSUE-194-ROOT-CAUSE-ANALYSIS.md](ISSUE-194-ROOT-CAUSE-ANALYSIS.md)** for complete evidence, command outputs, and analysis proving the bug is in the dependency's comparison logic.

