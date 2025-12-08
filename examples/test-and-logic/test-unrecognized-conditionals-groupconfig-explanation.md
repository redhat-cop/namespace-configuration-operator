# test-unrecognized-conditionals-groupconfig.yaml - Stanza-by-Stanza Explanation

This document provides a detailed explanation of each section in the `test-unrecognized-conditionals-groupconfig.yaml` file, which demonstrates unrecognized conditional logic detection.

---

## **STANZA 1: API Version and Kind (Lines 1-2)**
```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: GroupConfig
```

**Explanation:**
- **`apiVersion`**: Specifies the Custom Resource API version for the GroupConfig CRD
- **`kind`**: Identifies the resource type - tells Kubernetes this is a `GroupConfig` resource

**Purpose**: These fields tell Kubernetes which CRD schema to use when processing this resource.

---

## **STANZA 2: Metadata (Lines 3-11)**
```yaml
metadata:
  name: test-unrecognized-conditionals-groupconfig
  labels:
    app.kubernetes.io/name: namespace-configuration-operator
    app.kubernetes.io/component: test
    rbac.ocp.io/scope: test
    rbac.ocp.io/kind: GroupConfig
  annotations:
    description: "Test GroupConfig to verify unrecognized conditional logic detection - uses eq, hasPrefix, and other functions not recognized by pattern extraction"
```

**Explanation:**
- **`name`**: The unique name of this GroupConfig resource (`test-unrecognized-conditionals-groupconfig`)
- **`labels`**: Key-value pairs for resource organization
  - `app.kubernetes.io/name`: Identifies the operator managing this resource
  - `app.kubernetes.io/component`: Categorizes this as a test component
  - `rbac.ocp.io/scope`: Indicates this is for testing purposes
  - `rbac.ocp.io/kind`: Identifies the resource type
- **`annotations`**: Human-readable metadata
  - `description`: Explains this tests unrecognized conditional logic detection

**Purpose**: Provides identification, organization, and documentation for the resource.

---

## **STANZA 3: Label Selector (Lines 12-16)**
```yaml
spec:
  labelSelector:
    matchExpressions:
    - key: group-sync-operator.redhat-cop.io/sync-provider
      operator: Exists  # Only match synced groups
```

**Explanation:**
- **`labelSelector`**: Filters which OpenShift Groups this GroupConfig will process
- **`matchExpressions`**: Defines label matching rules
  - `key`: The label key to check for
  - `operator: Exists`: Requires the label to be present

**Purpose**: Only processes Groups that have been synced from LDAP, excluding manually created groups.

---

## **STANZA 4: Template 1 - `eq` Function (Lines 17-51)**

### **4a: Template Header and Comments (Lines 18-23)**
```yaml
# Test Case 1: Using 'eq' function (equality check)
# This template uses 'eq' which is NOT recognized by the pattern extraction regex
# The operator should detect this as "unrecognized conditional logic" and log appropriately
```

**Explanation**: Documents that this template uses `eq` function which is not recognized by pattern extraction.

---

### **4b: Conditional Logic (Line 25)**
```yaml
{{- if eq .Name "app-ocp-rbac-alpha-cluster-admin" }}
```

**Explanation:**
- **`{{- if eq .Name "app-ocp-rbac-alpha-cluster-admin" }}`**: Uses the `eq` (equals) function to check if the group name exactly matches the specified string
- **Unrecognized Function**: The `eq` function is NOT recognized by the pattern extraction regex (`hasSuffix` and `contains` are the only recognized functions)
- **Detection**: The operator will detect this as "unrecognized conditional logic" because:
  1. Pattern extraction returns empty arrays (`suffixPatterns: []`, `containsPatterns: []`)
  2. Template contains `{{- if` (conditional detected)
  3. No extractable patterns found → Logs: `"template contains unrecognized conditional logic, applying to all groups (relying on template rendering)"`

**Behavior**: 
- ✅ Template is applied to ALL groups (fail-open approach)
- ✅ Template renderer evaluates the `eq` condition
- ✅ Resource only created if group name exactly matches `"app-ocp-rbac-alpha-cluster-admin"`
- ✅ For non-matching groups, template renders to empty/null (expected)

**Example Matches:**
- ✅ `app-ocp-rbac-alpha-cluster-admin` (exact match)

**Example Non-Matches:**
- ❌ `app-ocp-rbac-alpha-cluster-developer` (different suffix)
- ❌ `app-ocp-rbac-demo-cluster-admin` (different prefix)

---

### **4c: Resource Definition (Lines 26-50)**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: "{{ .Name }}-unrecognized-eq-test-crb"
  labels:
    rbac.ocp.io/config-source: test-unrecognized-eq
  annotations:
    rbac.ocp.io/test-scenario: "Unrecognized conditional - eq function"
    rbac.ocp.io/matched-condition: "eq app-ocp-rbac-alpha-cluster-admin"
```

**Explanation:**
- **`name`**: Uses template variable `{{ .Name }}` to create unique ClusterRoleBinding names
- **`labels`**: 
  - `rbac.ocp.io/config-source: test-unrecognized-eq` - Identifies this as test case 1
- **`annotations`**:
  - `rbac.ocp.io/test-scenario` - Documents the test scenario
  - `rbac.ocp.io/matched-condition` - Shows which condition matched (for debugging)

**Purpose**: Creates a ClusterRoleBinding that binds the group to the `view` ClusterRole, with metadata for tracking.

---

### **4d: Template End (Line 51)**
```yaml
{{- end }}
```

**Explanation**: Closes the `{{- if eq ... }}` conditional block.

---

## **STANZA 5: Template 2 - `hasPrefix` Function (Lines 52-86)**

### **5a: Template Header and Comments (Lines 52-58)**
```yaml
# Test Case 2: Using 'hasPrefix' function (prefix check)
# This template uses 'hasPrefix' which is NOT recognized by the pattern extraction regex
```

**Explanation**: Documents that this template uses `hasPrefix` function which is not recognized by pattern extraction.

---

### **5b: Conditional Logic (Line 60)**
```yaml
{{- if hasPrefix "app-ocp-rbac-alpha" .Name }}
```

**Explanation:**
- **`{{- if hasPrefix "app-ocp-rbac-alpha" .Name }}`**: Uses the `hasPrefix` function to check if the group name starts with the specified string
- **Unrecognized Function**: The `hasPrefix` function is NOT recognized by the pattern extraction regex
- **Detection**: The operator will detect this as "unrecognized conditional logic" because:
  1. Pattern extraction returns empty arrays
  2. Template contains `{{- if` (conditional detected)
  3. No extractable patterns found → Logs: `"template contains unrecognized conditional logic..."`

**Behavior**: 
- ✅ Template is applied to ALL groups
- ✅ Template renderer evaluates the `hasPrefix` condition
- ✅ Resource only created if group name starts with `"app-ocp-rbac-alpha"`

**Example Matches:**
- ✅ `app-ocp-rbac-alpha-cluster-admin` (starts with "app-ocp-rbac-alpha")
- ✅ `app-ocp-rbac-alpha-cluster-developer` (starts with "app-ocp-rbac-alpha")
- ✅ `app-ocp-rbac-alpha-ns-developer` (starts with "app-ocp-rbac-alpha")

**Example Non-Matches:**
- ❌ `app-ocp-rbac-demo-cluster-admin` (starts with "app-ocp-rbac-demo")
- ❌ `app-ocp-rbac-platform-cluster-admin` (starts with "app-ocp-rbac-platform")

---

### **5c: Resource Definition (Lines 61-85)**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: "{{ .Name }}-unrecognized-hasprefix-test-crb"
  labels:
    rbac.ocp.io/config-source: test-unrecognized-hasprefix
  annotations:
    rbac.ocp.io/test-scenario: "Unrecognized conditional - hasPrefix function"
    rbac.ocp.io/matched-condition: "hasPrefix app-ocp-rbac-alpha"
```

**Explanation**: Similar to Template 1, but with different labels/annotations to identify this as test case 2.

---

## **STANZA 6: Template 3 - `ne` Function (Lines 87-119)**

### **6a: Template Header and Comments (Lines 87-91)**
```yaml
# Test Case 3: Using 'ne' function (not equal check)
# This template uses 'ne' which is NOT recognized by the pattern extraction regex
```

**Explanation**: Documents that this template uses `ne` (not equal) function which is not recognized by pattern extraction.

---

### **6b: Conditional Logic (Line 93)**
```yaml
{{- if ne .Name "app-ocp-rbac-alpha-cluster-admin" }}
```

**Explanation:**
- **`{{- if ne .Name "app-ocp-rbac-alpha-cluster-admin" }}`**: Uses the `ne` (not equal) function to check if the group name does NOT equal the specified string
- **Unrecognized Function**: The `ne` function is NOT recognized by the pattern extraction regex
- **Detection**: The operator will detect this as "unrecognized conditional logic"

**Behavior**: 
- ✅ Template is applied to ALL groups
- ✅ Template renderer evaluates the `ne` condition
- ✅ Resource created for ALL groups EXCEPT `"app-ocp-rbac-alpha-cluster-admin"`

**Example Matches:**
- ✅ `app-ocp-rbac-demo-cluster-admin` (not equal to excluded name)
- ✅ `app-ocp-rbac-beta-ns-admin` (not equal to excluded name)
- ✅ `app-ocp-rbac-platform-cluster-admin` (not equal to excluded name)

**Example Non-Matches:**
- ❌ `app-ocp-rbac-alpha-cluster-admin` (exactly matches the excluded name)

---

### **6c: Resource Definition (Lines 94-118)**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: "{{ .Name }}-unrecognized-ne-test-crb"
  labels:
    rbac.ocp.io/config-source: test-unrecognized-ne
  annotations:
    rbac.ocp.io/test-scenario: "Unrecognized conditional - ne function"
    rbac.ocp.io/matched-condition: "ne app-ocp-rbac-alpha-cluster-admin"
```

**Explanation**: Similar structure to previous templates, with labels/annotations for test case 3.

---

## **STANZA 7: Template 4 - `and` with Unrecognized Functions (Lines 120-152)**

### **7a: Template Header and Comments (Lines 120-124)**
```yaml
# Test Case 4: Using 'and' with unrecognized functions
# This template uses 'and' with 'eq' which is NOT recognized by the pattern extraction regex
```

**Explanation**: Documents that this template uses `and` with unrecognized functions (`eq` and `hasPrefix`).

---

### **7b: Conditional Logic (Line 126)**
```yaml
{{- if and (eq .Name "app-ocp-rbac-demo-cluster-admin") (hasPrefix "app-ocp-rbac-demo" .Name) }}
```

**Explanation:**
- **`{{- if and ... }}`**: Uses the `and` function to require BOTH conditions to be true
- **Condition 1**: `eq .Name "app-ocp-rbac-demo-cluster-admin"` - Exact name match
- **Condition 2**: `hasPrefix "app-ocp-rbac-demo" .Name` - Prefix match
- **Unrecognized Functions**: Both `eq` and `hasPrefix` are NOT recognized by pattern extraction
- **Detection**: The operator will detect this as "unrecognized conditional logic" because:
  1. Pattern extraction returns empty arrays
  2. Template contains `{{- if and` (conditional detected)
  3. No extractable patterns found → Logs: `"template contains unrecognized conditional logic..."`

**Behavior**: 
- ✅ Template is applied to ALL groups
- ✅ Template renderer evaluates BOTH conditions
- ✅ Resource only created if BOTH conditions are true:
  - Group name exactly equals `"app-ocp-rbac-demo-cluster-admin"` AND
  - Group name starts with `"app-ocp-rbac-demo"`

**Example Matches:**
- ✅ `app-ocp-rbac-demo-cluster-admin` (matches both: exact name AND prefix)

**Example Non-Matches:**
- ❌ `app-ocp-rbac-demo-cluster-developer` (wrong suffix, doesn't match exact name)
- ❌ `app-ocp-rbac-alpha-cluster-admin` (wrong prefix)

---

### **7c: Resource Definition (Lines 127-151)**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: "{{ .Name }}-unrecognized-and-test-crb"
  labels:
    rbac.ocp.io/config-source: test-unrecognized-and
  annotations:
    rbac.ocp.io/test-scenario: "Unrecognized conditional - and with eq/hasPrefix"
    rbac.ocp.io/matched-condition: "and (eq app-ocp-rbac-demo-cluster-admin) (hasPrefix app-ocp-rbac-demo)"
```

**Explanation**: Similar structure, with labels/annotations for test case 4.

---

## **STANZA 8: Template 5 - Universal Template (No Conditionals) (Lines 153-181)**

### **8a: Template Header and Comments (Lines 153-155)**
```yaml
# Test Case 5: Template with NO conditionals (truly universal)
# This template has NO conditionals at all - it should apply to ALL groups
# The operator should log "template has no patterns, applying to all groups"
```

**Explanation**: Documents that this template has NO conditionals - it's a truly universal template.

---

### **8b: Resource Definition (Lines 157-181)**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: "{{ .Name }}-unrecognized-universal-test-crb"
  ...
```

**Explanation:**
- **No Conditionals**: This template has NO `{{- if ... }}` statements
- **Universal Application**: Applies to ALL groups without any filtering
- **Detection**: The operator will detect this as "no patterns" because:
  1. Pattern extraction returns empty arrays
  2. Template does NOT contain `{{- if` (no conditionals detected)
  3. No conditionals found → Logs: `"template has no patterns, applying to all groups"`

**Behavior**: 
- ✅ Template is applied to ALL groups
- ✅ Resource created for EVERY group (no filtering)

**Example Matches:**
- ✅ ALL groups (universal template)

---

## **Key Differences Between Test Cases**

| Test Case | Conditional Type | Recognized? | Log Message | Behavior |
|-----------|----------------|------------|-------------|----------|
| **1** | `eq` | ❌ No | `"template contains unrecognized conditional logic..."` | Applied to all, rendered conditionally |
| **2** | `hasPrefix` | ❌ No | `"template contains unrecognized conditional logic..."` | Applied to all, rendered conditionally |
| **3** | `ne` | ❌ No | `"template contains unrecognized conditional logic..."` | Applied to all, rendered conditionally |
| **4** | `and` with `eq`/`hasPrefix` | ❌ No | `"template contains unrecognized conditional logic..."` | Applied to all, rendered conditionally |
| **5** | None (universal) | N/A | `"template has no patterns, applying to all groups"` | Applied to all, always rendered |

---

## **Operator Detection Logic**

### How Unrecognized Conditionals Are Detected

1. **Pattern Extraction**:
   ```go
   suffixPatterns := r.extractHasSuffixPatterns(templateContent)  // Returns []
   containsPatterns := r.extractContainsPatterns(templateContent) // Returns []
   ```

2. **Conditional Detection**:
   ```go
   if len(suffixPatterns) == 0 && len(containsPatterns) == 0 {
       if strings.Contains(templateContent, "{{- if") || strings.Contains(templateContent, "{{ if") {
           // Unrecognized conditional detected
           r.Log.V(2).Info("template contains unrecognized conditional logic...")
       } else {
           // No conditionals (universal template)
           r.Log.V(2).Info("template has no patterns, applying to all groups")
       }
   }
   ```

3. **Result**: 
   - Templates with unrecognized conditionals → Logged as "unrecognized conditional logic"
   - Templates with no conditionals → Logged as "no patterns"

---

## **Expected Log Output**

When running with log level 2 (`--log-level 2`), you should see:

### For Unrecognized Conditionals (Test Cases 1-4):
```
LEVEL(-2) controllers.GroupConfig checking template applicability
  {"group": "app-ocp-rbac-beta-ns-admin", "suffixPatterns": [], "containsPatterns": [], 
   "templatePreview": "{{- if eq .Name \"app-ocp-rbac-alpha-cluster-admin\" }}..."}

LEVEL(-2) controllers.GroupConfig template contains unrecognized conditional logic, applying to all groups (relying on template rendering)
  {"group": "app-ocp-rbac-beta-ns-admin"}
```

### For Universal Template (Test Case 5):
```
LEVEL(-2) controllers.GroupConfig checking template applicability
  {"group": "app-ocp-rbac-demo-cluster-audit", "suffixPatterns": [], "containsPatterns": [], 
   "templatePreview": "apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRoleBinding..."}

LEVEL(-2) controllers.GroupConfig template has no patterns, applying to all groups
  {"group": "app-ocp-rbac-demo-cluster-audit"}
```

---

## **Related Documentation**

- [test-unrecognized-conditionals-results.md](test-unrecognized-conditionals-results.md) - Test results and verification
- [test-unrecognized-conditionals-explanation.md](test-unrecognized-conditionals-explanation.md) - Overview and usage instructions
- [README.md](README.md) - Main test documentation
