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
- `0-10` - Integer levels (higher = more verbose). `main.go` maps an integer N to `zapcore.Level(-N)`:
  - `0` = info
  - `1` = debug (also shows the template filter's `skipping namespace/group/user` lines, logged at V(1))
  - `2` = the template filter's per-decision lines (`template applicability decided ...`, V(2))
  - `3+` = even more verbose
  - the `error` level is only reachable by name (`ZAP_LOG_LEVEL=error`); no integer maps to it

**Default:** `debug` (when `ZAP_DEVEL=true`)

### `ZAP_DEVEL`

Controls development mode (affects log format and default verbosity).

**Valid Values:**
- `true` or `1` - Development mode (console format, debug level default)
- `false` or `0` - Production mode (JSON format, info level default)

**Default:** `true`

## Configuration Methods

**Important:** For OLM-managed deployments, you have **two options** to change log levels:
1. **Update Subscription** (OLM-native method, recommended)
2. **Use Kyverno Policy** (Policy-based method, alternative)

Both methods work with OLM-managed deployments and persist across operator updates. Choose one method based on your preference.

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

**When to use this method:**
- You prefer policy-based configuration management
- You want centralized configuration via GitOps
- You're already using Kyverno for other operator configurations
- You want to apply the same log level configuration across multiple clusters

**Note:** If you're using Subscription configuration (Method 1), you don't need Kyverno policy. Choose one method.

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

## Configuration Method Comparison

| Method | OLM-Managed | Manual Deployment | Persists Across Updates | Requires | Best For |
|--------|-------------|-------------------|------------------------|----------|----------|
| **Subscription** | ✅ Yes | ❌ No | ✅ Yes | OLM | OLM-native configuration |
| **Kyverno Policy** | ✅ Yes | ✅ Yes | ✅ Yes | Kyverno | Policy-based/GitOps management |
| **Dockerfile ENV** | ⚠️ Fallback only | ✅ Yes | ❌ No | None | Defaults only (not recommended for OLM) |

**Recommendation:**
- **For OLM-managed deployments**: Use **Method 1 (Subscription)** - it's the OLM-native approach
- **For policy-based management**: Use **Method 2 (Kyverno Policy)** - useful for centralized configuration
- **For manual deployments**: Dockerfile ENV defaults work, but can be overridden via Deployment spec

## Recommended Configurations

### Production (Default) - Minimal Logging

**Use case:** Reduce log volume sent to ELK/centralized logging systems.

**Configuration:**
```yaml
# For Subscription (Method 1)
spec:
  config:
    env:
    - name: ZAP_LOG_LEVEL
      value: "error"  # Only errors (minimal logging)
    - name: ZAP_DEVEL
      value: "false"   # JSON format

# For Kyverno Policy (Method 2)
env:
- name: ZAP_LOG_LEVEL
  value: "error"
- name: ZAP_DEVEL
  value: "false"
```

**Results:**
- ✅ Minimal log volume (only errors)
- ✅ JSON formatted logs (production-ready)
- ✅ Significantly reduces ELK log ingestion
- ✅ No info/debug noise

### Production (Normal Operations)

**Use case:** Standard production logging with normal operations visibility.

**Configuration:**
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
- ✅ Includes deletion tracking and resource lifecycle events

### Production Debugging (Template Filtering Visibility)

**Use case:** Troubleshooting template matching issues while maintaining JSON format for log aggregation.

**Configuration:**
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
- ✅ Shows skipping logs (V(1)) and retry success logs (V(1))
- ✅ Use when troubleshooting template matching issues

### Development/Local Testing

**Use case:** Local operator development with human-readable console logs.

**Configuration:**
```yaml
env:
- name: ZAP_LOG_LEVEL
  value: "info"  # or "debug" or "2" for template filtering
- name: ZAP_DEVEL
  value: "true"  # Console format
```

**Results:**
- ✅ Console formatted logs (human-readable)
- ✅ Easier to read during local development
- ✅ Template filtering debug logs hidden at info level (use "2" to show them)
- ✅ Use for local operator development
- ⚠️ Not recommended for production (console format not ideal for log aggregation)

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

## Template Filter Logs

The template filter (`controllers/common/templatefilter.go`) logs at two verbosities:

- `V(1)` (`ZAP_LOG_LEVEL=1` or `debug`): `skipping namespace - no NamespaceConfig templates match the
  namespace pattern` (and the group/user equivalents) once per rejected object per reconcile;
  `template does not parse, leaving it to the renderer`; `template applicability could not be decided by
  rendering, leaving it to the renderer`.
- `V(2)` (`ZAP_LOG_LEVEL=2`): one line per template per object saying how the decision was made,
  `template applicability decided statically` or `template applicability decided by rendering`, with
  `applicable=true|false` and a preview of the template.

See `docs/TEMPLATE_FILTERING_LOGS_EXPLANATION.md` for what each line means.

## Dockerfile Defaults

The Dockerfile sets default environment variables for log configuration:

```dockerfile
ENV ZAP_LOG_LEVEL=info
ENV ZAP_DEVEL=false
```

**Why set defaults in Dockerfile?**
- Provides sensible production defaults (info level, JSON format)
- Can be overridden at runtime via Subscription `config.env` or Kyverno policy
- Ensures consistent behavior if not explicitly configured
- Follows Operator SDK best practices for logging configuration

**Configuration Priority (highest to lowest):**
1. **Subscription/Kyverno environment variables** - Runtime configuration (recommended)
2. **Dockerfile ENV defaults** - Fallback if not explicitly configured
3. **Operator SDK defaults** - Built-in defaults (debug level if ZAP_DEVEL=true)

**Important:** For OLM-managed deployments, always use Subscription or Kyverno policy to configure log levels. The Dockerfile defaults serve as a fallback but should be overridden for production use.

**For detailed information about Dockerfile enhancements (version info, build args, etc.), see [DOCKERFILE_ENHANCEMENTS.md](./DOCKERFILE_ENHANCEMENTS.md).**

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
