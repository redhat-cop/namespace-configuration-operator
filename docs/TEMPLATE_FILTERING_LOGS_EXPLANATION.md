# Template Filtering Logs Explanation

## Overview

This document explains the meaning and significance of template filtering log messages that appear at verbosity level 2 (V(2)) in the operator logs.

## Log Level

These logs appear at `Level(-2)`, which corresponds to **verbosity level 2** (V(2)) in zap logging. They are **debug-level informational logs**, not errors or warnings.

**To see these logs:**
- Set `ZAP_LOG_LEVEL=2` in the operator deployment
- Or use Kyverno policy to set log level to 2

## Understanding the Log Messages

### 1. "checking template applicability"

**Meaning**: The operator is evaluating whether a specific template should be applied to a specific group.

**When it appears**: For every combination of:
- Every group in the cluster
- Every template in the GroupConfig

**Example**:
```json
{
  "level": "Level(-2)",
  "ts": "2025-12-10T05:18:01Z",
  "logger": "controllers.GroupConfig",
  "msg": "checking template applicability",
  "group": "app-ocp-rbac-jeff-ns-admin",
  "suffixPatterns": ["-ns-admin"],
  "containsPatterns": [],
  "templatePreview": "{{- if hasSuffix \"-ns-admin\" .Name }}..."
}
```

**What it shows**:
- `group`: The group name being evaluated
- `suffixPatterns`: Patterns extracted from the template (e.g., `["-ns-admin"]`)
- `containsPatterns`: Contains patterns extracted from the template
- `templatePreview`: First 100 characters of the template content

### 2. "group does not match any template patterns"

**Meaning**: The group name does not match the patterns required by this template, so the template will **not** be applied to this group.

**When it appears**: When a group is checked against a template and:
- The group name doesn't have the required suffix (from `suffixPatterns`)
- AND the group name doesn't contain the required substring (from `containsPatterns`)

**Example**:
```json
{
  "level": "Level(-2)",
  "msg": "group does not match any template patterns",
  "group": "app-ocp-rbac-devops-cluster-admin",
  "suffixPatterns": ["-ns-admin"],
  "containsPatterns": []
}
```

**Interpretation**: 
- Group: `app-ocp-rbac-devops-cluster-admin`
- Template requires suffix: `-ns-admin`
- Group has suffix: `-cluster-admin`
- **Result**: ❌ No match - template will NOT be applied

**Is this a problem?** ❌ **No, this is expected behavior!**

Not every group should match every template. This is the **normal filtering behavior** that ensures templates are only applied to appropriate groups.

### 3. "group matches hasSuffix pattern"

**Meaning**: The group name matches the suffix pattern required by the template, so the template **will** be applied to this group.

**When it appears**: When a group is checked against a template and:
- The group name has the required suffix (from `suffixPatterns`)

**Example**:
```json
{
  "level": "Level(-2)",
  "msg": "group matches hasSuffix pattern",
  "group": "app-ocp-rbac-jeff-ns-admin",
  "pattern": "-ns-admin"
}
```

**Interpretation**:
- Group: `app-ocp-rbac-jeff-ns-admin`
- Template requires suffix: `-ns-admin`
- Group has suffix: `-ns-admin`
- **Result**: ✅ Match - template WILL be applied

## Why Do We See Multiple Checks for the Same Group?

You may notice the same group being checked multiple times. This happens because:

1. **Multiple Templates in One GroupConfig**: If a GroupConfig has multiple templates, each template is checked against each group.

   **Example**:
   - GroupConfig has 3 templates
   - Cluster has 10 groups
   - Total checks: 3 templates × 10 groups = **30 checks**

2. **Multiple GroupConfigs**: If you have multiple GroupConfig resources, each one processes all groups independently.

   **Example**:
   - 2 GroupConfigs, each with 2 templates
   - Cluster has 10 groups
   - Total checks: (2 GroupConfigs × 2 templates × 10 groups) = **40 checks**

3. **Reconciliation Triggers**: Every time a GroupConfig is reconciled (due to changes, periodic reconciliation, or group changes), all templates are re-evaluated against all groups.

## Common Scenarios

### Scenario 1: Template for Database Admins

**Template pattern**: `-database-admin`

**Groups checked**:
- ✅ `app-ocp-rbac-database-admin` → **Matches** (will get template)
- ❌ `app-ocp-rbac-platform-cluster-admin` → **No match** (won't get template)
- ❌ `app-ocp-rbac-alpha-ns-admin` → **No match** (won't get template)

**Logs you'll see**:
```
"checking template applicability" for each group
"group matches hasSuffix pattern" for database-admin group
"group does not match any template patterns" for other groups
```

**This is correct behavior!** Only database admin groups should get database admin templates.

### Scenario 2: Template for Namespace Admins

**Template pattern**: `-ns-admin`

**Groups checked**:
- ❌ `app-ocp-rbac-devops-cluster-admin` → **No match** (has `-cluster-admin`, not `-ns-admin`)
- ❌ `app-ocp-rbac-jeff-ns-developer` → **No match** (has `-ns-developer`, not `-ns-admin`)
- ✅ `app-ocp-rbac-jeff-ns-admin` → **Matches** (will get template)

**Logs you'll see**:
```
"group does not match any template patterns" for devops-cluster-admin
"group does not match any template patterns" for jeff-ns-developer
"group matches hasSuffix pattern" for jeff-ns-admin
```

**This is correct behavior!** Only namespace admin groups should get namespace admin templates.

## Performance Considerations

### Is This Efficient?

**Yes**, the filtering happens **before** template rendering:

1. **Pre-filtering**: Templates are filtered BEFORE processing, so only applicable templates are rendered
2. **Avoids unnecessary work**: Groups that don't match patterns skip template rendering entirely
3. **Logs are debug-only**: These logs only appear at V(2), so they don't impact production performance

### When to Be Concerned

You should only be concerned if:

1. **Too many "checking template applicability" logs**: This might indicate:
   - Too many groups in the cluster
   - Too many templates in GroupConfigs
   - Consider splitting GroupConfigs or using more specific selectors

2. **Unexpected "does not match" messages**: If you expect a group to match but it doesn't:
   - Check the group name spelling
   - Verify the pattern in the template (e.g., `-ns-admin` vs `-nsadmin`)
   - Check if the template uses AND logic (requires multiple conditions)

3. **Unexpected "matches" messages**: If a group matches when it shouldn't:
   - Review the template patterns
   - Check if patterns are too broad (e.g., `-admin` matches both `-ns-admin` and `-cluster-admin`)

## Best Practices

1. **Use Specific Patterns**: Prefer specific patterns like `-database-admin` over generic ones like `-admin`

2. **Monitor Logs During Development**: Use V(2) logs to verify template filtering works as expected

3. **Production Log Level**: In production, use `ZAP_LOG_LEVEL=info` (or 0) to avoid verbose debug logs

4. **Group Naming Convention**: Use consistent naming conventions to make pattern matching predictable

## Summary

| Log Message | Meaning | Is it a Problem? |
|------------|---------|------------------|
| `checking template applicability` | Operator is evaluating template for a group | ✅ Normal - informational |
| `group does not match any template patterns` | Template won't be applied to this group | ✅ Normal - expected filtering |
| `group matches hasSuffix pattern` | Template will be applied to this group | ✅ Normal - successful match |
| `group matches all AND logic patterns` | Template will be applied (AND logic) | ✅ Normal - successful match |
| `group does not match all AND logic patterns` | Template won't be applied (AND logic) | ✅ Normal - expected filtering |

**Key Takeaway**: These are **informational debug logs** showing the template filtering process. Seeing "does not match" messages is **normal and expected** - it means the filtering is working correctly to ensure templates are only applied to appropriate groups.

## Related Documentation

- [Template AND/OR Logic Testing](../examples/test-and-logic/README.md)
- [Log Level Configuration](./LOG_LEVEL_CONFIGURATION.md)
- [Resolved Issues Tracker](../resolved-issues-tracker/resolved-issues-tracker.md) - Template Filtering Implementation
