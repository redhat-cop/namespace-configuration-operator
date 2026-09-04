# Kyverno Policies for Namespace Configuration Operator

This directory contains various Kyverno policies to manage container image handling, registry redirection, and security configurations for the Namespace Configuration Operator.

## Overview

The policies in this directory provide automated image registry management, pull secret injection, and operator deployment configurations. They are designed to work together to provide a seamless experience when working with different container registries.

## Policies Index

| Policy | Type | Purpose | Status | Template |
|--------|------|---------|---------|----------|
| [inject-dockerhub-secret.yaml](#inject-dockerhub-secret) | Security | Inject Docker Hub pull secrets | ✅ Active | N/A |
| [dockerhub-imagePullSecret-injection.yaml](#dockerhub-imagepullsecret-injection) | Security | Enhanced Docker Hub secret injection | ✅ Active | N/A |
| [replace-operator-image-to-dockerhub.yaml](#replace-operator-image-to-dockerhub) | Registry | Force operator to use Docker Hub images | ✅ Active | `env-replace-operator-image-to-dockerhub.yaml.tpl` |
| [dockerhub-image-replacement.yaml](#dockerhub-image-replacement) | Registry | Replace Quay.io with Docker Hub | ✅ Active | `env-dockerhub-image-replacement.yaml.tpl` |
| [internal-registry-image-replacement.yaml](#internal-registry-image-replacement) | Registry | Replace Quay.io with OpenShift internal registry | ✅ Active | N/A (no variables) |
| [sample-image-replacement.yaml](#sample-image-replacement) | Example | Harbor registry redirection example | 📚 Reference | N/A |
| [operator-log-level-config.yaml](#operator-log-level-config) | Configuration | Configure operator log levels via Kyverno | ✅ Active | `env-operator-log-level-config.yaml.tpl` |

## Quick Start: Using Templates

**For policies that require Docker Hub username or log level configuration, use the template files:**

```bash
# 1. Set your Docker Hub username (required)
export DOCKERHUB_USERNAME=your-username

# 2. Optional: Set log level configuration
export ZAP_LOG_LEVEL=info
export ZAP_DEVEL=false

# 3. Generate policies from templates (run from repository root)
./local-utilities/generate-policies.sh

# 4. Apply generated policies
oc apply -f kyverno-policies/replace-operator-image-to-dockerhub.yaml
oc apply -f kyverno-policies/dockerhub-image-replacement.yaml
oc apply -f kyverno-policies/operator-log-level-config.yaml
```

See [README-TEMPLATES.md](README-TEMPLATES.md) for detailed template usage instructions.

---

## Policy Details

### inject-dockerhub-secret

**File**: `inject-dockerhub-secret.yaml`  
**Type**: ClusterPolicy  
**Purpose**: Inject Docker Hub image pull secrets for operator components

#### What it does:
- Automatically injects `dockerhub-secret` for pods using Docker Hub images
- Targets the `namespace-configuration-operator` namespace specifically
- Handles both direct Pods and Deployment workloads
- Focuses on the operator controller manager deployment

#### Targets:
- **Pods**: All pods in `namespace-configuration-operator` namespace with `docker.io/*` or `library/*` images
- **Deployments**: The `namespace-configuration-operator-controller-manager` deployment

#### Prerequisites:
- `dockerhub-secret` must exist in the `namespace-configuration-operator` namespace
- Kyverno must be installed and running

---

### dockerhub-imagePullSecret-injection

**File**: `dockerhub-imagePullSecret-injection.yaml`  
**Type**: ClusterPolicy  
**Purpose**: Enhanced Docker Hub image pull secret injection with intelligent image detection

#### What it does:
- Uses Kyverno's `imageRegistry` context to intelligently detect Docker Hub images
- Automatically injects `dockerhub-secret` for any workload using Docker Hub
- Supports all workload types (Deployment, StatefulSet, DaemonSet, ReplicaSet)
- Uses precise image registry detection rather than pattern matching

#### Key Features:
- **Smart Detection**: Uses `imageData.registry` context for precise matching
- **Broad Coverage**: Handles all Kubernetes workload types
- **Auto-generation**: Supports Kyverno's autogen for controller resources

#### Targets:
- Any Pod or workload using images from `docker.io` registry
- Handles both explicit `docker.io/` and implicit Docker Hub references

---

### replace-operator-image-to-dockerhub

**File**: `replace-operator-image-to-dockerhub.yaml`  
**Type**: ClusterPolicy  
**Purpose**: Force the namespace configuration operator to always use Docker Hub images

#### What it does:
- Rewrites the operator deployment to use `docker.io/DOCKERHUB_USERNAME/namespace-configuration-operator:latest`
- **Note**: Use the template file (`env-replace-operator-image-to-dockerhub.yaml.tpl`) with `generate-policies.sh` to set your Docker Hub username
- Applies to both Pod and Deployment resources
- Specifically targets the `manager` container
- Automatically injects the required `dockerhub-secret`

#### Use Cases:
- **Development**: Force use of custom Docker Hub builds
- **Testing**: Override default operator images
- **Air-gapped environments**: Redirect to internal Docker Hub mirror

#### Targets:
- **Pods**: Direct pods in `namespace-configuration-operator` namespace
- **Deployments**: The `namespace-configuration-operator-controller-manager` deployment

---

### dockerhub-image-replacement

**File**: `dockerhub-image-replacement.yaml`  
**Type**: ClusterPolicy  
**Purpose**: Replace Quay.io namespace-configuration-operator images with Docker Hub equivalents

#### What it does:
- Intercepts any use of `quay.io/*/namespace-configuration-operator` images
- Redirects to `docker.io/DOCKERHUB_USERNAME/namespace-configuration-operator`
- **Note**: Use the template file (`env-dockerhub-image-replacement.yaml.tpl`) with `generate-policies.sh` to set your Docker Hub username
- Handles both tag-based and digest-based image references
- Automatically injects `dockerhub-secret` for authentication

#### Features:
- **Digest Support**: Handles `sha256:` digest references correctly
- **Tag Support**: Preserves tag names when redirecting
- **Secret Injection**: Automatically adds required pull secrets
- **Comprehensive Coverage**: Handles both initContainers and containers

#### Use Cases:
- **Registry Migration**: Move from Quay.io to Docker Hub
- **Access Control**: Use Docker Hub when Quay.io access is restricted
- **Cost Optimization**: Avoid Quay.io pull limits

---

### internal-registry-image-replacement

**File**: `internal-registry-image-replacement.yaml`  
**Type**: ClusterPolicy  
**Purpose**: Replace Quay.io images with OpenShift internal registry

#### What it does:
- Redirects `quay.io/redhat-cop/namespace-configuration-operator` to internal registry
- Uses the full internal registry URL: `image-registry.openshift-image-registry.svc.cluster.local:5000`
- Preserves image tags and digests
- No pull secret injection needed (uses internal cluster authentication)

#### Benefits:
- **No Pull Secrets**: Uses OpenShift's internal authentication
- **Network Efficiency**: Images stay within the cluster
- **Air-gapped Support**: Works without external registry access
- **Cost Savings**: No external registry bandwidth costs

#### Target Registry:
```
image-registry.openshift-image-registry.svc.cluster.local:5000/namespace-configuration-operator/namespace-configuration-operator:TAG
```

#### No Template Needed:
Unlike Docker Hub policies, this policy uses **fixed, standardized values** that never change:
- **Registry URL**: `image-registry.openshift-image-registry.svc.cluster.local:5000` (standard OpenShift internal registry)
- **Namespace**: `namespace-configuration-operator` (operator's namespace)
- **Image Name**: `namespace-configuration-operator` (operator's image name)

These values are consistent across all OpenShift clusters, so this policy can be applied directly without any variable substitution or templates.

---

### operator-log-level-config

**File**: `operator-log-level-config.yaml`  
**Type**: ClusterPolicy  
**Purpose**: Configure operator log levels via Kyverno mutation (works with OLM-managed deployments)

#### What it does:
- Injects `ZAP_LOG_LEVEL` and `ZAP_DEVEL` environment variables into the operator Deployment
- Works with OLM-managed deployments (OLM will not revert Kyverno mutations)
- Ensures log level configuration persists across operator updates

#### Why use this:
- **OLM Constraint**: Direct Deployment edits are reverted by OLM
- **Subscription Alternative**: If Subscription config is not available or preferred
- **Persistent Configuration**: Kyverno mutations survive OLM updates

#### Configuration:

**Option 1: Use Template with Environment Variables (Recommended)**
```bash
# Set Docker Hub username (required for other policies, optional for log level policy)
export DOCKERHUB_USERNAME=your-username

# Set log level environment variables
export ZAP_LOG_LEVEL=info
export ZAP_DEVEL=false

# Generate all policies from templates (run from repository root)
./local-utilities/generate-policies.sh

# Apply generated log level policy
oc apply -f kyverno-policies/operator-log-level-config.yaml
```

**Option 2: Edit Policy Directly**
Edit the policy file to change log levels:
```yaml
env:
- name: ZAP_LOG_LEVEL
  value: "info"  # Options: "error", "info", "debug", "0-10"
- name: ZAP_DEVEL
  value: "false"  # Options: "true" (console), "false" (JSON)
```

#### Recommended Settings:
- **Production**: `ZAP_LOG_LEVEL=info`, `ZAP_DEVEL=false` (JSON, info level)
- **Debugging**: `ZAP_LOG_LEVEL=2`, `ZAP_DEVEL=false` (JSON, shows template filtering logs)
- **Development**: `ZAP_LOG_LEVEL=info`, `ZAP_DEVEL=true` (console, human-readable)

#### Alternative: Subscription Configuration
For OLM-deployed operators, you can also configure log levels via Subscription:
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
spec:
  config:
    env:
    - name: ZAP_LOG_LEVEL
      value: "info"
    - name: ZAP_DEVEL
      value: "false"
```

See [LOG_LEVEL_CONFIGURATION.md](../docs/LOG_LEVEL_CONFIGURATION.md) for detailed documentation.

---

### sample-image-replacement

**File**: `sample-image-replacement.yaml`  
**Type**: ClusterPolicy  
**Purpose**: Example policy showing Harbor registry redirection

#### What it does:
- **Reference Implementation**: Shows how to redirect Docker Hub to Harbor
- **Pull-through Cache**: Demonstrates Harbor's proxy functionality
- **Educational**: Template for creating custom registry redirections

#### Example Use Case:
```yaml
# Original image
docker.io/library/nginx:latest

# Redirected to
harbor.example.com/k8s/library/nginx:latest
```

---

## Deployment Strategies

### Strategy 1: Docker Hub Focus
Deploy these policies for Docker Hub-centric environments:
```bash
oc apply -f inject-dockerhub-secret.yaml
oc apply -f dockerhub-imagePullSecret-injection.yaml
oc apply -f dockerhub-image-replacement.yaml
oc apply -f replace-operator-image-to-dockerhub.yaml
```

### Strategy 2: Internal Registry Focus
Deploy these policies for air-gapped/internal environments:
```bash
oc apply -f internal-registry-image-replacement.yaml
```

### Strategy 3: Development/Testing
For development with custom images:
```bash
oc apply -f replace-operator-image-to-dockerhub.yaml
```

---

## Prerequisites

### Required Secrets
Create the Docker Hub secret before applying policies:
```bash
oc create secret docker-registry dockerhub-secret \
  --docker-server=docker.io \
  --docker-username=your-username \
  --docker-password=your-password \
  --docker-email=your-email@example.com \
  -n namespace-configuration-operator
```

### Required Cluster Components
- **Kyverno**: All policies require Kyverno to be installed
- **OpenShift Image Registry**: Required for internal registry policies
- **Proper RBAC**: Kyverno needs permissions to mutate resources

---

## Policy Interactions

### Complementary Policies
These policies work well together:
- `inject-dockerhub-secret.yaml` + `dockerhub-imagePullSecret-injection.yaml`: Comprehensive Docker Hub support
- `dockerhub-image-replacement.yaml` + `inject-dockerhub-secret.yaml`: Complete Quay.io → Docker Hub migration

### Conflicting Policies
Avoid using these together:
- `dockerhub-image-replacement.yaml` + `internal-registry-image-replacement.yaml`: Both try to replace Quay.io images
- `replace-operator-image-to-dockerhub.yaml` + `internal-registry-image-replacement.yaml`: Conflicting operator image sources

---

## Customization Guide

### Changing Docker Hub User

**Recommended: Use Template Files**

The easiest way is to use the template files with `envsubst`:

```bash
# Set your Docker Hub username
export DOCKERHUB_USERNAME=your-username

# Generate policies from templates (run from repository root)
./local-utilities/generate-policies.sh

# Apply generated policies
oc apply -f kyverno-policies/replace-operator-image-to-dockerhub.yaml
oc apply -f kyverno-policies/dockerhub-image-replacement.yaml
```

**Alternative: Manual Replacement**

If you prefer to edit files directly:

```bash
# Replace placeholder with your username
sed -i 's/DOCKERHUB_USERNAME/your-username/g' \
  kyverno-policies/dockerhub-image-replacement.yaml \
  kyverno-policies/replace-operator-image-to-dockerhub.yaml

# Then apply
oc apply -f kyverno-policies/
```

**Updating Existing Cluster Policies**

If policies are already deployed with a different username:

```bash
# 1. Generate new policy with updated username (run from repository root)
export DOCKERHUB_USERNAME=new-username
./local-utilities/generate-policies.sh

# 2. Apply to update existing policy
oc apply -f kyverno-policies/replace-operator-image-to-dockerhub.yaml

# 3. Verify update
oc get cpol replace-operator-image-to-dockerhub -o yaml | grep "image:"
```

**Files that need updating:**
- `dockerhub-image-replacement.yaml` (9 instances - includes initContainers and containers)
- `replace-operator-image-to-dockerhub.yaml` (2 instances)

**Template Files Available:**
- `env-replace-operator-image-to-dockerhub.yaml.tpl` - Use with envsubst
- `env-dockerhub-image-replacement.yaml.tpl` - Use with envsubst

### Adding New Registry Redirections
Use `sample-image-replacement.yaml` as a template:
1. Copy the file
2. Update registry URLs
3. Modify image path patterns
4. Add any required secret injections

---

## Troubleshooting

### Policy Not Applying
1. **Check Kyverno Status**: `oc get pods -n kyverno`
2. **Verify Policy Status**: `oc get cpol`
3. **Check Events**: `oc get events --field-selector reason=PolicyViolation`

### Images Still Pulling from Wrong Registry
1. **Check Policy Precedence**: Multiple policies can conflict - disable conflicting policies
2. **Verify Image Patterns**: Ensure your images match the policy conditions
3. **Check Background Processing**: Some policies only apply to new resources - recreate the resource

### Pull Secret Issues
1. **Verify Secret Exists**: 
   ```bash
   oc get secret dockerhub-secret -n namespace-configuration-operator
   ```
   
   **Create Secret (if missing):**
   ```bash
   # Use the utility script (run from repository root)
   ./local-utilities/create-dockerhub-secret.sh
   
   # Or manually
   oc create secret docker-registry dockerhub-secret \
       --docker-server=docker.io \
       --docker-username=YOUR_USERNAME \
       --docker-password=YOUR_PASSWORD \
       --docker-email=YOUR_EMAIL \
       -n namespace-configuration-operator
   ```
2. **Test Secret**: Try manual pull with the secret
3. **Check Secret Format**: Ensure it's a `docker-registry` type secret

---

## Monitoring

### Check Policy Status
```bash
# List all cluster policies
oc get cpol

# Check specific policy details
oc describe cpol inject-dockerhub-secret

# View policy events
oc get events --field-selector involvedObject.kind=ClusterPolicy
```

### Verify Mutations
```bash
# Check if secrets were injected
oc get pod -o yaml | grep -A5 imagePullSecrets

# Verify image redirections
oc get deployment namespace-configuration-operator-controller-manager -o yaml | grep image:
```

---

## Contributing

When adding new policies:
1. **Follow Naming Convention**: Use descriptive, kebab-case names
2. **Add Documentation**: Include comprehensive annotations
3. **Test Thoroughly**: Verify policy works in isolation and with others
4. **Update This README**: Add new policy to the index and details sections

---

## Security Considerations

### Pull Secret Security
- Store Docker Hub credentials securely
- Use least-privilege access for registry accounts
- Rotate credentials regularly
- Consider using service accounts instead of personal accounts

### Policy Security
- Review all policies before applying to production
- Test policies in development environments first
- Monitor policy mutations for unexpected behavior
- Regularly audit applied policies

---

## Version Compatibility

| Kyverno Version | OpenShift Version | Kubernetes Version | Status |
|----------------|------------------|-------------------|---------|
| 1.11.4+ | 4.12+ | 1.27+ | ✅ Tested |
| 1.10+ | 4.10+ | 1.25+ | ✅ Compatible |
| < 1.10 | < 4.10 | < 1.25 | ❌ Not supported |

---

For questions or issues with these policies, please refer to the main repository documentation or create an issue in the project repository.