# Kyverno Policy Templates

This directory contains both **template files** (`.tpl`) and **ready-to-use policy files** (`.yaml`).

## Template Files (env-*.yaml.tpl)

Template files use `${DOCKERHUB_USERNAME}` placeholders that can be replaced using `envsubst`.

**Available Templates:**
- `env-replace-operator-image-to-dockerhub.yaml.tpl` - Operator image replacement template (uses `${DOCKERHUB_USERNAME}`)
- `env-dockerhub-image-replacement.yaml.tpl` - Quay.io to Docker Hub redirection template (uses `${DOCKERHUB_USERNAME}`)
- `env-operator-log-level-config.yaml.tpl` - Log level configuration template (uses `${ZAP_LOG_LEVEL}`, `${ZAP_DEVEL}`)

**Note**: The `internal-registry-image-replacement.yaml` policy does **not** need a template because:
- OpenShift internal registry URL is standardized: `image-registry.openshift-image-registry.svc.cluster.local:5000`
- Operator namespace is fixed: `namespace-configuration-operator`
- Image name is fixed: `namespace-configuration-operator`

These values never change, so the policy can be used directly without variable substitution.

## Generating Policies from Templates

### Method 1: Using the Helper Script (Recommended)

```bash
# Set your Docker Hub username
export DOCKERHUB_USERNAME=your-username

# Generate all policies (run from repository root)
./local-utilities/generate-policies.sh

# Or pass username as argument
./local-utilities/generate-policies.sh your-username
```

The script processes all `env-*.yaml.tpl` files and replaces environment variable placeholders:
- `${DOCKERHUB_USERNAME}` - Docker Hub username (required)
- `${ZAP_LOG_LEVEL}` - Log level (optional: error, info, debug, 0-10)
- `${ZAP_DEVEL}` - Development mode (optional: true/false)

Generated `.yaml` files are created without the `env-` prefix and `.tpl` extension.

### Method 2: Manual envsubst

```bash
# Set your Docker Hub username
export DOCKERHUB_USERNAME=your-username

# Optional: Set log level configuration
export ZAP_LOG_LEVEL=info
export ZAP_DEVEL=false

# Generate a specific policy (run from repository root)
envsubst < kyverno-policies/env-replace-operator-image-to-dockerhub.yaml.tpl > kyverno-policies/replace-operator-image-to-dockerhub.yaml
envsubst < kyverno-policies/env-dockerhub-image-replacement.yaml.tpl > kyverno-policies/dockerhub-image-replacement.yaml
envsubst < kyverno-policies/env-operator-log-level-config.yaml.tpl > kyverno-policies/operator-log-level-config.yaml
```

### Method 3: Using sed (Alternative)

```bash
# Replace placeholder in template files (run from repository root)
sed 's/\${DOCKERHUB_USERNAME}/your-username/g' kyverno-policies/env-replace-operator-image-to-dockerhub.yaml.tpl > kyverno-policies/replace-operator-image-to-dockerhub.yaml
```

## Applying Generated Policies

After generating the policies:

```bash
# Apply a specific policy (run from repository root)
oc apply -f kyverno-policies/replace-operator-image-to-dockerhub.yaml

# Apply all generated policies
oc apply -f kyverno-policies/dockerhub-image-replacement.yaml
oc apply -f kyverno-policies/replace-operator-image-to-dockerhub.yaml
```

## Current Cluster Policies

Check what's currently deployed:

```bash
# List all Kyverno policies
oc get cpol

# Check specific policy details
oc get cpol replace-operator-image-to-dockerhub -o yaml | grep "image:"

# See what username is currently configured
oc get cpol replace-operator-image-to-dockerhub -o jsonpath='{.spec.rules[*].mutate.foreach[*].patchStrategicMerge.spec.containers[*].image}'
```

## Updating Existing Policies

If you need to update an existing policy with a new username:

```bash
# 1. Generate new policy with updated username (run from repository root)
export DOCKERHUB_USERNAME=new-username
./local-utilities/generate-policies.sh

# 2. Apply the updated policy (will update existing)
oc apply -f kyverno-policies/replace-operator-image-to-dockerhub.yaml

# 3. Verify the update
oc get cpol replace-operator-image-to-dockerhub -o yaml | grep "image:"
```

## File Structure

```
kyverno-policies/
├── env-replace-operator-image-to-dockerhub.yaml.tpl  # Template (use envsubst)
├── env-dockerhub-image-replacement.yaml.tpl           # Template (use envsubst)
├── replace-operator-image-to-dockerhub.yaml           # Generated/Manual (ready to apply)
├── dockerhub-image-replacement.yaml                   # Generated/Manual (ready to apply)
├── ../local-utilities/generate-policies.sh            # Helper script (in local-utilities/)
├── README.md                                          # Main documentation
└── README-TEMPLATES.md                                # This file
```

## Best Practices

1. **Never commit generated files with real usernames** - Generated `.yaml` files with usernames should not be committed
2. **Use templates for CI/CD** - Generate policies during deployment from templates
3. **Document your configuration** - Keep track of which username and log levels are used in each environment
4. **Version control templates** - Commit `.tpl` files, regenerate `.yaml` files as needed

## Troubleshooting

### envsubst not found
```bash
# Install on macOS
brew install gettext

# Install on Linux (usually pre-installed)
# On RHEL/CentOS: yum install gettext
# On Ubuntu/Debian: apt-get install gettext-base
```

### Placeholders not replaced
- Ensure `DOCKERHUB_USERNAME` is exported: `export DOCKERHUB_USERNAME=your-username`
- Check template uses `${DOCKERHUB_USERNAME}` (not `$DOCKERHUB_USERNAME` or `DOCKERHUB_USERNAME`)
- Verify envsubst is working: `echo '${DOCKERHUB_USERNAME}' | envsubst`

### Policy not applying
- Check Kyverno is running: `oc get pods -n kyverno`
- Verify policy syntax: `oc apply --dry-run=client -f replace-operator-image-to-dockerhub.yaml`
- Check policy status: `oc get cpol replace-operator-image-to-dockerhub`

