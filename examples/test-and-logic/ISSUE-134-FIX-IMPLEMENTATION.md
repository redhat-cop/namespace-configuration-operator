# Issue #134 — Fix Implementation

## Issue Reference
- **GitHub Issue**: https://github.com/redhat-cop/namespace-configuration-operator/issues/134
- **Problem**: Operator creating lots of Info logs sent to ELK, need to set log level to Error
- **Status**: ✅ RESOLVED

## Solution Overview
- **Environment Variable Support**: Added `ZAP_LOG_LEVEL` and `ZAP_DEVEL` environment variable support in `main.go`
- **Two Configuration Methods for OLM-Managed Deployments**:
  1. **Update Subscription** (OLM-native, recommended) - Add environment variables to Subscription spec.config.env
  2. **Kyverno Policy** (Alternative) - Created ClusterPolicy to inject log level environment variables into the operator Deployment
- **OLM-Compatible**: Both methods work with OLM-managed deployments and persist across operator updates
- **Enhanced Logging Features**:
  - V(1) level logging for skipped resources (groups/namespaces/users)
  - V(2) level logging for template filtering details
  - Info-level deletion tracking logs
  - V(1) level retry success logs
  - Structured JSON logging format
- **Documentation**: Added comprehensive documentation for log level configuration and all logging enhancements

## Implementation Details

### 1. Environment Variable Support (main.go)

**Location**: `main.go`

**Implementation**:
```go
// Check for ZAP_LOG_LEVEL environment variable
if zapLogLevel := os.Getenv("ZAP_LOG_LEVEL"); zapLogLevel != "" {
    // Parse log level from environment variable
    if err := level.UnmarshalText([]byte(zapLogLevel)); err == nil {
        // Set log level
    } else if intLevel, err := strconv.Atoi(zapLogLevel); err == nil && intLevel >= 0 {
        // Set numeric verbosity level
    }
}
```

**Supported values**:
- `"error"` - Only error-level logs
- `"info"` - Info and error logs (default)
- `"debug"` - Debug, info, and error logs
- `"0-10"` - Numeric verbosity levels

**Additional variable**: `ZAP_DEVEL`
- `"false"` - JSON format (production, works with ELK)
- `"true"` - Console format (development)

### 2. Subscription Configuration (OLM-native method)

**Location**: Subscription resource in `openshift-operators` namespace

**Purpose**: 
- OLM-native way to configure operator environment variables
- Add `ZAP_LOG_LEVEL` and `ZAP_DEVEL` to Subscription spec.config.env
- OLM automatically propagates environment variables to the Deployment
- Persists across operator updates (OLM-managed)

**How it works**:
1. User edits Subscription to add environment variables to spec.config.env
2. OLM detects the change and updates the Deployment
3. Operator pod restarts automatically with new environment variables
4. `main.go` reads the environment variables and configures the logger

### 3. Kyverno Policy (operator-log-level-config.yaml)

**Location**: `kyverno-policies/operator-log-level-config.yaml`

**Purpose**: 
- Alternative method for policy-based configuration management
- Injects `ZAP_LOG_LEVEL` and `ZAP_DEVEL` environment variables into the operator Deployment
- Works with OLM-managed deployments
- Persists across operator updates (OLM won't overwrite Kyverno-injected env vars)

**Policy structure**:
```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: configure-operator-log-level
spec:
  rules:
    - name: inject-log-level-env
      match:
        resources:
          kinds: [Deployment]
          names: [namespace-configuration-operator-controller-manager]
          namespaces: [namespace-configuration-operator]
      mutate:
        patchStrategicMerge:
          spec:
            template:
              spec:
                containers:
                  - name: manager
                    env:
                      - name: ZAP_LOG_LEVEL
                        value: "error"  # Change this to desired level
                      - name: ZAP_DEVEL
                        value: "false"
```

**How it works**:
1. Kyverno watches for CREATE/UPDATE operations on the Deployment
2. When the Deployment is created or updated, Kyverno mutates it
3. Adds/updates the `ZAP_LOG_LEVEL` and `ZAP_DEVEL` environment variables
4. Operator pod picks up the environment variables on startup
5. `main.go` reads the environment variables and configures the logger

### 4. Configuration Methods

**Important**: For OLM-managed deployments, you have **two options**:
1. **Update the Subscription** (OLM-native method) - Recommended
2. **Use Kyverno Policy** (Policy-based injection) - Alternative

**Method 1: Update Subscription (Recommended for OLM-managed deployments)**

This is the OLM-native approach for configuring operator environment variables.

**Steps:**
1. Edit the Subscription to add environment variables:
```bash
oc edit subscription <subscription-name> -n openshift-operators
```

2. Add environment variables to the Subscription spec:
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: namespace-configuration-operator
  namespace: openshift-operators
spec:
  config:
    env:
      - name: ZAP_LOG_LEVEL
        value: "error"
      - name: ZAP_DEVEL
        value: "false"
```

3. OLM automatically propagates the environment variables to the Deployment
4. The operator pod restarts automatically

**Method 2: Kyverno Policy (Alternative for OLM-managed deployments)**

Use this method if you prefer policy-based configuration management.

**Steps:**
1. Edit `kyverno-policies/operator-log-level-config.yaml`
2. Change `ZAP_LOG_LEVEL` value to `"error"`
3. Apply: `oc apply -f kyverno-policies/operator-log-level-config.yaml`
4. Restart deployment: `oc rollout restart deployment/...`

**Method 3: Direct Deployment Edit (Manual deployments only)**

**Note**: Will be overwritten by OLM if operator is OLM-managed.

**Steps:**
- `oc set env deployment/... ZAP_LOG_LEVEL=error`
- Only use for manually deployed operators (not via OLM)

## Code Changes

### Files Modified

1. **`main.go`**
   - Added environment variable parsing for `ZAP_LOG_LEVEL`
   - Added support for numeric verbosity levels (0-10)
   - Added `ZAP_DEVEL` support for output format control

2. **`controllers/groupconfig_controller.go`**
   - Added V(1) level "skipping" logs when groups don't match any templates
   - Added V(2) level template filtering debug logs
   - Added info-level deletion tracking logs
   - Added V(1) level retry success logs

3. **`controllers/namespaceconfig_controller.go`**
   - Added V(1) level "skipping" logs when namespaces don't match any templates
   - Added V(2) level template filtering debug logs
   - Added info-level deletion tracking logs
   - Added V(1) level retry success logs

4. **`controllers/userconfig_controller.go`**
   - Added V(1) level "skipping" logs when users don't match any templates
   - Added V(2) level template filtering debug logs
   - Added info-level deletion tracking logs
   - Added V(1) level retry success logs

5. **`kyverno-policies/operator-log-level-config.yaml`** (new)
   - ClusterPolicy to inject log level environment variables (alternative method)
   - Works with OLM-managed deployments
   - Includes documentation comments
   - **Note**: Users should either update Subscription OR use Kyverno policy

6. **`resolved-issues-tracker/resolved-issues-tracker.md`**
   - Documented issue #134 resolution
   - Added reference to GitHub issue
   - Documented all logging enhancements

### Files Created

1. **`examples/test-and-logic/ISSUE-134-ROOT-CAUSE-SUMMARY.md`**
   - Problem description
   - Root cause analysis
   - Solution approach

2. **`examples/test-and-logic/ISSUE-134-VERIFICATION-GUIDE.md`**
   - Step-by-step verification instructions
   - Configuration methods
   - Troubleshooting guide

3. **`examples/test-and-logic/ISSUE-134-FIX-IMPLEMENTATION.md`** (this file)
   - Implementation details
   - Code changes
   - Configuration methods

## How to Use

### Set log level to "error" (minimal logging)

**Option 1: Update Subscription (Recommended for OLM-managed deployments)**

```bash
# 1. Get the subscription name
oc get subscription -n openshift-operators | grep namespace-configuration-operator

# 2. Edit the subscription
oc edit subscription <subscription-name> -n openshift-operators

# 3. Add environment variables to spec.config.env:
#    spec:
#      config:
#        env:
#          - name: ZAP_LOG_LEVEL
#            value: "error"
#          - name: ZAP_DEVEL
#            value: "false"

# 4. OLM will automatically update the deployment
#    No manual restart needed - OLM handles it
```

**Option 2: Use Kyverno Policy (Alternative for OLM-managed deployments)**

```bash
# 1. Edit the policy file
oc edit clusterpolicy configure-operator-log-level

# 2. Change ZAP_LOG_LEVEL value to "error":
#    - name: ZAP_LOG_LEVEL
#      value: "error"

# 3. Restart deployment
oc rollout restart deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator
```

**Or patch Kyverno policy directly:**
```bash
oc patch clusterpolicy configure-operator-log-level --type='json' -p='[
  {
    "op": "replace",
    "path": "/spec/rules/0/mutate/patchStrategicMerge/spec/template/spec/containers/0/env/0/value",
    "value": "error"
  }
]'

oc rollout restart deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator
```

### Verify it's working

```bash
# Check environment variable
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo

# Check logs (should be minimal)
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=100
```

## Benefits

1. **Reduced log volume**: Setting log level to "error" significantly reduces log entries sent to ELK
2. **OLM-compatible**: Kyverno policy works with OLM-managed deployments
3. **Persistent**: Configuration persists across operator updates
4. **Flexible**: Supports multiple log levels (error, info, debug, numeric)
5. **Production-ready**: JSON format works seamlessly with ELK and other log aggregation systems
6. **Enhanced visibility**: V(1) skipping logs provide clear visibility into why resources are skipped
7. **Better debugging**: V(2) template filtering logs help troubleshoot template matching issues
8. **Deletion tracking**: Info-level logs track resource deletion lifecycle for audit purposes
9. **Retry visibility**: V(1) retry success logs help distinguish retries from errors in centralized logging
10. **Structured logging**: All logs use structured JSON format for easy parsing and filtering in ELK

## Testing

### Test 1: Verify environment variable is set
```bash
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo
```

### Test 2: Verify log output is minimal
```bash
# With error level, should see mostly errors
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=1000 | \
  grep -o '"level":"[^"]*"' | sort | uniq -c
```

### Test 3: Verify configuration persists
```bash
# Restart deployment
oc rollout restart deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator

# Verify log level is still set
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo
```

## Enhanced Logging Features

### 1. V(1) Level Skipping Logs

**Purpose**: Provide clear visibility when resources are skipped because no templates match

**Implementation**: Added to all three controllers (`groupconfig_controller.go`, `namespaceconfig_controller.go`, `userconfig_controller.go`)

**Log format**:
```json
{"level":"debug","msg":"skipping group - no GroupConfig templates match the group pattern","group":"app-ocp-rbac-platform-cluster-admin","groupconfig":"cluster-audit-groupconfig-rbac"}
```

**Visibility**: Requires `ZAP_LOG_LEVEL=1` or higher

**Benefits**:
- Clear explanation of why resources are skipped
- Includes resource name and CR name for context
- Helps identify groups/namespaces/users that need templates

### 2. V(2) Level Template Filtering Logs

**Purpose**: Detailed debug logs for template matching and pattern evaluation

**Implementation**: Already existed, enhanced with better pattern extraction

**Log format**:
```json
{"level":"Level(-2)","msg":"checking template applicability","group":"app-ocp-rbac-alpha-cluster-admin","suffixPatterns":["-cluster-admin"],"containsPatterns":[]}
{"level":"Level(-2)","msg":"group matches hasSuffix pattern","group":"app-ocp-rbac-alpha-cluster-admin","pattern":"-cluster-admin"}
```

**Visibility**: Requires `ZAP_LOG_LEVEL=2` or higher

**Benefits**:
- Shows which patterns are being checked
- Explains why groups match or don't match
- Helps troubleshoot template filtering issues

### 3. Info-Level Deletion Tracking Logs

**Purpose**: Track resource deletion lifecycle for audit and troubleshooting

**Implementation**: Added to all three controllers

**Log formats**:
```json
{"level":"info","msg":"resource deletion detected - resource not found, skipping reconciliation","groupconfig":{"name":"test-groupconfig"}}
{"level":"info","msg":"resource deletion detected - processing deletion cleanup","groupconfig":"test-groupconfig","deletionTimestamp":"2025-12-10T05:11:57Z"}
{"level":"info","msg":"resource deletion completed successfully","groupconfig":"test-groupconfig"}
```

**Visibility**: Always visible (info level)

**Benefits**:
- Clear audit trail of resource deletions
- Helps prevent false positives in centralized logging
- Shows deletion lifecycle stages

### 4. V(1) Level Retry Success Logs

**Purpose**: Log when operations succeed after retries to distinguish from errors

**Implementation**: Added to `manageSuccessWithRetry` function in all three controllers

**Log format**:
```json
{"level":"Level(-1)","msg":"ManageSuccess succeeded after retry","attempt":2,"groupconfig":"test-groupconfig"}
```

**Visibility**: Requires `ZAP_LOG_LEVEL=1` or higher

**Benefits**:
- Distinguishes successful retries from actual errors
- Prevents false positives in centralized logging systems
- Shows retry attempts and success

## Related Documentation
- [Issue #134 Root Cause Summary](./ISSUE-134-ROOT-CAUSE-SUMMARY.md)
- [Issue #134 Verification Guide](./ISSUE-134-VERIFICATION-GUIDE.md)
- [Template Filtering Logs Explanation](../../docs/TEMPLATE_FILTERING_LOGS_EXPLANATION.md)
- [Kyverno Policies README](../../kyverno-policies/README.md)
- [Resolved Issues Tracker](../../resolved-issues-tracker/resolved-issues-tracker.md)
