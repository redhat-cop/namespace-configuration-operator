# test-or-logic-groupconfig.yaml - Stanza-by-Stanza Explanation

This document provides a detailed explanation of each section in the `test-or-logic-groupconfig.yaml` file, which demonstrates OR logic template filtering.

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
  name: test-or-logic-groupconfig
  labels:
    app.kubernetes.io/name: namespace-configuration-operator
    app.kubernetes.io/component: test
    rbac.ocp.io/scope: test
    rbac.ocp.io/kind: GroupConfig
  annotations:
    description: "Test GroupConfig to verify OR logic - requires ANY condition to match"
```

**Explanation:**
- **`name`**: The unique name of this GroupConfig resource (`test-or-logic-groupconfig`)
- **`labels`**: Key-value pairs for resource organization
  - `app.kubernetes.io/name`: Identifies the operator managing this resource
  - `app.kubernetes.io/component`: Categorizes this as a test component
  - `rbac.ocp.io/scope`: Indicates this is for testing purposes
  - `rbac.ocp.io/kind`: Identifies the resource type
- **`annotations`**: Human-readable metadata
  - `description`: Explains this tests OR logic (ANY condition can match)

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

## **STANZA 4: Template 1 - OR Logic with hasSuffix Patterns (Lines 17-108)**

### **4a: Template Header and Comments (Lines 18-25)**
```yaml
# Test Case 1: OR logic with hasSuffix patterns
# This template applies to groups that match ANY of these conditions:
#   - Has suffix "-cluster-developer" OR
#   - Has suffix "-cluster-audit" OR
#   - Has suffix "-ns-developer"
```

**Explanation**: Documents that this template demonstrates OR logic with multiple `hasSuffix` conditions.

---

### **4b: First OR Condition - cluster-developer (Lines 26-56)**
```yaml
{{- if hasSuffix "-cluster-developer" .Name }}
  ... ClusterRoleBinding definition ...
```

**Explanation:**
- **`{{- if hasSuffix "-cluster-developer" .Name }}`**: First condition - checks if group name ends with `-cluster-developer`
- **OR Logic**: If this condition matches, the template applies immediately
- **No `and` keyword**: This is OR logic, not AND logic

**Behavior**: 
- ✅ If group matches → Template applies, creates ClusterRoleBinding
- ❌ If group doesn't match → Check next condition (`else if`)

**Example Matches:**
- ✅ `app-ocp-rbac-alpha-cluster-developer`
- ✅ `app-ocp-rbac-finance-cluster-developer`

---

### **4c: Second OR Condition - cluster-audit (Lines 57-87)**
```yaml
{{- else if hasSuffix "-cluster-audit" .Name }}
  ... ClusterRoleBinding definition ...
```

**Explanation:**
- **`{{- else if hasSuffix "-cluster-audit" .Name }}`**: Second condition - only checked if first condition was false
- **OR Logic**: If this condition matches, template applies

**Behavior**: 
- Only evaluated if first condition failed
- If matches → Template applies

**Example Matches:**
- ✅ `app-ocp-rbac-alpha-cluster-audit`
- ✅ `app-ocp-rbac-demo-cluster-audit`

---

### **4d: Third OR Condition - ns-developer (Lines 88-108)**
```yaml
{{- else if hasSuffix "-ns-developer" .Name }}
  ... ClusterRoleBinding definition ...
```

**Explanation:**
- **`{{- else if hasSuffix "-ns-developer" .Name }}`**: Third condition - only checked if previous conditions were false
- **OR Logic**: If this condition matches, template applies

**Example Matches:**
- ✅ `app-ocp-rbac-alpha-ns-developer`
- ✅ `app-ocp-rbac-beta-ns-developer`

---

### **4e: ClusterRoleBinding Structure (Repeated in each condition)**

Each condition creates the same ClusterRoleBinding structure:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: "{{ .Name }}-or-logic-suffix-test-crb"
  labels:
    rbac.ocp.io/config-source: test-or-logic-suffix
  annotations:
    rbac.ocp.io/matched-condition: "hasSuffix -cluster-developer"  # Varies per condition
```

**Key Fields:**
- **`name`**: Same name for all conditions (only one will execute per group)
- **`rbac.ocp.io/config-source: test-or-logic-suffix`**: Label to identify resources from this test case
- **`rbac.ocp.io/matched-condition`**: Annotation showing which condition matched

**Purpose**: Creates ClusterRoleBinding granting `view` ClusterRole to matching groups.

---

### **4f: Template End (Line 108)**
```yaml
{{- end }}
```

**Explanation**: Closes the Go template `if/else if` chain.

---

## **STANZA 5: Template 2 - OR Logic with contains Patterns (Lines 109-200)**

### **5a: Template Header and Comments (Lines 110-118)**
```yaml
# Test Case 2: OR logic with contains patterns
# This template applies to groups that match ANY of these conditions:
#   - Contains "monitoring" OR
#   - Contains "platform" OR
#   - Contains "devops"
```

**Explanation**: Documents OR logic with multiple `contains` conditions.

---

### **5b: First OR Condition - monitoring (Lines 119-149)**
```yaml
{{- if contains "monitoring" .Name }}
```

**Explanation:**
- Checks if group name contains the string "monitoring"
- If true → Template applies

**Example Matches:**
- ✅ `user-workload-monitoring-admin`
- ✅ `app-ocp-rbac-monitoring-cluster-admin`

---

### **5c: Second OR Condition - platform (Lines 150-180)**
```yaml
{{- else if contains "platform" .Name }}
```

**Explanation:**
- Checks if group name contains "platform"
- Only evaluated if first condition failed

**Example Matches:**
- ✅ `app-ocp-rbac-platform-cluster-admin`
- ✅ `app-ocp-rbac-platform-ns-admin`

---

### **5d: Third OR Condition - devops (Lines 181-200)**
```yaml
{{- else if contains "devops" .Name }}
```

**Explanation:**
- Checks if group name contains "devops"
- Only evaluated if previous conditions failed

**Example Matches:**
- ✅ `app-ocp-rbac-devops-cluster-admin`
- ✅ `app-ocp-rbac-devops-ns-developer`

---

### **5e: ClusterRoleBinding Structure**

Each condition creates:
```yaml
metadata:
  name: "{{ .Name }}-or-logic-contains-test-crb"
  labels:
    rbac.ocp.io/config-source: test-or-logic-contains
  annotations:
    rbac.ocp.io/matched-condition: "contains monitoring"  # Varies per condition
```

**Purpose**: Creates ClusterRoleBinding with label `test-or-logic-contains` for easy filtering.

---

## **STANZA 6: Template 3 - OR Logic with Mixed Patterns (Lines 201-285)**

### **6a: Template Header and Comments (Lines 202-210)**
```yaml
# Test Case 3: OR logic mixing hasSuffix and contains
# This template applies to groups that match ANY of these conditions:
#   - Has suffix "-cluster-admin" OR
#   - Contains "finance" OR
#   - Contains "test"
```

**Explanation**: Documents OR logic mixing different pattern types (`hasSuffix` and `contains`).

---

### **6b: First OR Condition - hasSuffix cluster-admin (Lines 211-241)**
```yaml
{{- if hasSuffix "-cluster-admin" .Name }}
```

**Explanation:**
- Uses `hasSuffix` pattern matching
- Checks if group name ends with `-cluster-admin`

**Example Matches:**
- ✅ `app-ocp-rbac-alpha-cluster-admin`
- ✅ `app-ocp-rbac-demo-cluster-admin`

---

### **6c: Second OR Condition - contains finance (Lines 242-272)**
```yaml
{{- else if contains "finance" .Name }}
```

**Explanation:**
- Uses `contains` pattern matching
- Checks if group name contains "finance"

**Example Matches:**
- ✅ `app-ocp-rbac-finance-cluster-developer`
- ✅ `app-ocp-rbac-finance-ns-admin`

---

### **6d: Third OR Condition - contains test (Lines 273-285)**
```yaml
{{- else if contains "test" .Name }}
```

**Explanation:**
- Uses `contains` pattern matching
- Checks if group name contains "test"

**Example Matches:**
- ✅ `app-ocp-rbac-test-cluster-admin`
- ✅ `app-ocp-rbac-test-ns-developer`

---

### **6e: ClusterRoleBinding Structure**

Each condition creates:
```yaml
metadata:
  name: "{{ .Name }}-or-logic-mixed-test-crb"
  labels:
    rbac.ocp.io/config-source: test-or-logic-mixed
  annotations:
    rbac.ocp.io/matched-condition: "hasSuffix -cluster-admin"  # Varies per condition
```

**Purpose**: Demonstrates that OR logic works with mixed pattern types.

---

## **Summary Table**

| Template | Pattern Types | Conditions | Label | Behavior |
|---------|---------------|------------|-------|----------|
| **Template 1** | `hasSuffix` only | `-cluster-developer` OR `-cluster-audit` OR `-ns-developer` | `test-or-logic-suffix` | Any suffix matches |
| **Template 2** | `contains` only | `monitoring` OR `platform` OR `devops` | `test-or-logic-contains` | Any contains matches |
| **Template 3** | Mixed | `-cluster-admin` OR `finance` OR `test` | `test-or-logic-mixed` | Any pattern matches |

---

## **Key OR Logic Characteristics**

### **OR Logic Behavior**
```yaml
{{- if condition1 }}
  ... apply template ...
{{- else if condition2 }}
  ... apply template ...
{{- else if condition3 }}
  ... apply template ...
{{- end }}
```

**Characteristics:**
- ✅ **First match wins**: If condition1 matches, conditions 2 and 3 are not checked
- ✅ **Any condition can trigger**: Only one condition needs to match
- ✅ **Sequential evaluation**: Conditions are checked in order
- ✅ **Single execution**: Only one branch executes per group

### **Comparison: OR vs AND Logic**

| Aspect | OR Logic | AND Logic |
|--------|---------|-----------|
| **Syntax** | `{{- if ... }}` / `{{- else if ... }}` | `{{- if and (...) (...) }}` |
| **Requirements** | ANY condition matches | ALL conditions match |
| **Evaluation** | Sequential, stops at first match | All conditions checked |
| **Use Case** | Flexible matching, multiple acceptable patterns | Strict filtering, multiple required criteria |

---

## **Testing the Templates**

### **Verify Test Case 1 (hasSuffix patterns)**
```bash
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-suffix
```

**Expected**: ClusterRoleBindings for groups with suffix:
- `-cluster-developer` OR
- `-cluster-audit` OR
- `-ns-developer`

### **Verify Test Case 2 (contains patterns)**
```bash
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-contains
```

**Expected**: ClusterRoleBindings for groups containing:
- `monitoring` OR
- `platform` OR
- `devops`

### **Verify Test Case 3 (mixed patterns)**
```bash
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-mixed
```

**Expected**: ClusterRoleBindings for groups matching:
- Suffix `-cluster-admin` OR
- Contains `finance` OR
- Contains `test`

### **Check Matched Conditions**
```bash
# See which condition matched for each ClusterRoleBinding
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic-suffix \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.metadata.annotations.rbac\.ocp\.io/matched-condition}{"\n"}{end}'
```

---

## **Related Documentation**

- [README.md](README.md) - Overview and usage instructions
- [test-or-logic-results.md](test-or-logic-results.md) - Test results from production cluster
- [test-and-logic-groupconfig-explanation.md](test-and-logic-groupconfig-explanation.md) - AND logic explanation (for comparison)

