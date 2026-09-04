# Issue #134 — Verification & Configuration Guide

## What this verifies
- That the operator log level can be configured to "error" to reduce log volume sent to ELK
- That the configuration persists across operator updates (OLM-compatible)
- That log level changes take effect immediately after deployment update

## Prerequisites
- `oc` or `kubectl` access to a cluster with the operator deployed
- Kyverno installed in the cluster (for policy-based configuration)
- Operator deployed via OLM or manually

## Configuration methods

**Important**: For OLM-managed deployments, you have **two options**:
1. **Update the Subscription** (OLM-native method) - Recommended for OLM deployments
2. **Use Kyverno Policy** (Policy-based injection) - Alternative method that works with OLM

### Method 1: Update Subscription (Recommended for OLM-managed deployments)

**Why this method:**
- OLM-native approach
- Persists across operator updates
- Standard OLM configuration method
- No additional dependencies (Kyverno not required)

**Steps:**

1. **Edit the Subscription to add environment variables**:
```bash
# Get the subscription name
oc get subscription -n openshift-operators | grep namespace-configuration-operator

# Edit the subscription
oc edit subscription <subscription-name> -n openshift-operators
```

2. **Add environment variables to the Subscription spec**:
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: namespace-configuration-operator
  namespace: openshift-operators
spec:
  # ... existing spec ...
  config:
    env:
      - name: ZAP_LOG_LEVEL
        value: "error"
      - name: ZAP_DEVEL
        value: "false"
```

3. **OLM will automatically update the Deployment**:
   - OLM will propagate the environment variables to the Deployment
   - The operator pod will restart automatically
   - No manual restart needed

4. **Verify the configuration**:
```bash
# Check environment variable is set in the deployment
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo

# Expected output: error
```

5. **Verify logs are minimal**:
```bash
# Wait for pod to be ready
oc wait --for=condition=ready pod -n namespace-configuration-operator \
  -l control-plane=controller-manager --timeout=60s

# Check logs (should be minimal, mostly errors)
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=100

# Count log entries by level
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=1000 | \
  grep -o '"level":"[^"]*"' | sort | uniq -c
```

### Method 2: Kyverno Policy (Alternative for OLM-managed deployments)

**Why this method:**
- Works with OLM-managed deployments
- Persists across operator updates
- Centralized configuration management via policy
- Useful when you want policy-based configuration management

**Steps:**

1. **Apply the Kyverno policy**:
```bash
oc apply -f kyverno-policies/operator-log-level-config.yaml
```

2. **Update the policy to set log level to "error"**:
```bash
# Edit the policy file
oc edit clusterpolicy configure-operator-log-level

# Change the ZAP_LOG_LEVEL value from "2" to "error":
#   - name: ZAP_LOG_LEVEL
#     value: "error"
```

Or patch directly:
```bash
oc patch clusterpolicy configure-operator-log-level --type='json' -p='[
  {
    "op": "replace",
    "path": "/spec/rules/0/mutate/patchStrategicMerge/spec/template/spec/containers/0/env/0/value",
    "value": "error"
  }
]'
```

3. **Trigger policy application** (restart deployment):
```bash
oc rollout restart deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator
```

4. **Verify the configuration**:
```bash
# Check environment variable is set
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo

# Expected output: error
```

5. **Verify logs are minimal**:
```bash
# Wait for pod to be ready
oc wait --for=condition=ready pod -n namespace-configuration-operator \
  -l control-plane=controller-manager --timeout=60s

# Check logs (should be minimal, mostly errors)
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=100

# Count log entries by level
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=1000 | \
  grep -o '"level":"[^"]*"' | sort | uniq -c
```

### Method 3: Direct Deployment Edit (Manual deployments only)

**Note**: 
- This method will be **overwritten by OLM** if the operator is OLM-managed
- Only use this method for manually deployed operators (not via OLM)

**Steps:**

1. **Edit the deployment**:
```bash
oc set env deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  ZAP_LOG_LEVEL=error
```

2. **Verify**:
```bash
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo
```

## Configuration Method Summary

| Method | OLM-Managed | Manual Deployment | Persists Across Updates | Requires |
|--------|-------------|-------------------|------------------------|----------|
| **Subscription** | ✅ Yes | ❌ No | ✅ Yes | OLM |
| **Kyverno Policy** | ✅ Yes | ✅ Yes | ✅ Yes | Kyverno |
| **Direct Deployment Edit** | ❌ No (overwritten) | ✅ Yes | ❌ No | None |

**Recommendation**:
- **For OLM-managed deployments**: Use **Method 1 (Subscription)** - it's the OLM-native approach
- **For policy-based management**: Use **Method 2 (Kyverno Policy)** - useful for centralized configuration
- **For manual deployments**: Use **Method 3 (Direct Edit)** - only if not using OLM

## Verification test steps

### Test 1: Verify log level is set to "error"

```bash
# Check environment variable
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo

# Expected: error
```

### Test 2: Verify log output is minimal

```bash
# Get recent logs
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=100

# Count log entries by level
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=1000 | \
  grep -o '"level":"[^"]*"' | sort | uniq -c

# With error level, you should see:
# - Mostly "error" level messages
# - Very few or no "info" or "debug" messages
# - No V(1) or V(2) level messages
```

### Test 2b: Verify enhanced logging features (with log level 1 or 2)

**With log level 1 (`ZAP_LOG_LEVEL=1`):**
```bash
# Check for skipping logs
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=500 | \
  grep -i "skipping" | head -10

# Check for retry success logs
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=500 | \
  grep "succeeded after retry" | head -5
```

**With log level 2 (`ZAP_LOG_LEVEL=2`):**
```bash
# Check for template filtering logs
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=500 | \
  grep "checking template applicability" | head -10

# Check for pattern matching logs
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=500 | \
  grep -E "(matches|does not match)" | head -10
```

### Test 3: Verify configuration persists after operator update

```bash
# Simulate operator update by restarting
oc rollout restart deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator

# Wait for rollout
oc rollout status deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator --timeout=120s

# Verify log level is still set
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo

# Expected: error (should persist)
```

### Test 4: Compare log volume before/after

**Before (with default/info level):**
```bash
# Count total log entries in last 1000 lines
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=1000 | wc -l

# Count by level
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=1000 | \
  grep -o '"level":"[^"]*"' | sort | uniq -c
```

**After (with error level):**
```bash
# Count total log entries in last 1000 lines (should be much lower)
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=1000 | wc -l

# Count by level (should be mostly errors)
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=1000 | \
  grep -o '"level":"[^"]*"' | sort | uniq -c
```

## Expected results

### With log level set to "error":
- ✅ Environment variable `ZAP_LOG_LEVEL=error` is set in the deployment
- ✅ Log output is minimal (only error-level messages)
- ✅ Log volume sent to ELK is significantly reduced
- ✅ Configuration persists across operator updates (if using Kyverno policy)

### Log level comparison:

| Log Level | Shows | Use Case |
|-----------|-------|----------|
| `error` | Only errors | Production (minimal logging, reduces ELK volume) |
| `info` | Info and errors | Production (normal operations, includes deletion tracking) |
| `1` or `debug` | V(1) + info + errors | Development (shows skipping logs, retry success) |
| `2` | V(2) + V(1) + info + errors | Troubleshooting (shows template filtering details) |

**Log Level Breakdown**:
- **Error level**: Only actual errors
- **Info level**: Includes deletion tracking, resource lifecycle events
- **V(1) level**: Includes skipping logs, retry success logs
- **V(2) level**: Includes template filtering debug logs

## Troubleshooting

### Issue: Log level not taking effect

**Check 1: Verify environment variable is set**
```bash
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo
```

**Check 2: Verify pod has the environment variable**
```bash
POD=$(oc get pods -n namespace-configuration-operator -l control-plane=controller-manager -o jsonpath='{.items[0].metadata.name}')
oc exec -n namespace-configuration-operator $POD -- env | grep ZAP_LOG_LEVEL
```

**Check 3: Restart the deployment**
```bash
oc rollout restart deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator
```

### Issue: Kyverno policy not applying

**Check 1: Verify Kyverno is installed**
```bash
oc get pods -n kyverno
```

**Check 2: Check policy status**
```bash
oc get clusterpolicy configure-operator-log-level -o yaml
```

**Check 3: Check policy violations/events**
```bash
oc get events -n namespace-configuration-operator --sort-by='.lastTimestamp' | grep -i kyverno
```

## Related documentation
- [Kyverno Policies README](../../kyverno-policies/README.md)
- [Log Level Configuration](../../docs/LOG_LEVEL_CONFIGURATION.md) (if exists)
- [Resolved Issues Tracker](../../resolved-issues-tracker/resolved-issues-tracker.md)
