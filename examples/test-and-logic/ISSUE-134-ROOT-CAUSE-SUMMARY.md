# Issue #134 — Root Cause Summary

## Problem
- The operator was creating lots of Info-level logs that were being sent to ELK (hosted in AWS) via OpenShift LogForwarder
- This caused excessive log volume and potential cost/performance issues
- Users needed a way to set the log level to "error" to reduce log noise
- Question: Is it ConfigMap or environment variable?

## How this was identified
1. **Issue reported**: https://github.com/redhat-cop/namespace-configuration-operator/issues/134
2. **Problem**: Operator generating excessive Info logs sent to centralized logging (ELK)
3. **Need**: Ability to configure log level to "error" to reduce log volume

## Root cause analysis
- **Operator uses zap logger**: The operator uses the `zap` structured logging library
- **Default log level**: Operator was running with default log level (info), which includes:
  - Info-level messages (normal operations)
  - Debug-level messages (template filtering, reconciliation details)
  - Error-level messages (actual errors)
- **No persistent configuration**: Log level was not easily configurable for OLM-managed deployments
- **Environment variable support existed**: `ZAP_LOG_LEVEL` and `ZAP_DEVEL` were supported in `main.go`, but:
  - Not documented clearly
  - Not easily configurable for OLM-managed deployments
  - Would be overwritten when OLM updates the Deployment

## Solution approach
- **Environment variables**: Use `ZAP_LOG_LEVEL` environment variable to control log level
- **Two configuration methods for OLM-managed deployments**:
  1. **Update Subscription** (OLM-native method) - Add environment variables to Subscription spec.config.env
  2. **Kyverno policy** (Policy-based method) - Create a ClusterPolicy that injects log level environment variables into the Deployment
- **OLM-compatible**: Both methods work with OLM-managed deployments and persist across updates
- **Flexible configuration**: Supports "error", "info", "debug", or numeric levels (0-10)
- **Enhanced logging features**:
  - V(1) level logging for skipped resources (groups/namespaces/users that don't match templates)
  - V(2) level logging for template filtering details (debug-level template matching)
  - Deletion tracking logs (info-level) for resource deletion lifecycle
  - Retry success logs (V(1)) for optimistic concurrency conflict resolution
  - Structured JSON logging format for ELK integration

## Key findings
- **Log level options**:
  - `"error"` = only errors (minimal logging, reduces ELK volume)
  - `"info"` = info and above (recommended for production)
  - `"debug"` = debug and above (development)
  - `"0-10"` = numeric verbosity levels (e.g., "2" shows template filtering logs)
- **Format control**: `ZAP_DEVEL` controls output format:
  - `"false"` = JSON format (production, works with ELK)
  - `"true"` = console format (development)
- **Configuration methods**: For OLM-managed deployments, users should **either**:
  1. **Update Subscription** (OLM-native method, recommended) - Add environment variables to Subscription spec.config.env
  2. **Use Kyverno policy** (Policy-based method, alternative) - ClusterPolicy injects environment variables into Deployment
- **Enhanced logging features added**:
  - **V(1) skipping logs**: Clear messages when resources are skipped because no templates match
    - Format: `"skipping group - no GroupConfig templates match the group pattern"`
    - Visible with `ZAP_LOG_LEVEL=1` or higher
  - **V(2) template filtering logs**: Detailed debug logs for template matching
    - Shows which patterns are checked and why groups match/don't match
    - Visible with `ZAP_LOG_LEVEL=2` or higher
  - **Info-level deletion tracking**: Logs for resource deletion lifecycle
    - Detection, processing, and completion messages
    - Always visible (info level)
  - **V(1) retry success logs**: Logs when operations succeed after retries
    - Helps distinguish retries from actual errors in centralized logging
    - Visible with `ZAP_LOG_LEVEL=1` or higher

## Conclusion
- The issue was not a bug, but a missing configuration mechanism
- **Solution**: Users can configure log level in two ways for OLM-managed deployments:
  1. **Update Subscription** (OLM-native, recommended) - Add `ZAP_LOG_LEVEL=error` to Subscription spec.config.env
  2. **Use Kyverno policy** (Alternative) - ClusterPolicy injects `ZAP_LOG_LEVEL=error` into the operator Deployment
- This allows users to reduce log volume by setting log level to "error"
- Both methods work with OLM-managed deployments and persist across operator updates

## Key commands used to verify the solution
```bash
# 1) Check if ZAP_LOG_LEVEL is supported in main.go
grep -A 10 "ZAP_LOG_LEVEL" main.go

# 2) Verify Kyverno policy exists
ls kyverno-policies/operator-log-level-config.yaml

# 3) Check current log level in deployment
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}'

# 4) Verify logs are at error level (should see minimal output)
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=100 | grep -v '"level":"error"'
```

## Minimal verification commands
```bash
# Check current log level
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ZAP_LOG_LEVEL")].value}' && echo

# View recent logs (with error level, should be minimal)
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=50

# Count log entries by level (with error level, should be mostly errors)
oc logs -n namespace-configuration-operator deployment/namespace-configuration-operator-controller-manager --tail=1000 | \
  grep -o '"level":"[^"]*"' | sort | uniq -c
```
