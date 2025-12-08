# test-and-logic-groupconfig.yaml - Stanza-by-Stanza Explanation

This document provides a detailed explanation of each section in the `test-and-logic-groupconfig.yaml` file.

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
  name: test-and-logic-groupconfig
  labels:
    app.kubernetes.io/name: namespace-configuration-operator
    app.kubernetes.io/component: test
    rbac.ocp.io/scope: test
    rbac.ocp.io/kind: GroupConfig
  annotations:
    description: "Test GroupConfig to verify AND logic fix - requires both prefix and suffix patterns"
```

**Explanation:**
- **`name`**: The unique name of this GroupConfig resource in the cluster
- **`labels`**: Key-value pairs for resource organization and selection
  - `app.kubernetes.io/name`: Identifies the operator managing this resource
  - `app.kubernetes.io/component`: Categorizes this as a test component
  - `rbac.ocp.io/scope`: Indicates this is for testing purposes
  - `rbac.ocp.io/kind`: Identifies the resource type
- **`annotations`**: Human-readable metadata (not used for selection)
  - `description`: Explains the purpose of this test GroupConfig

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
  - `operator: Exists`: Requires the label to be present (value doesn't matter)

**Purpose**: Only processes Groups that have been synced from LDAP (have the `group-sync-operator.redhat-cop.io/sync-provider` label), excluding manually created groups.

**Example**: 
- ✅ `app-ocp-rbac-alpha-cluster-admin` (has sync-provider label) → Processed
- ❌ `custom-manual-group` (no sync-provider label) → Ignored

---

## **STANZA 4: Template 1 - AND Logic (Lines 17-50)**

### **4a: Template Header and Comments (Lines 18-23)**
```yaml
# Test Case 1: AND logic - requires BOTH conditions
# This template should ONLY apply to groups that:
#   1. Have suffix "-cluster-admin" AND
#   2. Contain "app-ocp-rbac" in the name
# Example matching groups: "app-ocp-rbac-alpha-cluster-admin"
# Example non-matching: "custom-cluster-admin" (missing "app-ocp-rbac")
```

**Explanation**: Documentation explaining the AND logic requirement.

---

### **4b: Go Template Conditional - AND Logic (Line 25)**
```yaml
{{- if and (hasSuffix "-cluster-admin" .Name) (contains "app-ocp-rbac" .Name) }}
```

**Explanation:**
- **`{{- if and ... }}`**: Go template syntax for AND logic - **this is the key feature being tested**
- **`(hasSuffix "-cluster-admin" .Name)`**: First condition - checks if group name ends with `-cluster-admin`
- **`(contains "app-ocp-rbac" .Name)`**: Second condition - checks if group name contains `app-ocp-rbac`
- **`.Name`**: Template variable containing the current Group's name

**Behavior**: 
- ✅ **BOTH conditions must be true** for the template to apply
- ❌ If only one condition matches, template is **rejected**

**Example Matches:**
- ✅ `app-ocp-rbac-alpha-cluster-admin` → Both true → Template applies
- ❌ `custom-cluster-admin` → Only suffix matches → Template rejected
- ❌ `app-ocp-rbac-alpha-cluster-audit` → Only contains matches → Template rejected

---

### **4c: ClusterRoleBinding Resource (Lines 26-49)**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: "{{ .Name }}-and-logic-test-crb"
```

**Explanation:**
- **`apiVersion`**: Kubernetes RBAC API version
- **`kind: ClusterRoleBinding`**: Cluster-scoped RBAC resource that grants permissions
- **`name`**: Unique name for the ClusterRoleBinding
  - `{{ .Name }}`: Template variable - replaced with the actual group name
  - Example: For group `app-ocp-rbac-alpha-cluster-admin`, creates `app-ocp-rbac-alpha-cluster-admin-and-logic-test-crb`

**Purpose**: Creates a ClusterRoleBinding that grants the `view` ClusterRole to matching groups.

---

### **4d: Labels (Lines 30-37)**
```yaml
labels:
  app.kubernetes.io/managed-by: namespace-configuration-operator
  app.kubernetes.io/version: 0.1.0
  rbac.ocp.io/policy-version: 0.1.0
  rbac.ocp.io/role-type: test-and-logic
  rbac.ocp.io/group-name: "{{ .Name }}"
  rbac.ocp.io/access-level: test
  rbac.ocp.io/config-source: test-and-logic
```

**Explanation:**
- **Standard labels**: Identify the operator and version
- **Custom labels**: Track RBAC configuration details
  - `rbac.ocp.io/config-source: test-and-logic` → **Used to find all resources created by this template**
  - `rbac.ocp.io/group-name`: The group this binding is for
  - `rbac.ocp.io/role-type`: Identifies this as an AND logic test

**Purpose**: Enables querying and filtering of resources created by this template.

**Usage Example:**
```bash
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-and-logic
```

---

### **4e: Annotations (Lines 38-41)**
```yaml
annotations:
  rbac.ocp.io/created-by: namespace-configuration-operator
  rbac.ocp.io/source-groupconfig: test-and-logic-groupconfig
  rbac.ocp.io/test-scenario: "AND logic - both conditions required"
```

**Explanation:**
- **`rbac.ocp.io/created-by`**: Identifies the operator that created this resource
- **`rbac.ocp.io/source-groupconfig`**: Links back to the GroupConfig that created it
- **`rbac.ocp.io/test-scenario`**: Documents what this resource is testing

**Purpose**: Provides traceability and documentation for debugging and auditing.

---

### **4f: Subjects (Lines 42-45)**
```yaml
subjects:
- kind: Group
  name: "{{ .Name }}"
  apiGroup: rbac.authorization.k8s.io
```

**Explanation:**
- **`subjects`**: Who receives the permissions
- **`kind: Group`**: OpenShift Group (not a User)
- **`name: "{{ .Name }}"`**: The group name (template variable)
- **`apiGroup`**: API group for the Group resource

**Purpose**: Grants permissions to all members of the specified OpenShift Group.

**Example**: For group `app-ocp-rbac-alpha-cluster-admin`, all users in that group get the `view` ClusterRole.

---

### **4g: Role Reference (Lines 46-49)**
```yaml
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: view
```

**Explanation:**
- **`roleRef`**: What permissions are being granted
- **`kind: ClusterRole`**: Cluster-scoped role (applies cluster-wide)
- **`name: view`**: The built-in Kubernetes `view` role (read-only permissions)

**Purpose**: Grants read-only access to cluster resources. Safe for testing as it doesn't allow modifications.

**Note**: The `view` ClusterRole is a standard Kubernetes role that provides read-only access to most resources.

---

### **4h: Template End (Line 50)**
```yaml
{{- end }}
```

**Explanation**: Closes the Go template `if` statement. Everything between `{{- if and ... }}` and `{{- end }}` is only rendered when both conditions are true.

---

## **STANZA 5: Template 2 - OR Logic (Lines 51-105)**

### **5a: Template Header and Comments (Lines 51-53)**
```yaml
# Test Case 2: OR logic (for comparison) - requires ANY condition
# This template should apply to groups that:
#   EITHER have suffix "-cluster-developer" OR contain "monitoring"
```

**Explanation**: Documents that this template demonstrates OR logic (any condition can match).

---

### **5b: First OR Condition (Lines 55-79)**
```yaml
{{- if hasSuffix "-cluster-developer" .Name }}
  ... ClusterRoleBinding definition ...
```

**Explanation:**
- **`{{- if hasSuffix ... }}`**: First condition - checks if group name ends with `-cluster-developer`
- If **true**: Creates a ClusterRoleBinding with the same structure as Template 1
- **No `and` keyword**: This is OR logic, not AND logic

**Behavior**: If this condition matches, the template applies immediately (no need to check other conditions).

---

### **5c: Second OR Condition (Lines 80-105)**
```yaml
{{- else if contains "monitoring" .Name }}
  ... ClusterRoleBinding definition ...
```

**Explanation:**
- **`{{- else if contains ... }}`**: Second condition - checks if group name contains "monitoring"
- **`else if`**: Only checked if the first condition was false
- If **true**: Creates a ClusterRoleBinding (same structure)

**Behavior**: 
- If first condition matches → Template applies
- If first condition fails but second matches → Template applies
- If both fail → Template does not apply

**OR Logic**: Either condition can trigger the template.

---

### **5d: Template End (Line 105)**
```yaml
{{- end }}
```

**Explanation**: Closes the Go template `if/else if` statement.

---

## **Summary Table**

| Template | Logic Type | Conditions | Behavior | Example Match | Example Non-Match |
|---------|------------|------------|----------|---------------|-------------------|
| **Template 1** | **AND** | `hasSuffix "-cluster-admin"` **AND** `contains "app-ocp-rbac"` | **Both must match** | `app-ocp-rbac-alpha-cluster-admin` | `custom-cluster-admin` |
| **Template 2** | **OR** | `hasSuffix "-cluster-developer"` **OR** `contains "monitoring"` | **Either can match** | `app-ocp-rbac-alpha-cluster-developer` or `user-workload-monitoring-admin` | `app-ocp-rbac-alpha-cluster-admin` |

---

## **Key Differences: AND vs OR Logic**

### **AND Logic (Template 1)**
```yaml
{{- if and (condition1) (condition2) }}
```
- ✅ **Both conditions must be true**
- ❌ If only one matches, template is **rejected**
- **Use case**: Strict filtering requiring multiple criteria

### **OR Logic (Template 2)**
```yaml
{{- if condition1 }}
  ...
{{- else if condition2 }}
  ...
{{- end }}
```
- ✅ **Either condition can be true**
- ✅ If first matches, second is not checked
- **Use case**: Flexible filtering with multiple acceptable patterns

---

## **Testing the Templates**

### **Verify AND Logic Results**
```bash
# Check ClusterRoleBindings created by AND logic template
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-and-logic

# Expected: Only groups matching BOTH conditions
# Example: app-ocp-rbac-alpha-cluster-admin-and-logic-test-crb
```

### **Verify OR Logic Results**
```bash
# Check ClusterRoleBindings created by OR logic template
oc get clusterrolebindings -l rbac.ocp.io/config-source=test-or-logic

# Expected: Groups matching EITHER condition
# Examples: 
#   - app-ocp-rbac-alpha-cluster-developer-or-logic-test-crb (suffix match)
#   - user-workload-monitoring-admin-or-logic-test-crb (contains match)
```

---

## **Related Documentation**

- [README.md](README.md) - Overview and usage instructions
- [test-and-logic-results.md](test-and-logic-results.md) - Test results from production cluster

