# OR Logic Test Results

## Test Date
2025-12-08

## Test Configuration
**Test GroupConfig**: `test-or-logic-groupconfig`
**Location**: `examples/test-and-logic/test-or-logic-groupconfig.yaml`

## Test Scenarios

This test includes **three comprehensive test cases** demonstrating OR logic with different pattern types:

---

### ✅ Test Case 1: OR Logic with hasSuffix Patterns

**Template Conditions**:
```yaml
{{- if hasSuffix "-cluster-developer" .Name }}
{{- else if hasSuffix "-cluster-audit" .Name }}
{{- else if hasSuffix "-ns-developer" .Name }}
```

**Expected Behavior**: 
- Template should apply to groups that match ANY of the three suffix conditions

**Test Results**:
- ✅ **12 ClusterRoleBindings created** for groups matching any suffix condition

**Groups Matched** (12 total):
- **Condition 1** (`hasSuffix "-cluster-developer"`): 
  - `app-ocp-rbac-alpha-cluster-developer`
  - `app-ocp-rbac-finance-cluster-developer`
  - `app-ocp-rbac-platform-cluster-developer`
  - `app-ocp-rbac-demo-cluster-developer`
  
- **Condition 2** (`hasSuffix "-cluster-audit"`):
  - `app-ocp-rbac-alpha-cluster-audit`
  - `app-ocp-rbac-demo-cluster-audit`
  
- **Condition 3** (`hasSuffix "-ns-developer"`):
  - `app-ocp-rbac-alpha-ns-developer`
  - `app-ocp-rbac-beta-ns-developer`
  - `app-ocp-rbac-devops-ns-developer`
  - `app-ocp-rbac-jeff-ns-developer`
  - `app-ocp-rbac-lateef-ns-developer`
  - `app-ocp-rbac-demo-ns-developer`

**ClusterRoleBindings Created**:
- `app-ocp-rbac-alpha-cluster-audit-or-logic-suffix-test-crb`
- `app-ocp-rbac-alpha-cluster-developer-or-logic-suffix-test-crb`
- `app-ocp-rbac-alpha-ns-developer-or-logic-suffix-test-crb`
- `app-ocp-rbac-beta-ns-developer-or-logic-suffix-test-crb`
- `app-ocp-rbac-demo-cluster-audit-or-logic-suffix-test-crb`
- `app-ocp-rbac-demo-cluster-developer-or-logic-suffix-test-crb`
- `app-ocp-rbac-demo-ns-developer-or-logic-suffix-test-crb`
- `app-ocp-rbac-devops-ns-developer-or-logic-suffix-test-crb`
- `app-ocp-rbac-finance-cluster-developer-or-logic-suffix-test-crb`
- `app-ocp-rbac-jeff-ns-developer-or-logic-suffix-test-crb`
- `app-ocp-rbac-lateef-ns-developer-or-logic-suffix-test-crb`
- `app-ocp-rbac-platform-cluster-developer-or-logic-suffix-test-crb`

**Verification**:
```bash
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-suffix
```

**Result**: ✅ **PASSED** - All groups matching any of the three suffix patterns received ClusterRoleBindings

---

### ✅ Test Case 2: OR Logic with contains Patterns

**Template Conditions**:
```yaml
{{- if contains "monitoring" .Name }}
{{- else if contains "platform" .Name }}
{{- else if contains "devops" .Name }}
```

**Expected Behavior**: 
- Template should apply to groups that contain ANY of the three strings

**Test Results**:
- ✅ **6 ClusterRoleBindings created** for groups matching any contains condition

**Groups Matched** (6 total):
- **Condition 1** (`contains "monitoring"`):
  - No groups matched in this test run
  
- **Condition 2** (`contains "platform"`):
  - `app-ocp-rbac-platform-cluster-admin`
  - `app-ocp-rbac-platform-cluster-developer`
  - `app-ocp-rbac-platform-ns-admin`
  - `app-ocp-rbac-platform-ns-audit`
  
- **Condition 3** (`contains "devops"`):
  - `app-ocp-rbac-devops-cluster-admin`
  - `app-ocp-rbac-devops-ns-developer`

**ClusterRoleBindings Created**:
- `app-ocp-rbac-devops-cluster-admin-or-logic-contains-test-crb` (matched: `contains devops`)
- `app-ocp-rbac-devops-ns-developer-or-logic-contains-test-crb` (matched: `contains devops`)
- `app-ocp-rbac-platform-cluster-admin-or-logic-contains-test-crb` (matched: `contains platform`)
- `app-ocp-rbac-platform-cluster-developer-or-logic-contains-test-crb` (matched: `contains platform`)
- `app-ocp-rbac-platform-ns-admin-or-logic-contains-test-crb` (matched: `contains platform`)
- `app-ocp-rbac-platform-ns-audit-or-logic-contains-test-crb` (matched: `contains platform`)

**Verification**:
```bash
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-contains
```

**Result**: ✅ **PASSED** - All groups containing any of the three strings received ClusterRoleBindings

---

### ✅ Test Case 3: OR Logic with Mixed Patterns

**Template Conditions**:
```yaml
{{- if hasSuffix "-cluster-admin" .Name }}
{{- else if contains "finance" .Name }}
{{- else if contains "test" .Name }}
```

**Expected Behavior**: 
- Template should apply to groups that match ANY condition (mixing `hasSuffix` and `contains` patterns)

**Test Results**:
- ✅ **7 ClusterRoleBindings created** for groups matching any condition

**Groups Matched** (7 total):
- **Condition 1** (`hasSuffix "-cluster-admin"`):
  - `app-ocp-rbac-alpha-cluster-admin` (matched: `hasSuffix -cluster-admin`)
  - `app-ocp-rbac-demo-cluster-admin` (matched: `hasSuffix -cluster-admin`)
  - `app-ocp-rbac-devops-cluster-admin` (matched: `hasSuffix -cluster-admin`)
  - `app-ocp-rbac-newteam-cluster-admin` (matched: `hasSuffix -cluster-admin`)
  - `app-ocp-rbac-platform-cluster-admin` (matched: `hasSuffix -cluster-admin`)
  - `app-ocp-rbac-test-cluster-admin` (matched: `hasSuffix -cluster-admin`)
  
- **Condition 2** (`contains "finance"`):
  - `app-ocp-rbac-finance-cluster-developer` (matched: `contains finance`)
  
- **Condition 3** (`contains "test"`):
  - No additional matches (note: `app-ocp-rbac-test-cluster-admin` already matched condition 1, demonstrating "first match wins" behavior)

**ClusterRoleBindings Created**:
- `app-ocp-rbac-alpha-cluster-admin-or-logic-mixed-test-crb` (matched: `hasSuffix -cluster-admin`)
- `app-ocp-rbac-demo-cluster-admin-or-logic-mixed-test-crb` (matched: `hasSuffix -cluster-admin`)
- `app-ocp-rbac-devops-cluster-admin-or-logic-mixed-test-crb` (matched: `hasSuffix -cluster-admin`)
- `app-ocp-rbac-finance-cluster-developer-or-logic-mixed-test-crb` (matched: `contains finance`)
- `app-ocp-rbac-newteam-cluster-admin-or-logic-mixed-test-crb` (matched: `hasSuffix -cluster-admin`)
- `app-ocp-rbac-platform-cluster-admin-or-logic-mixed-test-crb` (matched: `hasSuffix -cluster-admin`)
- `app-ocp-rbac-test-cluster-admin-or-logic-mixed-test-crb` (matched: `hasSuffix -cluster-admin`)

**Verification**:
```bash
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-mixed
```

**Result**: ✅ **PASSED** - Mixed pattern types work correctly with OR logic

---

## Summary Statistics

| Test Case | Pattern Types | Conditions | ClusterRoleBindings Created | Status |
|-----------|---------------|------------|----------------------------|--------|
| **Test Case 1** | `hasSuffix` only | 3 conditions | **12** | ✅ PASSED |
| **Test Case 2** | `contains` only | 3 conditions | **6** | ✅ PASSED |
| **Test Case 3** | Mixed (`hasSuffix` + `contains`) | 3 conditions | **7** | ✅ PASSED |
| **TOTAL** | - | 9 conditions | **25** | ✅ ALL PASSED |

---

## Key Observations

### ✅ OR Logic Behavior Verified

1. **First Match Wins**: 
   - When multiple conditions could match, only the first matching condition executes
   - Example: `app-ocp-rbac-test-cluster-admin` matches both condition 1 (`hasSuffix "-cluster-admin"`) and condition 3 (`contains "test"`), but only condition 1 executes

2. **Sequential Evaluation**:
   - Conditions are checked in order (`if` → `else if` → `else if`)
   - Once a match is found, remaining conditions are skipped

3. **Pattern Type Independence**:
   - OR logic works correctly with:
     - Multiple `hasSuffix` patterns
     - Multiple `contains` patterns
     - Mixed `hasSuffix` and `contains` patterns

4. **Annotation Tracking**:
   - Each ClusterRoleBinding includes `rbac.ocp.io/matched-condition` annotation
   - Shows exactly which condition matched for debugging

---

## Operator Log Analysis

### Log Messages Observed

The operator logs confirmed OR logic processing:

```
LEVEL(-2) controllers.GroupConfig group matches hasSuffix pattern
  {"group": "app-ocp-rbac-alpha-cluster-audit", "pattern": "-cluster-audit"}

LEVEL(-2) controllers.GroupConfig group matches contains pattern
  {"group": "app-ocp-rbac-platform-ns-admin", "pattern": "platform"}

LEVEL(-2) controllers.GroupConfig group matches hasSuffix pattern
  {"group": "app-ocp-rbac-devops-ns-developer", "pattern": "-ns-developer"}
```

**Key Log Patterns**:
- ✅ `"group matches hasSuffix pattern"` - Suffix matches logged
- ✅ `"group matches contains pattern"` - Contains matches logged
- ✅ Multiple groups matched different conditions (OR behavior confirmed)

---

## Test Commands

### Apply Test GroupConfig
```bash
oc apply -f examples/test-and-logic/test-or-logic-groupconfig.yaml
```

### Verify Results
```bash
# Test Case 1
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-suffix

# Test Case 2
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-contains

# Test Case 3
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-mixed
```

### Check Matched Conditions
```bash
# See which condition matched for each resource
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-suffix \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.rbac\.ocp\.io/matched-condition}{"\n"}{end}'
```

### Monitor Operator Logs
```bash
tail -f /tmp/operator-current.log | grep -E "OR logic|group matches"
```

---

## Conclusion

✅ **OR Logic Implementation Verified**: All three test cases passed successfully

✅ **Pattern Type Support**: OR logic works with:
- Multiple `hasSuffix` patterns
- Multiple `contains` patterns  
- Mixed `hasSuffix` and `contains` patterns

✅ **Behavior Confirmed**:
- First match wins (sequential evaluation)
- Any condition can trigger template application
- Annotation tracking works correctly

✅ **Production Ready**: The OR logic implementation is working correctly in a live OpenShift cluster

---

## Cleanup

To remove test resources:

```bash
# Delete the GroupConfig
oc delete groupconfig test-or-logic-groupconfig

# Delete created ClusterRoleBindings
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-suffix
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-contains
oc delete clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-mixed
```

---

## Related Documentation

- [test-or-logic-groupconfig-explanation.md](test-or-logic-groupconfig-explanation.md) - Detailed stanza-by-stanza explanation
- [README.md](README.md) - Overview and usage instructions
- [test-and-logic-results.md](test-and-logic-results.md) - AND logic test results (for comparison)

