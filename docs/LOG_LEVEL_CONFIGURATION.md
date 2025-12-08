# Log Level Configuration for OLM-Deployed Operators

## ⚠️ Important: OLM Deployment Constraints

**This operator is deployed via OLM (Operator Lifecycle Manager).** Any direct modifications to the Deployment will be **automatically reverted or rejected** by OLM.

**Valid configuration methods:**
1. ✅ **Operator Subscription** - Environment variables in `Subscription.spec.config.env`
2. ✅ **Kyverno Policies** - Mutate the Deployment via policy

**Invalid methods (will be reverted):**
- ❌ Direct Deployment edits (`oc edit deployment`, `oc patch deployment`)
- ❌ ConfigMap references in Deployment (OLM manages the Deployment spec)
- ❌ Manual environment variable injection via `oc set env`

## Environment Variables

### `ZAP_LOG_LEVEL`

Controls the verbosity of logging.

**Valid Values:**
- `error` - Only error messages
- `info` - Info level and above (recommended for production)
- `debug` - Debug level and above (shows template filtering logs)
- `0-10` - Integer levels (higher = more verbose)
  - `0` = error
  - `1` = info
  - `2` = debug (shows template filtering debug logs)
  - `3+` = even more verbose

**Default:** `debug` (when `ZAP_DEVEL=true`)

### `ZAP_DEVEL`

Controls development mode (affects log format and default verbosity).

**Valid Values:**
- `true` or `1` - Development mode (console format, debug level default)
- `false` or `0` - Production mode (JSON format, info level default)

**Default:** `true`

## Configuration Methods

### Method 1: Operator Subscription (Recommended for OLM)

Configure log levels via the Subscription resource. OLM will propagate these environment variables to the operator Deployment.

**Find your Subscription:**
```bash
oc get subscription -A | grep namespace-configuration-operator
```

**Update Subscription with log level configuration:**
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: namespace-configuration-operator
  namespace: openshift-operators  # or your operator namespace
spec:
  channel: alpha
  name: namespace-configuration-operator
  source: community-operators
  sourceNamespace: openshift-marketplace
  config:
    env:
    - name: ZAP_LOG_LEVEL
      value: "info"  # Production: "info", Debug: "debug" or "2"
    - name: ZAP_DEVEL
      value: "false"  # Production: "false", Development: "true"
```

**Apply via CLI:**
```bash
# Edit the subscription
oc edit subscription namespace-configuration-operator -n openshift-operators

# Or patch it
oc patch subscription namespace-configuration-operator -n openshift-operators --type='merge' -p='
spec:
  config:
    env:
    - name: ZAP_LOG_LEVEL
      value: "info"
    - name: ZAP_DEVEL
      value: "false"
'
```

**Verify configuration:**
```bash
# Check Subscription config
oc get subscription namespace-configuration-operator -n openshift-operators -o jsonpath='{.spec.config.env}'

# Check if environment variables are in the Deployment (OLM should propagate them)
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator -o jsonpath='{.spec.template.spec.containers[0].env}' | jq
```

### Method 2: Kyverno Policy (Alternative for OLM)

Use a Kyverno ClusterPolicy to mutate the operator Deployment and inject log level environment variables. This works even with OLM-managed deployments.

**Create Kyverno policy:**
```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: configure-operator-log-level
  annotations:
    policies.kyverno.io/title: Configure Namespace Configuration Operator Log Level
    policies.kyverno.io/category: Operator Configuration
    policies.kyverno.io/severity: low
spec:
  background: false
  rules:
    - name: inject-log-level-env
      match:
        any:
          - resources:
              kinds:
                - Deployment
              names:
                - namespace-configuration-operator-controller-manager
              namespaces:
                - namespace-configuration-operator
              operations:
                - CREATE
                - UPDATE
      mutate:
        patchStrategicMerge:
          spec:
            template:
              spec:
                containers:
                  - name: manager
                    env:
                      - name: ZAP_LOG_LEVEL
                        value: "info"  # Change to "debug" or "2" for verbose logs
                      - name: ZAP_DEVEL
                        value: "false"  # Change to "true" for console format
```

**Apply the policy:**
```bash
oc apply -f operator-log-level-policy.yaml
```

**Note:** Kyverno will inject these environment variables whenever the Deployment is created or updated by OLM, ensuring the configuration persists.

## Recommended Configurations

### Production (Default)
```yaml
env:
- name: ZAP_LOG_LEVEL
  value: "info"
- name: ZAP_DEVEL
  value: "false"
```
**Results:**
- ✅ JSON formatted logs (production-ready)
- ✅ Info level only (no debug noise)
- ✅ Template filtering debug logs hidden (V(2) not shown)
- ✅ Clean, structured logs for log aggregation systems

### Production Debugging (Template Filtering Visibility)
```yaml
env:
- name: ZAP_LOG_LEVEL
  value: "2"  # Verbosity level 2 shows template filtering logs
- name: ZAP_DEVEL
  value: "false"  # Keep JSON format
```
**Results:**
- ✅ JSON formatted logs (log aggregation compatible)
- ✅ Shows template filtering debug logs (Level(-2) in output)
- ✅ Verbosity level 2 enables V(2) debug statements
- ✅ Use when troubleshooting template matching issues

### Development/Local Testing
```yaml
env:
- name: ZAP_LOG_LEVEL
  value: "info"  # or "debug"
- name: ZAP_DEVEL
  value: "true"
```
**Results:**
- ✅ Console formatted logs (human-readable)
- ✅ Easier to read during local development
- ✅ Template filtering debug logs hidden at info level
- ✅ Use for local operator development

### Debug Level Testing
```yaml
env:
- name: ZAP_LOG_LEVEL
  value: "debug"
- name: ZAP_DEVEL
  value: "false"
```
**Results:**
- ✅ JSON formatted logs
- ✅ Debug level (more verbose than info)
- ⚠️ Template filtering logs still require verbosity level 2 or higher

## Template Filtering Debug Logs

Template filtering debug logs use verbosity level `V(2)`, so they only appear when:
- `ZAP_LOG_LEVEL=2` or higher
- `ZAP_LOG_LEVEL=debug`
- `ZAP_DEVEL=true` (development mode shows debug by default)

These logs show:
- Which groups are being checked against templates
- Extracted patterns (hasSuffix, contains)
- Match/no-match decisions
- Template previews

## Dockerfile Defaults

The Dockerfile sets default environment variables that can be overridden at runtime:

```dockerfile
ENV ZAP_LOG_LEVEL=info
ENV ZAP_DEVEL=false
```

**Why set defaults in Dockerfile?**
- Provides sensible production defaults
- Can be overridden via Subscription `config.env` or Kyverno policy
- Ensures consistent behavior if not explicitly configured

**Priority (highest to lowest):**
1. Command-line flags (`--zap-log-level`, `--zap-devel`) - if supported
2. Subscription/Kyverno environment variables
3. Dockerfile ENV defaults

## Verification

**Check Subscription configuration:**
```bash
oc get subscription namespace-configuration-operator -n openshift-operators -o yaml | grep -A 5 "config:"
```

**Check Deployment environment variables:**
```bash
oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator -o jsonpath='{.spec.template.spec.containers[0].env}' | jq
```

**Check current log output:**
```bash
oc logs deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator | head -10
```

**Expected output:**
- If `ZAP_DEVEL=false`: JSON formatted logs
- If `ZAP_LOG_LEVEL=info`: No template filtering debug messages
- If `ZAP_LOG_LEVEL=2`: Template filtering debug logs visible

## Troubleshooting

### Configuration Not Applied

**Problem:** Log level changes aren't taking effect.

**Solutions:**
1. **Verify Subscription config:**
   ```bash
   oc get subscription namespace-configuration-operator -n openshift-operators -o yaml
   ```
   Ensure `spec.config.env` contains your environment variables.

2. **Check if OLM propagated the config:**
   ```bash
   oc get deployment namespace-configuration-operator-controller-manager -n namespace-configuration-operator -o yaml | grep -A 10 "env:"
   ```

3. **Restart the operator pod:**
   ```bash
   oc delete pod -l control-plane=controller-manager -n namespace-configuration-operator
   ```

4. **Check Kyverno policy (if using):**
   ```bash
   oc get cpol configure-operator-log-level -o yaml
   oc get policyreport -A | grep configure-operator-log-level
   ```

### OLM Reverting Changes

**Problem:** Direct Deployment edits are being reverted.

**Solution:** This is expected behavior. Use Subscription configuration or Kyverno policies instead. OLM manages the Deployment and will revert any manual changes.

### Logs Still Too Verbose

**Problem:** Even with `ZAP_LOG_LEVEL=info`, logs are too verbose.

**Solution:** Ensure `ZAP_DEVEL=false` is set. Development mode (`ZAP_DEVEL=true`) defaults to debug level regardless of `ZAP_LOG_LEVEL`.

## Example: Changing Log Level in Production

**Scenario:** Need to enable template filtering debug logs temporarily.

**Step 1: Update Subscription**
```bash
oc patch subscription namespace-configuration-operator -n openshift-operators --type='merge' -p='
spec:
  config:
    env:
    - name: ZAP_LOG_LEVEL
      value: "2"
    - name: ZAP_DEVEL
      value: "false"
'
```

**Step 2: Wait for OLM to update Deployment**
```bash
# Watch for pod restart
oc get pods -n namespace-configuration-operator -w
```

**Step 3: Verify logs**
```bash
oc logs deployment/namespace-configuration-operator-controller-manager -n namespace-configuration-operator | grep -i "template"
```

**Step 4: Revert to production settings**
```bash
oc patch subscription namespace-configuration-operator -n openshift-operators --type='merge' -p='
spec:
  config:
    env:
    - name: ZAP_LOG_LEVEL
      value: "info"
    - name: ZAP_DEVEL
      value: "false"
'
```
