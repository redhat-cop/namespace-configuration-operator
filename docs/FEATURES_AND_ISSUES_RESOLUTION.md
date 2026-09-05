# Features and Issues Resolution - Namespace Configuration Operator

**Last Updated:** December 10, 2025  
**Status:** Comprehensive improvements and feature enhancements completed ✅

**Recent Updates:**
- Code refactoring: Extracted common reconciler helpers (December 10, 2025)
- Documentation: Added groups-and-bindings-examples.md (December 10, 2025)
- Documentation: Fixed log level configuration guidance (December 10, 2025)

> **Note**: This document tracks all resolved issues, completed features, and improvements. For detailed technical documentation, see the `docs/` directory and `resolved-issues-tracker/` directory.

## Table of Contents

1. [Core Issues Resolved](#core-issues-resolved)
2. [GitHub Issues Resolved](#github-issues-resolved)
   - [Issue #50: Provide a way to identify operator generated resources](#issue-50-provide-a-way-to-identify-operator-generated-resources)
   - [Issue #132: Status Update Conflict Blocking Subsequent Reconciles](#issue-132-status-update-conflict-blocking-subsequent-reconciles)
   - [Issue #134: Log Level Configuration](#issue-134-log-level-configuration)
   - [Issue #194: Field Removal with Value 0](#issue-194-field-removal-with-value-0)
   - [Issue #50: Provide a way to identify operator generated resources](#issue-50-provide-a-way-to-identify-operator-generated-resources)
3. [Feature Enhancements](#feature-enhancements)
   - [Code Refactoring: Common Reconciler Helpers](#code-refactoring-common-reconciler-helpers)
   - [Enhanced Template Filtering with AND/OR Logic](#enhanced-template-filtering-with-andor-logic)
   - [Unrecognized Conditional Logic Detection](#unrecognized-conditional-logic-detection)
   - [Deletion Tracking and Logging](#deletion-tracking-and-logging)
   - [Retry Success Logging](#retry-success-logging)
   - [Skipping Resource Logging](#skipping-resource-logging)
4. [Build System Improvements](#build-system-improvements)
5. [Logging Enhancements](#logging-enhancements)
6. [Documentation](#documentation)
7. [Future Enhancements](#future-enhancements)

---

## Core Issues Resolved

### Issue 1: GroupConfig "Object is Null" Template Rendering Fix

**Status:** ✅ COMPLETED

**Problem Statement:**
The GroupConfigReconciler was attempting to process templates for groups that don't match the template's conditional logic, resulting in "object is null" errors during template rendering.

**Solution:**
Implemented dynamic pattern extraction and template filtering with four new methods:
- `filterApplicableTemplates` - Pre-filters templates for each group
- `isTemplateApplicableToGroup` - Determines if template conditions match group
- `extractHasSuffixPatterns` - Extracts `hasSuffix` patterns from templates
- `extractContainsPatterns` - Extracts `contains` patterns from templates

**Files Modified:**
- `controllers/groupconfig_controller.go` - Applied dynamic filtering directly
- `controllers/groupconfig_controller_test.go` - Comprehensive unit test coverage

**See Also:** [Resolved Issues Tracker - Issue 1](../resolved-issues-tracker/resolved-issues-tracker.md)

---

### Issue 2: Fix Finalizer Domain Qualification

**Status:** ✅ COMPLETED

**Problem Statement:**
Non-domain-qualified finalizer names causing Kubernetes API warnings and violating best practices.

**Solution:**
Updated all three controllers to use canonical domain-qualified finalizers:
- `redhatcop.redhat.io/namespaceconfig-controller`
- `redhatcop.redhat.io/groupconfig-controller`
- `redhatcop.redhat.io/userconfig-controller`

**Files Modified:**
- `controllers/namespaceconfig_controller.go`
- `controllers/groupconfig_controller.go`
- `controllers/userconfig_controller.go`

**See Also:** [Resolved Issues Tracker - Issue 2](../resolved-issues-tracker/resolved-issues-tracker.md)

---

### Issue 3: Controller Reconciliation Triggering (Predicates)

**Status:** ✅ COMPLETED

**Problem Statement:**
Resources stuck in deletion were not being reconciled because deletion timestamp changes weren't triggering reconciliation.

**Solution:**
Implemented custom predicate `ResourceGenerationOrFinalizerOrDeletionTimestampChangedPredicate` that handles:
- Generation changes (spec updates)
- Finalizer changes (added/removed)
- Deletion timestamp changes (new)

**Files Modified:**
- `controllers/common/common.go` - **NEW** - Custom predicate implementation
- All three controllers updated to use new predicate

**See Also:** [Resolved Issues Tracker - Issue 3](../resolved-issues-tracker/resolved-issues-tracker.md)

---

### Issue 4: Startup Banner and Version Information Display

**Status:** ✅ COMPLETED

**Problem Statement:**
No visible indication of which version or commit was running, making debugging and deployment tracking difficult.

**Solution:**
Implemented startup banner with version, commit, and build date information:
- Version package (`internal/version/version.go`)
- Automatic version detection from git or ldflags
- Prominent ASCII art banner on startup
- Build system integration (Makefile, PodmanMakefile, Dockerfile)

**Files Modified:**
- `internal/version/version.go` - **NEW** - Version management package
- `main.go` - Added startup banner call
- `Makefile` - Automatic version injection
- `PodmanMakefile` - Automatic version injection
- `Dockerfile` - Build args for version info

**See Also:** 
- [Resolved Issues Tracker - Issue 4](../resolved-issues-tracker/resolved-issues-tracker.md)
- [DOCKERFILE_ENHANCEMENTS.md](./DOCKERFILE_ENHANCEMENTS.md)
- [MAKEFILE_VERSION_INJECTION.md](./MAKEFILE_VERSION_INJECTION.md)

---

## GitHub Issues Resolved

### Issue #132: Status Update Conflict Blocking Subsequent Reconciles

**GitHub Issue:** https://github.com/redhat-cop/namespace-configuration-operator/issues/132  
**Status:** ✅ RESOLVED

**Problem Statement:**
When a status update failed on a CR due to optimistic concurrency conflicts (e.g., "the object has been modified; please apply your changes to the latest version and try again"), all following enqueued namespaceconfigs were not processed until the next reconcile event. This caused delays in processing multiple namespaceconfigs and blocked the reconciliation queue.

**Root Cause:**
The `ManageSuccess` function was called directly without retry logic. When an optimistic concurrency conflict occurred (resourceVersion mismatch), the reconcile would fail immediately, causing:
1. The current reconcile to fail
2. Subsequent reconciles in the queue to be blocked
3. No automatic retry with updated resourceVersion

**Solution:**
Implemented `ManageSuccessWithRetry` function in `controllers/common/reconciler_helpers.go` that:
1. **Automatic Conflict Detection**: Detects conflict errors using `errors.IsConflict(err)`
2. **Re-fetch Before Retry**: Re-fetches the instance before each retry to get the latest `resourceVersion`
3. **Exponential Backoff**: Retries up to 5 times with exponential backoff delays (50ms, 100ms, 200ms, 400ms, 800ms)
4. **Applied to All Controllers**: GroupConfig, NamespaceConfig, and UserConfig all use the retry mechanism

**Implementation Details:**
- Created centralized retry logic in `controllers/common/reconciler_helpers.go`
- Uses Go generics to work with any controller type
- Re-fetches instance before each retry to ensure latest resourceVersion
- V(1) level logging for retry attempts and success after retry
- Handles resource deletion gracefully (returns success if resource not found)

**Files Modified:**
- `controllers/common/reconciler_helpers.go` - **NEW** - `ManageSuccessWithRetry` function
- `controllers/groupconfig_controller.go` - Uses `ManageSuccessWithRetry`
- `controllers/namespaceconfig_controller.go` - Uses `ManageSuccessWithRetry`
- `controllers/userconfig_controller.go` - Uses `ManageSuccessWithRetry`

**Benefits:**
- ✅ **Prevents Queue Blocking**: Most conflicts are resolved automatically without failing the reconcile
- ✅ **Automatic Recovery**: No manual intervention needed for transient conflicts
- ✅ **Better Observability**: V(1) logs show retry attempts for debugging
- ✅ **Consistent Behavior**: All three controllers use the same retry logic
- ✅ **Reduced False Positives**: Fewer errors in monitoring systems

**Example Log Output:**
```json
{"level":"debug","ts":"2025-12-10T20:54:01Z","logger":"controllers.NamespaceConfig","msg":"retrying ManageSuccess due to conflict","attempt":2,"maxRetries":5,"delay":"100ms"}

{"level":"debug","ts":"2025-12-10T20:54:01Z","logger":"controllers.NamespaceConfig","msg":"ManageSuccess succeeded after retry","attempt":2,"namespaceconfig":"default-resourcequota"}
```

**See Also:** 
- [Resolved Issues Tracker - Issue #132](../resolved-issues-tracker/resolved-issues-tracker.md)
- [Code Refactoring: Common Reconciler Helpers](#code-refactoring-common-reconciler-helpers)
- [Issue #50 - Resource Identification](#issue-50-provide-a-way-to-identify-operator-generated-resources)

---

### Issue #134: Log Level Configuration

**GitHub Issue:** https://github.com/redhat-cop/namespace-configuration-operator/issues/134  
**Status:** ✅ RESOLVED

**Problem Statement:**
Operator creating lots of Info-level logs sent to ELK (hosted in AWS) via OpenShift LogForwarder. Users needed a way to set log level to "error" to reduce log volume.

**Solution:**
1. **Environment Variable Support**: `ZAP_LOG_LEVEL` and `ZAP_DEVEL` support in `main.go`
2. **Two Configuration Methods for OLM-managed deployments:**
   - **Update Subscription** (OLM-native, recommended) - Add environment variables to `Subscription.spec.config.env`
   - **Use Kyverno Policy** (Alternative) - ClusterPolicy injects environment variables into Deployment
3. **Enhanced Logging Features:**
   - V(1) level logging for skipped resources (groups/namespaces/users)
   - V(2) level logging for template filtering details
   - Info-level deletion tracking logs
   - V(1) level retry success logs
   - Structured JSON logging format

**Files Modified:**
- `main.go` - Environment variable parsing
- `controllers/groupconfig_controller.go` - Enhanced logging
- `controllers/namespaceconfig_controller.go` - Enhanced logging
- `controllers/userconfig_controller.go` - Enhanced logging
- `kyverno-policies/operator-log-level-config.yaml` - **NEW** - Kyverno policy

**Documentation:**
- [ISSUE-134-ROOT-CAUSE-SUMMARY.md](../examples/test-and-logic/ISSUE-134-ROOT-CAUSE-SUMMARY.md)
- [ISSUE-134-VERIFICATION-GUIDE.md](../examples/test-and-logic/ISSUE-134-VERIFICATION-GUIDE.md)
- [ISSUE-134-FIX-IMPLEMENTATION.md](../examples/test-and-logic/ISSUE-134-FIX-IMPLEMENTATION.md)
- [LOG_LEVEL_CONFIGURATION.md](./LOG_LEVEL_CONFIGURATION.md)

**See Also:** 
- [Resolved Issues Tracker - Issue #134](../resolved-issues-tracker/resolved-issues-tracker.md)
- [Issue #50 - Resource Identification](#issue-50-provide-a-way-to-identify-operator-generated-resources)

---

### Issue #194: Field Removal with Value 0

**GitHub Issue:** https://github.com/redhat-cop/namespace-configuration-operator/issues/194  
**Status:** ✅ ROOT CAUSE IDENTIFIED

**Problem Statement:**
Fields with value "0" not being removed when template conditionals change from true to false.

**Root Cause:**
Bug identified in `operator-utils` dependency (not in this operator). The issue is in `UpdateLockedResources` method of `lockedresourcecontroller.EnforcingReconciler` - comparison/patch logic doesn't produce removals for fields present in actual but missing in expected when value is "0".

**Workaround:**
Using forked operator-utils with fix: `github.com/ephico2real2/operator-utils@fix-issue-194-field-removal-zero-value`

**Documentation:**
- [ISSUE-194-ROOT-CAUSE-SUMMARY.md](../examples/test-and-logic/ISSUE-194-ROOT-CAUSE-SUMMARY.md)
- [ISSUE-194-VERIFICATION-GUIDE.md](../examples/test-and-logic/ISSUE-194-VERIFICATION-GUIDE.md)
- [ISSUE-194-FIX-IMPLEMENTATION.md](../examples/test-and-logic/ISSUE-194-FIX-IMPLEMENTATION.md)

**See Also:** 
- [Resolved Issues Tracker - Issue #194](../resolved-issues-tracker/resolved-issues-tracker.md)
- [Issue #50 - Resource Identification](#issue-50-provide-a-way-to-identify-operator-generated-resources)

---

### Issue #50: Provide a way to identify operator generated resources

**GitHub Issue:** https://github.com/redhat-cop/namespace-configuration-operator/issues/50  
**Status:** ✅ FIXED

**Problem Statement:**
It could be helpful to identify the resources created by the controller. Currently some teams in our clusters are creating their own network policies and they may get confused with the new NetworkPolicies we are injecting into their namespaces. They don't have an easy way to identify how such resources are created.

The common method for such case is to place an ownerReferences to the generated object with the triggering resource's reference (e.g. NamespaceConfig). But this will likely impact the implementation of the NamespaceConfig resources' deletion since Kubernetes itself will also try to delete the owned objects once the owner resource (NamespaceConfig) is removed.

Other options could be adding an annotation/label.

**Solution:**
The operator supports identifying operator-generated resources through **manual specification of labels and annotations in templates**. While the operator doesn't automatically inject identifying metadata, users can add labels and annotations to their templates, which are then applied to all created resources.

**Key Features:**
1. **Manual Metadata Specification**: Users add identifying labels/annotations to templates
2. **Automatic Cleanup**: When namespace labels are removed, operator automatically deletes resources for that namespace
3. **Production-Ready**: This approach is sustainable for production environments - no need to delete entire CRs to remove resources from specific namespaces

**Recommended Labels and Annotations:**

**Labels:**
- `app.kubernetes.io/managed-by: namespace-configuration-operator` - Standard Kubernetes label for identifying managed resources
- `rbac.ocp.io/role-type: <type>` - Custom label for role type (e.g., `cluster-admin`, `ns-developer`)
- `rbac.ocp.io/config-source: <source>` - Custom label identifying the configuration source
- `rbac.ocp.io/group-name: <group-name>` - Custom label for group name (for GroupConfig resources)
- `rbac.ocp.io/mnemonic: <mnemonic>` - Custom label for mnemonic (for NamespaceConfig resources)
- `rbac.ocp.io/environment: <env>` - Custom label for environment (for NamespaceConfig resources)

**Annotations:**
- `rbac.ocp.io/created-by: namespace-configuration-operator` - Identifies the operator that created the resource
- `rbac.ocp.io/source-groupconfig: <groupconfig-name>` - References the GroupConfig that created the resource
- `rbac.ocp.io/source-namespaceconfig: <namespaceconfig-name>` - References the NamespaceConfig that created the resource
- `rbac.ocp.io/source-namespace: <namespace-name>` - References the namespace (for NamespaceConfig resources)

**Verification Test Results:**

**Test 1: Metadata Verification on Created Resources**

**Step 1: Check deployed CRs:**
```bash
oc get groupconfigs -A
```
**Output:**
```
NAME                                                  AGE
cluster-admin-groupconfig-rbac                        18h
cluster-audit-groupconfig-rbac                        123m
cluster-developer-groupconfig-rbac                    2d20h
user-workload-monitoring-admin-groupconfig-rbac       119m
user-workload-monitoring-developer-groupconfig-rbac   119m
```

```bash
oc get namespaceconfigs -A
```
**Output:**
```
NAME                           AGE
nonprod-namespaceconfig-rbac   122m
prod-namespaceconfig-rbac      2d20h
```

**Step 2: Verify metadata on ClusterRoleBindings:**
```bash
oc get clusterrolebindings -l app.kubernetes.io/managed-by=namespace-configuration-operator --show-labels | head -5
```
**Output:**
```
NAME                                          ROLE                AGE     LABELS
app-ocp-rbac-alpha-cluster-admin-crb          ClusterRole/admin   18h     app.kubernetes.io/managed-by=namespace-configuration-operator,app.kubernetes.io/version=0.1.0,rbac.ocp.io/access-level=admin-cluster-wide,rbac.ocp.io/config-source=cluster-rbac,rbac.ocp.io/group-name=app-ocp-rbac-alpha-cluster-admin,rbac.ocp.io/policy-version=0.1.0,rbac.ocp.io/role-type=cluster-admin
app-ocp-rbac-alpha-cluster-audit-crb          ClusterRole/view    123m    app.kubernetes.io/managed-by=namespace-configuration-operator,app.kubernetes.io/version=0.1.0,rbac.ocp.io/access-level=view-cluster-wide,rbac.ocp.io/config-source=cluster-rbac,rbac.ocp.io/group-name=app-ocp-rbac-alpha-cluster-audit,rbac.ocp.io/policy-version=0.1.0,rbac.ocp.io/role-type=cluster-audit
app-ocp-rbac-alpha-cluster-developer-crb      ClusterRole/view    2d20h   app.kubernetes.io/managed-by=namespace-configuration-operator,app.kubernetes.io/version=0.1.0,rbac.ocp.io/access-level=view-cluster-wide,rbac.ocp.io/config-source=cluster-rbac,rbac.ocp.io/group-name=app-ocp-rbac-alpha-cluster-developer,rbac.ocp.io/policy-version=0.1.0,rbac.ocp.io/role-type=cluster-developer
```

```bash
oc get clusterrolebindings -l app.kubernetes.io/managed-by=namespace-configuration-operator -o json | jq -r '.items[0] | {name: .metadata.name, labels: .metadata.labels, annotations: .metadata.annotations}'
```
**Output:**
```json
{
  "name": "app-ocp-rbac-alpha-cluster-admin-crb",
  "labels": {
    "app.kubernetes.io/managed-by": "namespace-configuration-operator",
    "app.kubernetes.io/version": "0.1.0",
    "rbac.ocp.io/access-level": "admin-cluster-wide",
    "rbac.ocp.io/config-source": "cluster-rbac",
    "rbac.ocp.io/group-name": "app-ocp-rbac-alpha-cluster-admin",
    "rbac.ocp.io/policy-version": "0.1.0",
    "rbac.ocp.io/role-type": "cluster-admin"
  },
  "annotations": {
    "rbac.ocp.io/created-by": "namespace-configuration-operator",
    "rbac.ocp.io/group-pattern": "app-ocp-rbac-*-cluster-admin",
    "rbac.ocp.io/scope-restriction": "cluster-wide",
    "rbac.ocp.io/source-groupconfig": "cluster-admin-groupconfig-rbac"
  }
}
```

**Step 3: Verify metadata on RoleBindings:**
```bash
oc get rolebindings -A -l app.kubernetes.io/managed-by=namespace-configuration-operator -o json | jq -r '.items[0] | {name: .metadata.name, namespace: .metadata.namespace, labels: .metadata.labels, annotations: .metadata.annotations}'
```
**Output:**
```json
{
  "name": "beta-audit-rb",
  "namespace": "beta-prod",
  "labels": {
    "app.kubernetes.io/managed-by": "namespace-configuration-operator",
    "app.kubernetes.io/version": "0.1.0",
    "rbac.ocp.io/access-level": "audit-prod-only",
    "rbac.ocp.io/config-source": "prod-rbac",
    "rbac.ocp.io/environment": "prod",
    "rbac.ocp.io/mnemonic": "beta",
    "rbac.ocp.io/policy-version": "0.1.0",
    "rbac.ocp.io/role-type": "ns-audit"
  },
  "annotations": {
    "rbac.ocp.io/created-by": "namespace-configuration-operator",
    "rbac.ocp.io/environment-restriction": "prod-only",
    "rbac.ocp.io/group-pattern": "app-ocp-rbac-beta-ns-audit",
    "rbac.ocp.io/source-namespace": "beta-prod",
    "rbac.ocp.io/source-namespaceconfig": "prod-namespaceconfig-rbac"
  }
}
```

**Test 2: Automatic Cleanup Verification (Production-Ready Behavior)**

This test proves that removing a label from a namespace automatically triggers cleanup of operator-generated resources, making this approach sustainable for production environments.

**Step 1: Find a namespace with resources:**
```bash
oc get namespaces -l company.net/app-environment=prod
```
**Output:**
```
NAME              STATUS   AGE
beta-prod         Active   4d4h
demo-prod         Active   4d16h
demo-production   Active   4d16h
```

**Step 2: Verify namespace has the label:**
```bash
oc get namespace beta-prod -o jsonpath='{.metadata.labels.company\.net/app-environment}'
```
**Output:**
```
prod
```

**Step 3: Verify RoleBindings exist in test namespace:**
```bash
oc get rolebindings -n beta-prod -l rbac.ocp.io/config-source=prod-rbac -o custom-columns=NAME:.metadata.name
```
**Output:**
```
NAME
beta-audit-rb
beta-developer-rb
```

**Step 4: Remove the label:**
```bash
oc label namespace beta-prod company.net/app-environment-
```
**Output:**
```
namespace/beta-prod unlabeled
```

**Step 5: Wait for operator reconciliation:**
```bash
sleep 15
```

**Step 6: Verify label was removed:**
```bash
oc get namespace beta-prod -o jsonpath='{.metadata.labels.company\.net/app-environment}'
```
**Output:**
```
```
*(Label removed - empty output)*

**Step 7: Verify RoleBindings are automatically deleted:**
```bash
oc get rolebindings -n beta-prod -l rbac.ocp.io/config-source=prod-rbac
```
**Output:**
```
No resources found in beta-prod namespace.
```

**Step 8: Verify only default RoleBindings remain:**
```bash
oc get rolebindings -n beta-prod
```
**Output:**
```
NAME                    ROLE                               AGE
admin                   ClusterRole/admin                  4d4h
system:deployers        ClusterRole/system:deployer        4d4h
system:image-builders   ClusterRole/system:image-builder   4d4h
system:image-pullers    ClusterRole/system:image-puller    4d4h
```
*(Only default system RoleBindings remain - operator-generated resources were automatically deleted)*

**Step 9: Verify NamespaceConfig labelSelector configuration:**
```bash
oc get namespaceconfig prod-namespaceconfig-rbac -o json | jq '.spec.labelSelector'
```
**Output:**
```json
{
  "matchExpressions": [
    {
      "key": "company.net/mnemonic",
      "operator": "Exists"
    },
    {
      "key": "company.net/app-environment",
      "operator": "In",
      "values": [
        "prod"
      ]
    }
  ]
}
```
*(The selector requires `company.net/app-environment=prod`, which beta-prod no longer has)*

**Step 10: Verify namespace no longer matches selector:**
```bash
oc get namespaces -l company.net/app-environment=prod
```
**Output:**
```
NAME              STATUS   AGE
demo-prod         Active   4d16h
demo-production   Active   4d16h
```
*(beta-prod no longer appears in the list)*

**Step 11: Check operator logs showing cleanup:**
```bash
oc logs -n namespace-configuration-operator namespace-configuration-operator-controller-manager-86dd4c7dt6q --tail=30 | grep -i "beta-prod\|reconciling\|namespaceconfig"
```
**Output:**
```json
{"level":"info","ts":"2025-12-10T22:20:55Z","msg":"All workers finished","controller":"controller_locked_object_rbac.authorization.k8s.io/v1/RoleBinding/beta-prod/beta-audit-rb"}
{"level":"info","ts":"2025-12-10T22:20:56Z","logger":"resource-reconciler./prod-namespaceconfig-rbac.rbac.authorization.k8s.io/v1/RoleBinding/demo-production/demo-developer-rb","msg":"reconcile called for","object":"rbac.authorization.k8s.io/v1/RoleBinding/demo-production/demo-developer-rb","request":{"name":"demo-developer-rb","namespace":"demo-production"}}
{"level":"info","ts":"2025-12-10T22:20:56Z","logger":"resource-reconciler./prod-namespaceconfig-rbac.rbac.authorization.k8s.io/v1/RoleBinding/demo-prod/demo-developer-rb","msg":"reconcile called for","object":"rbac.authorization.k8s.io/v1/RoleBinding/demo-prod/demo-developer-rb","request":{"name":"demo-developer-rb","namespace":"demo-prod"}}
{"level":"info","ts":"2025-12-10T22:20:56Z","logger":"resource-reconciler./prod-namespaceconfig-rbac.rbac.authorization.k8s.io/v1/RoleBinding/demo-prod/demo-audit-rb","msg":"reconcile called for","object":"rbac.authorization.k8s.io/v1/RoleBinding/demo-prod/demo-audit-rb","request":{"name":"demo-audit-rb","namespace":"demo-prod"}}
{"level":"info","ts":"2025-12-10T22:20:56Z","logger":"resource-reconciler./prod-namespaceconfig-rbac.rbac.authorization.k8s.io/v1/RoleBinding/demo-production/demo-audit-rb","msg":"reconcile called for","object":"rbac.authorization.k8s.io/v1/RoleBinding/demo-production/demo-audit-rb","request":{"name":"demo-audit-rb","namespace":"demo-production"}}
{"level":"info","ts":"2025-12-10T22:20:56Z","logger":"controllers.NamespaceConfig","msg":"reconciling started","namespaceconfig":{"name":"prod-namespaceconfig-rbac"}}
{"level":"info","ts":"2025-12-10T22:20:56Z","logger":"controllers.NamespaceConfig","msg":"resources processed successfully","namespaceconfig":{"name":"prod-namespaceconfig-rbac"},"namespaceconfig":"prod-namespaceconfig-rbac","namespaces":2,"resources":4}
{"level":"info","ts":"2025-12-10T22:20:56Z","logger":"controllers.NamespaceConfig","msg":"reconciling started","namespaceconfig":{"name":"prod-namespaceconfig-rbac"}}
{"level":"info","ts":"2025-12-10T22:20:56Z","logger":"controllers.NamespaceConfig","msg":"resources processed successfully","namespaceconfig":{"name":"prod-namespaceconfig-rbac"},"namespaceconfig":"prod-namespaceconfig-rbac","namespaces":2,"resources":4}
```
*(Logs show: "All workers finished" for beta-prod resources, and reconciliation now shows "namespaces":2 instead of 3, confirming cleanup. The resource-reconciler logs show only demo-prod and demo-production RoleBindings being reconciled, with no beta-prod resources, proving automatic cleanup worked correctly.)*

**Test 3: Automatic Resource Recreation (Complete Lifecycle)**

This test demonstrates that the operator also automatically recreates resources when a namespace label is added back, completing the full lifecycle demonstration.

**Step 1: Add the label back to the namespace:**
```bash
oc label namespace beta-prod company.net/app-environment=prod
```
**Output:**
```
namespace/beta-prod labeled
```

**Step 2: Verify label was added:**
```bash
oc get namespace beta-prod -o jsonpath='{.metadata.labels.company\.net/app-environment}'
```
**Output:**
```
prod
```

**Step 3: Wait for operator reconciliation:**
```bash
sleep 15
```

**Step 4: Verify RoleBindings are automatically recreated:**
```bash
oc get rolebindings -n beta-prod -l rbac.ocp.io/config-source=prod-rbac
```
**Output:**
```
NAME                ROLE               AGE
beta-audit-rb       ClusterRole/view   1s
beta-developer-rb   ClusterRole/edit   1s
```
*(RoleBindings show AGE of 1s, confirming they were just recreated)*

**Step 5: Verify namespace now matches selector again:**
```bash
oc get namespaces -l company.net/app-environment=prod
```
**Output:**
```
NAME              STATUS   AGE
beta-prod         Active   4d4h
demo-prod         Active   4d16h
demo-production   Active   4d16h
```
*(beta-prod is back in the list, confirming it matches the selector again)*

**Complete Lifecycle Demonstration:**

This test proves the operator handles the complete lifecycle:
- ✅ **Label Removed** → Resources automatically deleted
- ✅ **Label Added Back** → Resources automatically recreated
- ✅ **Production-Ready**: No manual intervention needed, operator handles both directions automatically

**Test 4: NetworkPolicy Example - Demonstrating Issue #50**

This test uses the `multitenant-networkpolicy.yaml` example to demonstrate Issue #50 with NetworkPolicy resources, showing that resources created without identifying metadata cannot be easily identified.

**Step 1: Apply the Multitenant NamespaceConfig:**
```bash
oc apply -f examples/namespace-config/multitenant-networkpolicy.yaml
```
**Output:**
```
namespaceconfig.redhatcop.redhat.io/multitenant created
```

**Step 2: Check initial state of beta-prod namespace:**
```bash
oc get namespace beta-prod -o jsonpath='{.metadata.labels}' | jq .
```
**Output:**
```json
{
  "company.net/app-environment": "prod",
  "company.net/mnemonic": "beta",
  "kubernetes.io/metadata.name": "beta-prod",
  "pod-security.kubernetes.io/audit": "restricted",
  "pod-security.kubernetes.io/audit-version": "latest",
  "pod-security.kubernetes.io/warn": "restricted",
  "pod-security.kubernetes.io/warn-version": "latest"
}
```
*(No `multitenant=true` label initially)*

```bash
oc get networkpolicies -n beta-prod
```
**Output:**
```
No resources found in beta-prod namespace.
```

**Step 3: Add multitenant label to beta-prod:**
```bash
oc label namespace beta-prod multitenant=true
```
**Output:**
```
namespace/beta-prod labeled
```

**Step 4: Wait for operator reconciliation:**
```bash
sleep 15
```

**Step 5: Verify NetworkPolicies are created:**
```bash
oc get networkpolicies -n beta-prod
```
**Output:**
```
NAME                           POD-SELECTOR   AGE
allow-from-default-namespace   <none>         13s
allow-from-same-namespace      <none>         13s
```

**Step 6: Full NetworkPolicy YAML showing no operator-added metadata:**
```bash
oc get networkpolicies -n beta-prod -oyaml
```
**Output:**
```yaml
apiVersion: v1
items:
- apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    creationTimestamp: "2025-12-11T00:14:39Z"
    generation: 1
    name: allow-from-default-namespace
    namespace: beta-prod
    resourceVersion: "15564753"
    uid: 0568aa09-b053-438e-9065-dd558a4ee2b7
  spec:
    ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            name: default
    podSelector: {}
    policyTypes:
    - Ingress
- apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    creationTimestamp: "2025-12-11T00:14:39Z"
    generation: 1
    name: allow-from-same-namespace
    namespace: beta-prod
    resourceVersion: "15564752"
    uid: f636d79a-9ce6-4ca0-900c-deea135e9e90
  spec:
    ingress:
    - from:
      - podSelector: {}
    podSelector: {}
    policyTypes:
    - Ingress
kind: List
metadata:
  resourceVersion: ""
```

**Important Observation:**

The NetworkPolicies shown above have **NO labels or annotations** in their metadata section. This demonstrates:

1. **Operator Management Without Metadata**: The operator can watch, monitor, and manage these NetworkPolicies even without identifying labels/annotations. The operator tracks resources internally through the `EnforcingReconciler` mechanism.

2. **Resource Identification Issue**: However, **users cannot easily identify** these as operator-generated resources because there are no identifying labels or annotations. Teams cannot distinguish between NetworkPolicies they created manually and those injected by the operator.

3. **Solution - Manual Metadata**: As shown in the RBAC example (`prod-namespaceconfig-rbac.yaml`), if you want to identify operator-generated resources, you must **manually add labels and annotations** to your templates. The operator does not automatically inject identifying metadata.

**Step 7: Test automatic cleanup (remove label):**
```bash
oc label namespace beta-prod multitenant-
```
**Output:**
```
namespace/beta-prod unlabeled
```

```bash
sleep 15 && oc get networkpolicies -n beta-prod
```
**Output:**
```
No resources found in beta-prod namespace.
```
*(NetworkPolicies automatically deleted)*

**Step 8: Test automatic recreation (add label back):**
```bash
oc label namespace beta-prod multitenant=true
```
**Output:**
```
namespace/beta-prod labeled
```

```bash
sleep 15 && oc get networkpolicies -n beta-prod
```
**Output:**
```
NAME                           POD-SELECTOR   AGE
allow-from-default-namespace   <none>         28s
allow-from-same-namespace      <none>         28s
```
*(NetworkPolicies automatically recreated with new AGE)*

**How Automatic Cleanup Works:**

1. **Operator Reconciliation**: The operator reconciles `NamespaceConfig` periodically and when namespace changes are detected
2. **Selector Re-evaluation**: `getSelectedNamespaces()` re-evaluates which namespaces match the selector
3. **Resource Comparison**: `UpdateLockedResources()` compares current desired state (only matching namespaces) with previously tracked state
4. **Automatic Cleanup**: Resources for namespaces that no longer match are automatically removed

**Example Template with Proper Metadata Specification:**

The following is a complete example showing how to properly specify identifying labels and annotations in templates:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: prod-namespaceconfig-rbac
  labels:
    app.kubernetes.io/name: namespace-configuration-operator
    app.kubernetes.io/component: rbac-automation
    rbac.ocp.io/scope: namespace-scoped
    rbac.ocp.io/kind: NamespaceConfig
  annotations:
    description: "Universal RBAC: audit/developer access for ALL environments (admin restricted to non-prod)"
spec:
  labelSelector:
    matchExpressions:
    - key: company.net/mnemonic
      operator: Exists  # Match any namespace with mnemonic label
    - key: company.net/app-environment
      operator: In
      values: ["prod"]  # EXPLICIT prod environments only
  templates:
    # Developer RoleBinding - Universal access for ALL environments (power users)
    - objectTemplate: |
        apiVersion: rbac.authorization.k8s.io/v1
        kind: RoleBinding
        metadata:
          name: "{{ index .Labels "company.net/mnemonic" }}-developer-rb"
          namespace: "{{ .Name }}"
          labels:
            app.kubernetes.io/managed-by: namespace-configuration-operator
            app.kubernetes.io/version: 0.1.0
            rbac.ocp.io/policy-version: 0.1.0
            rbac.ocp.io/role-type: ns-developer
            rbac.ocp.io/mnemonic: "{{ index .Labels "company.net/mnemonic" }}"
            rbac.ocp.io/environment: "{{ index .Labels "company.net/app-environment" }}"
            rbac.ocp.io/access-level: developer-prod-only
            rbac.ocp.io/config-source: prod-rbac
          annotations:
            rbac.ocp.io/created-by: namespace-configuration-operator
            rbac.ocp.io/source-namespace: "{{ .Name }}"
            rbac.ocp.io/source-namespaceconfig: prod-namespaceconfig-rbac
            rbac.ocp.io/group-pattern: "app-ocp-rbac-{{ index .Labels "company.net/mnemonic" }}-ns-developer"
            rbac.ocp.io/environment-restriction: "prod-only"
        subjects:
        - kind: Group
          name: "app-ocp-rbac-{{ index .Labels "company.net/mnemonic" }}-ns-developer"
          apiGroup: rbac.authorization.k8s.io
        roleRef:
          kind: ClusterRole
          name: edit
          apiGroup: rbac.authorization.k8s.io

    # Audit RoleBinding - Universal access for ALL environments (including prod)
    - objectTemplate: |
        apiVersion: rbac.authorization.k8s.io/v1
        kind: RoleBinding
        metadata:
          name: "{{ index .Labels "company.net/mnemonic" }}-audit-rb"
          namespace: "{{ .Name }}"
          labels:
            app.kubernetes.io/managed-by: namespace-configuration-operator
            app.kubernetes.io/version: 0.1.0
            rbac.ocp.io/policy-version: 0.1.0
            rbac.ocp.io/role-type: ns-audit
            rbac.ocp.io/mnemonic: "{{ index .Labels "company.net/mnemonic" }}"
            rbac.ocp.io/environment: "{{ index .Labels "company.net/app-environment" }}"
            rbac.ocp.io/access-level: audit-prod-only
            rbac.ocp.io/config-source: prod-rbac
          annotations:
            rbac.ocp.io/created-by: namespace-configuration-operator
            rbac.ocp.io/source-namespace: "{{ .Name }}"
            rbac.ocp.io/source-namespaceconfig: prod-namespaceconfig-rbac
            rbac.ocp.io/group-pattern: "app-ocp-rbac-{{ index .Labels "company.net/mnemonic" }}-ns-audit"
            rbac.ocp.io/environment-restriction: "prod-only"
        subjects:
        - kind: Group
          name: "app-ocp-rbac-{{ index .Labels "company.net/mnemonic" }}-ns-audit"
          apiGroup: rbac.authorization.k8s.io
        roleRef:
          kind: ClusterRole
          name: view
          apiGroup: rbac.authorization.k8s.io
```

**Benefits:**
- ✅ **Resource Identification**: Resources can be easily identified via labels/annotations
- ✅ **Queryable Resources**: Users can query operator-generated resources using standard Kubernetes label selectors
- ✅ **Automatic Cleanup**: Removing namespace labels automatically triggers resource cleanup (production-ready)
- ✅ **No CR Deletion Required**: Resources can be removed from specific namespaces without deleting the entire CR
- ✅ **Sustainable for Production**: This approach works well in production environments where multiple namespaces are managed by a single CR
- ✅ **Clear Ownership**: Annotations clearly identify which CR created each resource

**Issue Status:** ✅ **FIXED** - This issue has been resolved. Users can now identify operator-generated resources by manually adding labels and annotations to their templates. The operator correctly handles automatic cleanup and recreation of resources based on namespace label changes, making this solution production-ready and sustainable.

**See Also:** 
- [Resolved Issues Tracker - Issue #50](../resolved-issues-tracker/resolved-issues-tracker.md)
- [Groups and Bindings Examples](./groups-and-bindings-examples.md) - Includes resource identification examples

---

## Feature Enhancements

### Code Refactoring: Common Reconciler Helpers

**Status:** ✅ COMPLETED (December 10, 2025)

**Description:**
Extracted duplicate retry logic and logging helpers from individual controllers into a centralized common package to improve code maintainability and consistency.

**Features:**
- **Centralized Retry Logic**: `ManageSuccessWithRetry` function in common package
- **Centralized Logging Helpers**: `LogReconcilingStarted` and `LogResourcesProcessedSuccessfully` functions
- **Consistent Behavior**: All three controllers now use the same retry and logging logic
- **Reduced Code Duplication**: Removed ~59 lines of duplicate code from each controller

**Implementation:**
- Created `controllers/common/reconciler_helpers.go` with shared functionality
- Refactored `GroupConfigReconciler`, `NamespaceConfigReconciler`, and `UserConfigReconciler` to use common helpers
- Removed duplicate `manageSuccessWithRetry` methods from all three controllers
- Removed unused `time` import from controllers

**Files Modified:**
- `controllers/common/reconciler_helpers.go` - **NEW** - Common reconciler helper functions
- `controllers/groupconfig_controller.go` - Refactored to use common helpers (-59 lines)
- `controllers/namespaceconfig_controller.go` - Refactored to use common helpers (-59 lines)
- `controllers/userconfig_controller.go` - Refactored to use common helpers (-59 lines)

**Benefits:**
- **Maintainability**: Single source of truth for retry logic and logging
- **Consistency**: All controllers behave identically for retry and logging
- **Testability**: Common logic can be tested once and reused
- **Code Quality**: Reduced duplication improves maintainability

**See Also:** Commit `d9f697c` - "Refactor: Extract common reconciler helpers and add groups/bindings documentation"

---

### Enhanced Template Filtering with AND/OR Logic

**Status:** ✅ COMPLETED

**Description:**
Extended template filtering to all controllers (GroupConfig, NamespaceConfig, UserConfig) with comprehensive AND/OR logic support.

**Features:**
- **AND Logic**: When template uses `{{- if and`, ALL patterns must match
- **OR Logic**: When template uses `{{- if` or `{{- else if`, ANY pattern match is sufficient
- **Comprehensive Test Coverage**: Unit tests for all three controllers
- **Real-world Examples**: Test examples in `../examples/test-and-logic/`

**Files Modified:**
- All three controllers - Template filtering with AND/OR logic
- `controllers/unrecognized_conditionals_test.go` - **NEW** - Comprehensive tests
- `controllers/groupconfig_controller_test.go` - Extended tests
- `controllers/namespaceconfig_controller_test.go` - **NEW** - Comprehensive tests
- `controllers/userconfig_controller_test.go` - **NEW** - Comprehensive tests

**See Also:** [Resolved Issues Tracker - Enhanced Template Filtering](../resolved-issues-tracker/resolved-issues-tracker.md)

---

### Unrecognized Conditional Logic Detection

**Status:** ✅ COMPLETED

**Description:**
Enhanced detection of unrecognized template conditionals (eq, hasPrefix, ne, etc.) with fallback behavior.

**Features:**
- Improved detection of unrecognized conditionals
- Fallback: Templates apply to all resources when unrecognized conditionals detected
- V(2) level logging for unrecognized conditional detection
- Comprehensive test coverage

**Files Modified:**
- All three controllers - Unrecognized conditional detection
- `controllers/unrecognized_conditionals_test.go` - Test coverage

**See Also:** [Resolved Issues Tracker - Unrecognized Conditionals](../resolved-issues-tracker/resolved-issues-tracker.md)

---

### Template Filtering: Guards on Labels and Annotations, Render Fallback

**Status:** ✅ COMPLETED — supersedes the two sections above. Their scenarios still hold; the mechanism changed.

**Problem:**
The pattern-based filter only understood `hasSuffix "…"` and `contains "…"`, and always compared them
against `.Name`. A guard on a label or an annotation — for example
`{{- if hasPrefix "app-team-" (index .Labels "example.com/oud-group") }}` — or any other conditional
(`eq`, `ne`, `not`, a bare truthiness check) fell through to "apply to every object, rely on the renderer".
The renderer cannot represent an empty render: operator-utils turns it into the JSON literal `null`, fails it
with `Object 'Kind' is missing in 'null'` at error level, and drops every object it had already rendered for that
param while returning a nil error. Measured on a cluster with two label-guarded NamespaceConfigs and four labelled
namespaces: 40 error lines in under three minutes, with the CRs reporting success throughout.

**Solution:**
- One implementation, `controllers/common/templatefilter.go`, used by all three controllers.
- The template is parsed once (cached by text, with the renderer's own function map) and its syntax tree is
  inspected. A single top-level `if` / `else if` / `else` chain over `hasPrefix`, `hasSuffix`, `contains`, `eq`,
  `ne`, `and`, `or`, `not`, and the truthiness of `.Name` or `(index .Labels "k")` / `(index .Annotations "k")`
  is evaluated statically against the object's name, labels and annotations.
- Any other shape — a top-level variable, a pipeline, `range`, `with`, `.Spec` access, an unknown function — is
  decided by rendering the template and checking for blank output. That is exactly what the renderer sees, so
  it is always right; it costs one extra render for that template only.
- A template that does not parse is still handed to the renderer, so the parse error is reported with the
  resource attached, as before.
- Guard-looking text inside YAML comments no longer influences the decision (the old regexes scanned comments).

**Files Modified:**
- `controllers/common/templatefilter.go` - **NEW** - the filter
- `controllers/common/templatefilter_test.go` - **NEW** - includes a property test asserting, for every guard shape
  and subject, that the filter's answer equals "the real render is non-blank"
- `controllers/{groupconfig,namespaceconfig,userconfig}_controller.go` - delegate to the common filter; the
  regex extractors are gone (-440 lines)
- `controllers/*_controller_test.go`, `controllers/unrecognized_conditionals_test.go` - rewritten for the new API
- `hack/local-ci.sh`, `hack/push-quay.sh` - **NEW** - the pre-push gate and the image publish step, kept out of
  `.github/workflows/` on purpose

**Verification:**
- `go test -race ./...` green.
- Run against a cluster with the same two CRs and four namespaces: zero error lines, the rejected namespaces logged
  once at V(1) as "skipping namespace", the matching namespaces received the same Role and RoleBinding as before.

---

### Render Failures Fail the Reconcile Instead of Deleting Managed Objects

**Status:** ✅ COMPLETED (issue #1)

**Problem:**
operator-utils' `GetLockedResourcesFromTemplatesWithRestConfig` logs a parse or render failure and returns an
EMPTY list with a nil error. The controllers appended that empty batch, so the enforcer saw a desired state
missing every object of the failing namespace/group/user and deleted them, while the CR reported
`ReconcileSuccess`. Measured on a cluster: removing a label that a template reads through `required` deleted that
namespace's RoleBinding under a green status; the only trace was three error-level log lines.

**Solution:**
- `TemplateFilter.Render` (controllers/common/templatefilter.go) renders through the renderer's own
  `ProcessTemplateArray` on the filter's cached parse and returns every error, naming the object and the template.
- `getResourceList` in all three controllers uses it; a failure ends the reconcile in `ManageError`, which sets
  `ReconcileError` on the CR and emits a Warning event, and the enforcer is never called with a partial batch.
- Side effect: the controllers no longer touch the library's unsynchronised package-global template map (issue #8).

**Files Modified:**
- `controllers/common/templatefilter.go` - `Render`, `renderData`
- `controllers/common/templatefilter_test.go` - error propagation, excludedPaths carried, value-not-pointer semantics
- `controllers/{namespaceconfig,groupconfig,userconfig}_controller.go` - `getResourceList(ctx, ...)`
- `controllers/render_errors_test.go` - **NEW** - one failing object fails `getResourceList` in every controller

---

### CR Deletion Recomputes the Owned Set

**Status:** ✅ COMPLETED (issue #2)

**Problem:**
`Terminate` deletes only what the in-memory enforcer was started with. That map is empty after an operator
restart and the entry is dropped after a failed Terminate, so a CR deleted in either state finalized with every
managed object orphaned; the chart's orphan-sweeper Job was papering over it.

**Solution:**
`manageCleanUpLogic` in all three controllers calls `Terminate` first, then recomputes the owned set from the spec
(`TemplateFilter.OwnedResources` over the selected namespaces/groups/users) and deletes it explicitly. A selected
object whose templates no longer render is reported as a `CleanupIncomplete` Warning event and an error-level log
line, and deletion proceeds; a failed DELETE keeps the finalizer.
A CR whose selector does not compile cannot have its owned set computed at all (and never created anything under
that spec); its deletion completes with a `CleanupIncomplete` warning instead of hanging on the finalizer.

**Files Modified:**
- `controllers/common/templatefilter.go` - `OwnedResources`
- `controllers/{namespaceconfig,groupconfig,userconfig}_controller.go` - `manageCleanUpLogic(ctx, ...)`
- `controllers/cleanup_test.go` - **NEW** - owned objects deleted without a started enforcer; list failure keeps the finalizer

---

### A Malformed Selector Only Affects Its Own CR

**Status:** ✅ COMPLETED (issue #3)

**Problem:**
The Namespace and Group watch map funcs returned on the first CR whose selector failed to compile (the API
server accepts an unknown operator; the CRD has no enum), so no CR was enqueued for any namespace/group event
while such a CR existed, and the outage persisted after its deletion until an unrelated event arrived.

**Solution:**
`findApplicableNameSpaceConfigs` and `findApplicableGroupConfigsFromGroup` log and skip the offending CR (whose own
reconcile already reports `ReconcileError`) and keep evaluating the rest, as `UserConfigReconciler.matches` always
did.

**Files Modified:**
- `controllers/namespaceconfig_controller.go`, `controllers/groupconfig_controller.go`
- `controllers/malformed_selector_test.go` - **NEW**

---

### Template Filter Agrees With the Renderer on Values and on What Counts as an Object

**Status:** ✅ COMPLETED (issue #4)

**Problem:**
The filter's render fallback executed against a pointer while the renderer receives the value, so a template
using a pointer-receiver method (`{{ .GetName }}`) passed the filter and then failed in the renderer. The filter
also equated "non-blank output" with "renders an object", so a taken branch containing only a `#` comment or a
bare `---` reached the renderer, whose YAML-to-JSON step turns it into `null` and fails.

**Solution:**
The fallback executes against the same value the renderer receives; applicability is decided by whether the
output parses to something other than `null`; the static evaluator ignores comment and `---` lines when judging a
branch's content. The property test's oracle is now operator-utils' own `ProcessTemplateArray` on the value.

**Files Modified:**
- `controllers/common/templatefilter.go` - `rendersAnObject`, `textHasYAMLContent`, value-based fallback
- `controllers/common/templatefilter_test.go` - renderer oracle; pointer-method, comment-only, `---`-only, template-comment shapes

---

### One Selection per User Across Identities

**Status:** ✅ COMPLETED (issue #6)

A user with N matching identities was appended N times by `getSelectedUsers` (and enqueued N times by the identity watch), so every template rendered N times and the enforcer ran N child controllers for one object. Both loops now stop at the first matching identity.

**Files Modified:** `controllers/userconfig_controller.go`, `controllers/user_dedupe_test.go` (**NEW**)

---

### Selected-Object Watches Only React to Label and Annotation Changes

**Status:** ✅ COMPLETED (issue #5)

The Namespace watch had no predicate, so every status or resourceVersion bump on any namespace listed every NamespaceConfig, re-rendered each matching one and rewrote its status. A namespace's contract with a NamespaceConfig is its labels and annotations, so `common.SelectedObjectChangedPredicate` (label or annotation changed; Create and Delete always pass) gates the Namespace watch only. Measured on a cluster: 5 status-only namespace updates caused 5 reconciles before and 0 after; a label change still causes 1. The Group and User watches are deliberately unfiltered: `Group.users` and `User.identities` are top-level fields a template can read, and with the gate on those watches a membership change was dropped (measured in review: 0 reconciles, 2 after removing it). Known limit, deliberate: a NamespaceConfig template reading `.Spec`/`.Status` of the namespace through the render fallback is not re-rendered when only those change.

**Files Modified:** `controllers/common/common.go`, `controllers/common/predicate_test.go` (**NEW**), the three controllers' `SetupWithManager`

---

### Small Correctness Cleanups

**Status:** ✅ COMPLETED (issue #17)

- `IsInitialized` no longer rewrites `excludedPaths` on a CR that is being deleted (one fewer spec Update mid-deletion); its return value is named for what it means. (Superseded by #16: it now writes finalizers only, on any CR.)
- `main.go` logs when the User or Group controller is not started because the cluster does not serve that kind; `SYNC_PERIOD_SECONDS` must be a positive integer (clear error otherwise); an invalid `ALLOW_SYSTEM_NAMESPACES` is logged instead of silently read as false. Both parsers are unit-tested.
- `ManageSuccessWithRetry` takes the reconciled generation and writes no success for an object whose spec moved on in the meantime (its reconcile is already queued), so `observedGeneration` never claims an unprocessed generation.
- `LogReconcilingStarted` names the resource; the identity watch logs an identity without a user at V(1) instead of error level; `findApplicableUserConfigsFromIdentities` takes the caller context; the `Reconcile` parameter no longer shadows the `context` package; the dead `common.GetResources` (which aliased the range variable) is gone.

---

### Deletion Tracking and Logging

**Status:** ✅ COMPLETED

**Description:**
Added comprehensive deletion tracking logs to prevent continuous lookups for deleted objects and avoid false positives.

**Features:**
- Info-level deletion detection logs
- Deletion processing logs
- Deletion completion logs
- Clear lifecycle tracking for all three CR types

**Files Modified:**
- `controllers/groupconfig_controller.go` - Deletion tracking
- `controllers/namespaceconfig_controller.go` - Deletion tracking
- `controllers/userconfig_controller.go` - Deletion tracking

**Test Resources:**
- `../examples/test-and-logic/test-deletion-tracking-groupconfig.yaml`
- `../examples/test-and-logic/test-deletion-tracking-namespaceconfig.yaml`
- `../examples/test-and-logic/test-deletion-tracking-userconfig.yaml`

**Real-World Example:**

The deletion tracking logs provide clear visibility into the resource deletion lifecycle. Here's an example from a production cluster:

**1. List existing GroupConfig resources:**
```bash
oc get groupconfig

NAME                                                  AGE
cluster-admin-groupconfig-rbac                        14h
cluster-audit-groupconfig-rbac                        2d15h
cluster-developer-groupconfig-rbac                    2d15h
user-workload-monitoring-admin-groupconfig-rbac       3d8h
user-workload-monitoring-developer-groupconfig-rbac   3d8h
```

**2. Delete a GroupConfig:**
```bash
oc delete groupconfig cluster-audit-groupconfig-rbac

groupconfig.redhatcop.redhat.io "cluster-audit-groupconfig-rbac" deleted
```

**3. Deletion tracking logs show the complete lifecycle:**

**Deletion Processing Log** (when deletion timestamp is detected):
```json
{
  "level": "info",
  "ts": "2025-12-10T17:51:07Z",
  "logger": "controllers.GroupConfig",
  "msg": "resource deletion detected - processing deletion cleanup",
  "groupconfig": {
    "name": "cluster-audit-groupconfig-rbac"
  },
  "groupconfig": "cluster-audit-groupconfig-rbac",
  "deletionTimestamp": "2025-12-10 17:51:07 +0000 UTC"
}
```

**Deletion Completion Log** (when deletion finishes successfully):
```json
{
  "level": "info",
  "ts": "2025-12-10T17:51:07Z",
  "logger": "controllers.GroupConfig",
  "msg": "resource deletion completed successfully",
  "groupconfig": {
    "name": "cluster-audit-groupconfig-rbac"
  },
  "groupconfig": "cluster-audit-groupconfig-rbac"
}
```

**Deletion Detection Log** (when resource is not found during reconciliation):
```json
{
  "level": "info",
  "ts": "2025-12-10T17:51:07Z",
  "logger": "controllers.GroupConfig",
  "msg": "resource deletion detected - resource not found, skipping reconciliation",
  "groupconfig": {
    "name": "cluster-audit-groupconfig-rbac"
  },
  "groupconfig": {
    "name": "cluster-audit-groupconfig-rbac"
  }
}
```

**Benefits:**
- **Clear visibility**: Operators can see exactly when resources are being deleted
- **Prevents false positives**: Logs clearly indicate when a resource is deleted vs. missing
- **Lifecycle tracking**: Complete audit trail of deletion events
- **Troubleshooting**: Easy to identify if deletion is stuck or completed successfully
- **No continuous lookups**: System stops attempting to reconcile deleted resources

**See Also:** [Resolved Issues Tracker - Deletion Tracking](../resolved-issues-tracker/resolved-issues-tracker.md)

---

### Retry Success Logging

**Status:** ✅ COMPLETED

**Description:**
Added V(1) level logging when operations succeed after retries to distinguish retries from actual errors in centralized logging.

**Features:**
- V(1) level retry success logs
- Retry attempt tracking
- Helps prevent false positives in ELK/log aggregation systems

**Files Modified:**
- All three controllers - Retry success logging in `manageSuccessWithRetry` function

**See Also:** [Resolved Issues Tracker - Retry Success Logging](../resolved-issues-tracker/resolved-issues-tracker.md)

---

### Skipping Resource Logging

**Status:** ✅ COMPLETED

**Description:**
Added V(1) level logging when resources are skipped because no templates match their pattern.

**Features:**
- Clear messages when groups/namespaces/users are skipped
- Includes resource name and CR name for context
- Visible with `ZAP_LOG_LEVEL=1` or higher

**Files Modified:**
- `controllers/groupconfig_controller.go` - Skipping logs
- `controllers/namespaceconfig_controller.go` - Skipping logs
- `controllers/userconfig_controller.go` - Skipping logs

**Log Format:**
```json
{"level":"debug","msg":"skipping group - no GroupConfig templates match the group pattern","group":"app-ocp-rbac-platform-cluster-admin","groupconfig":"cluster-audit-groupconfig-rbac"}
```

**See Also:** 
- [Issue #134 - Logging Enhancements](#issue-134-log-level-configuration)
- [Issue #50 - Resource Identification](#issue-50-provide-a-way-to-identify-operator-generated-resources)

---

## Build System Improvements

### Version Information Injection

**Status:** ✅ COMPLETED

**Description:**
Automatic version information injection in both Makefile and PodmanMakefile for consistent version tracking.

**Features:**
- Automatic version detection from git
- Build args passed to Dockerfile
- Version info embedded in binary via ldflags
- Works with both Makefile and PodmanMakefile

**Files Modified:**
- `Makefile` - Version injection in `docker-build` target
- `PodmanMakefile` - Version injection in `container_build` function
- `Dockerfile` - Build args for VERSION, COMMIT, BUILD_DATE

**Documentation:**
- [MAKEFILE_VERSION_INJECTION.md](./MAKEFILE_VERSION_INJECTION.md)
- [DOCKERFILE_ENHANCEMENTS.md](./DOCKERFILE_ENHANCEMENTS.md)
- [CI_CD_VERSION_INJECTION.md](./CI_CD_VERSION_INJECTION.md)

**See Also:** [Resolved Issues Tracker - Version Information System](../resolved-issues-tracker/resolved-issues-tracker.md)

---

### Build and Run Scripts

**Status:** ✅ COMPLETED

**Description:**
Simplified build and run scripts for local development.

**Features:**
- `build.sh` - Wrapper script with automatic version detection
- `run-go.sh` - Script to build and run operator locally with log configuration
- Supports `--log-level`, `--dev`, `--skip-build`, `--stop` options

**Files Created:**
- `build.sh` - **NEW**
- `run-go.sh` - **NEW**
- `BUILD-RUN.md` - **NEW** - Comprehensive documentation

**See Also:** [Resolved Issues Tracker - Build and Run Scripts](../resolved-issues-tracker/resolved-issues-tracker.md)

---

## Logging Enhancements

### Template Filtering Debug Logs

**Status:** ✅ COMPLETED

**Description:**
V(2) level debug logs for template filtering to help troubleshoot template matching issues.

**Features:**
- Shows which patterns are being checked
- Explains why groups match or don't match
- Visible with `ZAP_LOG_LEVEL=2` or higher

**Documentation:**
- [TEMPLATE_FILTERING_LOGS_EXPLANATION.md](./TEMPLATE_FILTERING_LOGS_EXPLANATION.md)

---

### Structured JSON Logging

**Status:** ✅ COMPLETED

**Description:**
All logs use structured JSON format for easy parsing and filtering in ELK and other log aggregation systems.

**Configuration:**
- `ZAP_DEVEL=false` - JSON format (production)
- `ZAP_DEVEL=true` - Console format (development)

**Important Configuration Note (Updated December 10, 2025):**
- **For OLM-managed deployments**: Configure `ZAP_LOG_LEVEL` and `ZAP_DEVEL` via `Subscription.spec.config.env`, NOT directly on the Deployment
- **For local development**: Set environment variables when running `./run-go.sh`
- **Documentation updated**: Corrected guidance in `groups-and-bindings-examples.md` to reflect proper configuration method

**Example Operator Logs:**
The documentation now includes real-world log examples showing:
- `reconciling started` messages with GroupConfig names
- `resources processed successfully` messages with group counts and resource counts
- Structured JSON format suitable for log aggregation systems
- Log level: `info` (ZAP_LOG_LEVEL=info)
- Development mode: `false` (ZAP_DEVEL=false)

**See Also:** 
- [Issue #134 - Log Level Configuration](#issue-134-log-level-configuration)
- [Groups and Bindings Examples](./groups-and-bindings-examples.md) - Includes log examples and configuration guidance

---

## Documentation

### Comprehensive Documentation Created

**Status:** ✅ COMPLETED

**New Documentation Files:**
1. **Issue Documentation:**
   - Issue #50: Comprehensive documentation in `FEATURES_AND_ISSUES_RESOLUTION.md` with test results and template examples
   - `../examples/test-and-logic/ISSUE-134-ROOT-CAUSE-SUMMARY.md`
   - `../examples/test-and-logic/ISSUE-134-VERIFICATION-GUIDE.md`
   - `../examples/test-and-logic/ISSUE-134-FIX-IMPLEMENTATION.md`
   - `../examples/test-and-logic/ISSUE-194-ROOT-CAUSE-SUMMARY.md`
   - `../examples/test-and-logic/ISSUE-194-VERIFICATION-GUIDE.md`
   - `../examples/test-and-logic/ISSUE-194-FIX-IMPLEMENTATION.md`

2. **Technical Documentation:**
   - `./groups-and-bindings-examples.md` - Groups and bindings examples with resource identification guidance (Issue #50)
   - `./LOG_LEVEL_CONFIGURATION.md` - Log level configuration guide
   - `./DOCKERFILE_ENHANCEMENTS.md` - Dockerfile enhancements
   - `./MAKEFILE_VERSION_INJECTION.md` - Makefile version injection
   - `./CI_CD_VERSION_INJECTION.md` - CI/CD version injection
   - `./TEMPLATE_FILTERING_LOGS_EXPLANATION.md` - Template filtering logs
   - `./groups-and-bindings-examples.md` - **NEW** (December 10, 2025) - Groups and bindings examples with commands

3. **Build and Run:**
   - `../BUILD-RUN.md` - Build and run instructions

4. **Resolved Issues Tracker:**
   - `../resolved-issues-tracker/resolved-issues-tracker.md` - Comprehensive tracker

**Groups and Bindings Examples Documentation (NEW - December 10, 2025):**

Created comprehensive documentation (`./groups-and-bindings-examples.md`) providing (related to Issue #50):
- **Group Naming Patterns**: Cluster-level and namespace-level group conventions
- **Example Groups**: Commands to view and inspect groups
- **ClusterRoleBindings Examples**: How to view and verify cluster-level bindings
- **RoleBindings Examples**: How to view and verify namespace-level bindings
- **Common Queries**: Practical commands for counting, finding, and verifying bindings
- **Example Operator Logs**: Real-world log examples with explanations
  - Shows structured JSON logs with `ZAP_LOG_LEVEL=info` and `ZAP_DEVEL=false`
  - Explains log fields: `reconciling started`, `resources processed successfully`, `groups`, `resources`
  - Includes commands for filtering and monitoring logs
- **Log Level Configuration**: Correct guidance on configuring via Subscription (not Deployment)
- **Troubleshooting**: Commands for verifying operator status and manual reconciliation

**Key Features:**
- Practical, copy-paste ready commands
- Real-world examples from production clusters
- Clear explanations of log structure and meaning
- Correct configuration guidance (Subscription-based, not Deployment-based)

**Documentation Locations:**
- `./groups-and-bindings-examples.md` - In this repository (namespace-configuration-operator)
- `../openshift-rbac-automation/docs/groups-and-bindings-examples.md` - In openshift-rbac-automation repository (for end users)

**See Also:** 
- [Resolved Issues Tracker - Documentation](../resolved-issues-tracker/resolved-issues-tracker.md)
- [Groups and Bindings Examples](./groups-and-bindings-examples.md) - Includes resource identification examples (Issue #50)
- [Issue #50 - Resource Identification](#issue-50-provide-a-way-to-identify-operator-generated-resources)

---

## Future Enhancements

### Template-Based Label/Annotation Matching

**GitHub Issue:** [#193 - Add support for template-based label/annotation matching](https://github.com/redhat-cop/namespace-configuration-operator/issues/193)  
**Status:** Open - Enhancement request

**Problem Statement:**
Currently, NamespaceConfig matching is limited to static label selectors. There's no way to match namespaces based on dynamic template expressions that evaluate against the namespace itself.

**Proposed Solution:**
Add `labelMatchTemplate` field to NamespaceConfig API to enable self-referential patterns.

**Complexity:** Moderate to High - Requires CRD schema changes

**See Also:** [Original Issue Documentation](#future-enhancement-template-based-labelannotation-matching) (below)

---

## Detailed Issue Documentation

### Issue 1: GroupConfig "Object is Null" Template Rendering Fix

#### Problem Statement
The GroupConfigReconciler was attempting to process templates for groups that don't match the template's conditional logic, resulting in "object is null" errors during template rendering. This happens when templates contain conditional statements like `{{- if hasSuffix "-cluster-admin" .Name }}` but the controller processes ALL groups regardless of whether they match the conditions.

#### Root Cause
The original `getResourceList` function processes all templates for all groups without filtering, causing template rendering failures when:
1. A template expects a group name ending with `-cluster-admin`
2. But a group with name `app-ocp-rbac-alpha-cluster-audit` is passed to it
3. The template's conditional logic fails and renders null objects

#### Solution: Dynamic Pattern Extraction and Template Filtering
Implemented four new methods to filter templates before processing:
1. **`filterApplicableTemplates`** - Pre-filters templates for each group
2. **`isTemplateApplicableToGroup`** - Determines if template conditions match group
3. **`extractHasSuffixPatterns`** - Extracts `hasSuffix` patterns from templates
4. **`extractContainsPatterns`** - Extracts `contains` patterns from templates

#### Resolution Status: ✅ COMPLETED
- **Code implemented**: Dynamic filtering methods applied directly to the original GroupConfigReconciler
- **Pattern extraction**: Supports both `hasSuffix` and `contains` conditions  
- **Production testing**: Verified with existing GroupConfig resources - no more null object errors
- **Unit testing**: Comprehensive test coverage created and validated
- **Location**: Fix applied directly in `controllers/groupconfig_controller.go`

#### Unit Test Coverage ✅
**Test File**: `controllers/groupconfig_controller_test.go`

**Test Functions:**
1. **`TestExtractHasSuffixPatterns`** (3 test cases)
2. **`TestExtractContainsPatterns`** (3 test cases)
3. **`TestIsTemplateApplicableToGroup`** (4 test cases)
4. **`TestFilterApplicableTemplates`** (2 test cases)

---

### Issue 2: Fix Finalizer Domain Qualification and Rebuild Operator

#### Problem Statement
The namespace-configuration-operator is using non-domain-qualified finalizer names which causes Kubernetes API warnings and violates best practices.

#### Solution Implementation
Updated to use canonical Kubernetes format:
- **NamespaceConfig**: `redhatcop.redhat.io/namespaceconfig-controller`
- **GroupConfig**: `redhatcop.redhat.io/groupconfig-controller`  
- **UserConfig**: `redhatcop.redhat.io/userconfig-controller`

#### Resolution Status: ✅ COMPLETED
- **Code implementation**: All three controller finalizers updated to canonical format
- **Domain alignment**: Now matches CRD API group `redhatcop.redhat.io`
- **Format compliance**: Follows Kubernetes `domain/name` standard
- **Backward compatibility**: Implemented robust migration logic to handle legacy finalizers
- **Deletion fix**: Added specific logic to handle resources stuck in deletion

---

### Issue 3: Controller Reconciliation Triggering (Predicates)

#### Problem Statement
Resources stuck in deletion were not being reconciled by the operator because the `ResourceGenerationOrFinalizerChangedPredicate` was filtering out update events where only the `deletionTimestamp` changed.

#### Solution: Custom Predicate Implementation
Implemented a custom predicate `ResourceGenerationOrFinalizerOrDeletionTimestampChangedPredicate` that extends the standard predicate to also handle deletion timestamp changes.

**Location**: `controllers/common/common.go`

**Key Features:**
1. ✅ **Generation changes** (spec updates) - triggers reconciliation
2. ✅ **Finalizer changes** (added/removed) - triggers reconciliation  
3. ✅ **Deletion timestamp changes** - triggers reconciliation

#### Resolution Status: ✅ COMPLETED
- **Code implementation**: Custom predicate created in `controllers/common/common.go`
- **All controllers updated**: NamespaceConfig, GroupConfig, and UserConfig controllers now use the new predicate
- **Production ready**: Properly handles all reconciliation scenarios including stuck deletions

---

### Issue 4: Startup Banner and Version Information Display

#### Problem Statement
When the operator starts, there was no visible indication of which version or commit was running.

#### Solution: Startup Banner with Version Information
Implemented a prominent startup banner that displays version, commit hash, and build date information.

**Location**: `internal/version/version.go` and `main.go`

#### Implementation Details

**1. Version Package (`internal/version/version.go`)**
- Variables: `Version`, `Commit`, `BuildDate` (set via `ldflags` during build)
- `GetVersion()`: Retrieves version with fallback priority
- `GetCommitHash()`: Retrieves commit hash with fallback priority
- `GetBuildDate()`: Retrieves build date with fallback priority
- `PrintStartupBanner()`: Displays formatted ASCII art banner

**2. Automatic Version Detection**
The Makefile and PodmanMakefile automatically detect version information from git.

**3. Banner Display**
Prominent ASCII art format showing version, commit, and build date.

#### Resolution Status: ✅ COMPLETED
- **Code implementation**: Version package created with automatic detection
- **Startup banner**: Prominent display on operator startup
- **Automatic versioning**: Makefiles automatically detect version from git
- **Container builds**: Version info embedded in container images

---

### Future Enhancement: Template-Based Label/Annotation Matching

**GitHub Issue**: [#193 - Add support for template-based label/annotation matching](https://github.com/redhat-cop/namespace-configuration-operator/issues/193)  
**Status**: Open - Enhancement request

#### Problem Statement
Currently, NamespaceConfig matching is limited to static label selectors. There's no way to match namespaces based on dynamic template expressions that evaluate against the namespace itself.

#### Proposed Solution
Add `labelMatchTemplate` field to NamespaceConfig API:

```yaml
apiVersion: redhatcop.redhat.io/v1alpha1
kind: NamespaceConfig
metadata:
  name: gitops-config
spec:
  labelMatchTemplate:
    argocd.argoproj.io/managed-by: "{{ .Name }}-argo"
  templates:
    - objectTemplate: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: gitops-config
          namespace: "{{ .Name }}-argo"
```

#### Implementation Complexity
**Moderate to High**:
- 🔄 Requires CRD schema changes
- 🔄 New API fields and validation
- 🔄 Template engine integration
- 🔄 Backward compatibility considerations

---

## Related Documentation

- [Resolved Issues Tracker](../resolved-issues-tracker/resolved-issues-tracker.md) - Comprehensive tracker of all resolved issues
- [Documentation Directory](./) - Technical documentation
- [Test Examples](../examples/test-and-logic/) - Test examples and documentation
- [Build and Run Guide](../BUILD-RUN.md) - Build and run instructions

---

**Note**: This document provides a high-level overview. For detailed technical information, see the specific documentation files referenced in each section.

---

### Enforcement by Server-Side Apply; `.metadata` No Longer Excluded by Default

**Status:** ✅ COMPLETED (issue #16; library branch feat/ssa-enforcer of the operator-utils fork)

The operator-utils enforcer applied a merge patch, which can add and replace but never remove, so a field a template stopped rendering survived and, because a foreign label was a permanent difference, `.metadata` had to be excluded wholesale: labels and annotations were set at creation and never enforced. The fork's `LockedResourceReconciler` now applies server-side under one field manager with force. Measured on a cluster with this build: every object's legacy `manager`/Update entry folded on the first pass (the operator names it explicitly in `main.go`, the library folds nothing by default), labels and annotations kept, a hand-added subject removed within one reconcile, a hand-added label kept, a forced reconcile changing no resourceVersion. With `.metadata` removed from a NamespaceConfig's excludedPaths, the reconciler took ownership of the rendered labels and annotations, a tampered rendered label was restored and a foreign label left alone.

`DefaultExcludedPaths` no longer carries `.metadata` (see the later entry for the current set and where it is applied). A CR that still lists `.metadata` keeps the old behaviour for its objects (set once, left alone) and is named by a `MetadataExcluded` Warning event; the chart migrates its CRs by declaring the list itself (chart 0.22.0). An excluded path is honoured at the granularity the server tracks ownership: an exclusion inside an atomic list (RBAC rules) or atomic map excludes the whole unit.

**Files Modified:** `controllers/common/common.go`, `controllers/isinitialized_test.go`, `main.go`, `go.mod`

---

### Default excludedPaths Applied in Memory; the CR Spec Is the Author's

**Status:** ✅ COMPLETED (issue #16, design review 2026-09-05; docs/DESIGN_excludedPaths.md)

Since upstream, `IsInitialized` unioned the default excludedPaths into `spec.templates[].excludedPaths` and wrote the CR on first reconcile. Every CR therefore differed from what its author or their Git declared, a GitOps controller with self-heal fought the operator over the spec (recorded in the chart that deploys this operator, 0.21.1), and the chart had started mirroring the operator's defaults to keep Git equal to the cluster. The defaults are now applied when the locked resources are built (`common.EffectiveExcludedPaths`, sorted union of the defaults and the author's list); `IsInitialized` writes finalizers only. Measured on a cluster with this build: a fresh CR declaring nothing keeps `excludedPaths` absent, its generation moves once for the finalizer, the operator owns and enforces its ConfigMap's rendered label and leaves a hand-added label alone; existing CRs' generations do not move.

Finalizers are written as a merge patch from the pre-mutation copy, so only `metadata.finalizers` crosses the wire (a whole-object Update serialised `annotationSelector: {}` into the spec; measured). A template that still excludes `.metadata` raises a `MetadataExcluded` Warning event on its CR. `.metadata.finalizers` joins the defaults (review of PR #40): a finalizer names the controller that owns that lifecycle step, and a template that renders one must not make this operator re-add it after that controller removed it. README, the CSV description and WARP now state the current defaults, and a test fails when the code and the documents disagree.

**Files Modified:** `controllers/common/common.go`, `controllers/common/templatefilter.go`, `controllers/{namespaceconfig,groupconfig,userconfig}_controller.go`, `controllers/isinitialized_test.go`, `controllers/common/templatefilter_test.go`, `README.md`, `config/manifests/bases/*.clusterserviceversion.yaml`, `WARP.md`, `go.mod` (comment), `main.go` (comment)
