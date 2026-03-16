# Offline Helm Charts for Air-Gapped Environments

This directory contains Helm charts packaged as tar.gz archives for installation in environments with proxy restrictions or air-gapped clusters.

## Available Charts

- **kyverno-3.6.1.tgz** - Kyverno v1.16.1 (504 KB)

## Why Offline Charts?

In environments with:
- Strict proxy restrictions
- Air-gapped clusters
- Limited internet access
- Corporate firewall rules

Pre-downloaded Helm charts allow installation without accessing external registries.

---

## Installation: Kyverno 3.6.1

### Prerequisites

- `oc` or `kubectl` CLI
- `helm` v3.x
- Cluster admin access
- Access to container registry (for pulling images)

### Step 1: Copy Chart to Target Environment

```bash
# Copy the tar.gz file to your target environment
scp helm-charts/kyverno-3.6.1.tgz user@target-host:/tmp/

# Or use your preferred file transfer method
```

### Step 2: Install from Local Chart

```bash
# Install Kyverno using the local tar.gz file
helm install kyverno /path/to/kyverno-3.6.1.tgz \
  --namespace kyverno \
  --create-namespace

# Example with full path
helm install kyverno /tmp/kyverno-3.6.1.tgz \
  --namespace kyverno \
  --create-namespace
```

### Step 3: Install with Custom Values (TLS Certificate Fix)

**IMPORTANT**: If you see TLS certificate errors, use the provided values file:

```bash
# Install with TLS certificate fix
helm install kyverno /path/to/kyverno-3.6.1.tgz \
  --namespace kyverno \
  --create-namespace \
  --values kyverno-values-tls-fix.yaml
```

The `kyverno-values-tls-fix.yaml` file is included in this directory and enables automatic certificate generation.

**Contents of `kyverno-values-tls-fix.yaml`**:

```yaml
# Kyverno Values - TLS Certificate Fix
#
# This values file fixes the TLS certificate generation issue
# Use this when installing Kyverno to ensure certificates are created

# Enable self-signed certificate generation
admissionController:
  createSelfSignedCert: true

backgroundController:
  createSelfSignedCert: true

cleanupController:
  createSelfSignedCert: true

reportsController:
  createSelfSignedCert: true

# Alternative: Use cert-manager if available
# Uncomment below if you have cert-manager installed
# admissionController:
#   createSelfSignedCert: false
#   certManager:
#     enabled: true

# backgroundController:
#   createSelfSignedCert: false
#   certManager:
#     enabled: true

# cleanupController:
#   createSelfSignedCert: false
#   certManager:
#     enabled: true

# reportsController:
#   createSelfSignedCert: false
#   certManager:
#     enabled: true
```

**Or create your own custom values** (with TLS fix included):

```yaml
# kyverno-values.yaml

# Enable TLS certificate generation (fixes certificate errors)
admissionController:
  createSelfSignedCert: true
  replicas: 3

backgroundController:
  createSelfSignedCert: true
  replicas: 2

reportsController:
  createSelfSignedCert: true
  replicas: 2

cleanupController:
  createSelfSignedCert: true
  replicas: 2

# Resource configuration
resources:
  limits:
    cpu: 2000m
    memory: 4Gi
  requests:
    cpu: 250m
    memory: 500Mi
```

Install with custom values:

```bash
helm install kyverno /path/to/kyverno-3.6.1.tgz \
  --namespace kyverno \
  --create-namespace \
  --values kyverno-values.yaml
```

### Step 4: Verify Installation

```bash
# Check Helm release
helm list -n kyverno

# Check pods
oc get pods -n kyverno

# Check version
oc get deployment -n kyverno -o jsonpath='{.items[0].spec.template.spec.containers[0].image}'
```

---

## Extracting and Modifying the Chart

If you need to customize the chart before installation:

### Extract the Archive

```bash
# Extract the chart
tar -xzf kyverno-3.6.1.tgz

# This creates a kyverno/ directory with:
# - Chart.yaml
# - values.yaml
# - templates/
# - crds/
```

### Modify Values

```bash
# Edit the default values
vi kyverno/values.yaml

# Or create a custom values overlay
```

### Install from Extracted Directory

```bash
# Install from the extracted directory
helm install kyverno ./kyverno/ \
  --namespace kyverno \
  --create-namespace
```

---

## Upgrading Kyverno

### Upgrade from Local Chart

```bash
# Upgrade to a new version
helm upgrade kyverno /path/to/kyverno-3.6.1.tgz \
  --namespace kyverno

# Upgrade with custom values
helm upgrade kyverno /path/to/kyverno-3.6.1.tgz \
  --namespace kyverno \
  --values kyverno-values.yaml
```

---

## Image Considerations

### Default Image Registries

Kyverno 3.6.1 pulls images from:
- `ghcr.io/kyverno/kyverno:v1.16.1`
- `ghcr.io/kyverno/kyvernopre:v1.16.1`
- `ghcr.io/kyverno/background-controller:v1.16.1`
- `ghcr.io/kyverno/cleanup-controller:v1.16.1`
- `ghcr.io/kyverno/reports-controller:v1.16.1`

### If GitHub Container Registry is Blocked

#### Option 1: Mirror Images to Internal Registry

```bash
# Pull images on a machine with internet access
podman pull ghcr.io/kyverno/kyverno:v1.16.1
podman pull ghcr.io/kyverno/kyvernopre:v1.16.1
podman pull ghcr.io/kyverno/background-controller:v1.16.1
podman pull ghcr.io/kyverno/cleanup-controller:v1.16.1
podman pull ghcr.io/kyverno/reports-controller:v1.16.1

# Tag for your internal registry
podman tag ghcr.io/kyverno/kyverno:v1.16.1 registry.internal.com/kyverno/kyverno:v1.16.1
# ... repeat for all images

# Push to internal registry
podman push registry.internal.com/kyverno/kyverno:v1.16.1
# ... repeat for all images
```

#### Option 2: Override Image Registry in Values

Create `kyverno-values.yaml`:

```yaml
image:
  repository: registry.internal.com/kyverno/kyverno
  tag: v1.16.1

admissionController:
  image:
    repository: registry.internal.com/kyverno/kyverno
    tag: v1.16.1

backgroundController:
  image:
    repository: registry.internal.com/kyverno/background-controller
    tag: v1.16.1

cleanupController:
  image:
    repository: registry.internal.com/kyverno/cleanup-controller
    tag: v1.16.1

reportsController:
  image:
    repository: registry.internal.com/kyverno/reports-controller
    tag: v1.16.1
```

Install with overridden images:

```bash
helm install kyverno /path/to/kyverno-3.6.1.tgz \
  --namespace kyverno \
  --create-namespace \
  --values kyverno-values.yaml
```

---

## Troubleshooting

### Chart Archive Corrupted

```bash
# Verify the archive integrity
tar -tzf kyverno-3.6.1.tgz | head

# Re-download if needed (on a machine with internet)
helm pull kyverno/kyverno --version 3.6.1
```

### Image Pull Errors

```bash
# Check if images are accessible
oc run test-pull --image=ghcr.io/kyverno/kyverno:v1.16.1 --rm -it --restart=Never

# If fails, you need to mirror images to an accessible registry
```

### TLS Certificate Issues

**Symptom**: Errors like `secret "kyverno-svc.kyverno.svc.kyverno-tls-pair" not found`

**Solution**: Install with TLS certificate generation enabled:

```bash
# Uninstall if already installed
helm uninstall kyverno -n kyverno

# Reinstall with TLS fix
helm install kyverno /path/to/kyverno-3.6.1.tgz \
  --namespace kyverno \
  --create-namespace \
  --values kyverno-values-tls-fix.yaml

# Or upgrade existing installation
helm upgrade kyverno /path/to/kyverno-3.6.1.tgz \
  --namespace kyverno \
  --values kyverno-values-tls-fix.yaml
```

**Verify certificates were created**:

```bash
# Check certificate secrets
oc get secrets -n kyverno | grep tls

# Should see:
# kyverno-svc.kyverno.svc.kyverno-tls-ca
# kyverno-svc.kyverno.svc.kyverno-tls-pair
# kyverno-cleanup-controller.kyverno.svc.kyverno-tls-ca
# kyverno-cleanup-controller.kyverno.svc.kyverno-tls-pair
```

---

## Uninstallation

```bash
# Uninstall Helm release
helm uninstall kyverno -n kyverno

# Delete namespace
oc delete namespace kyverno

# Clean up webhooks (if needed)
oc delete validatingwebhookconfiguration -l webhook.kyverno.io/managed-by=kyverno
oc delete mutatingwebhookconfiguration -l webhook.kyverno.io/managed-by=kyverno
```

---

## Downloading Additional Versions

For maintainers who need to download new chart versions:

### Step 1: Add Kyverno Helm Repository (if not already added)

```bash
# Add the Kyverno Helm repository
helm repo add kyverno https://kyverno.github.io/kyverno/

# Verify repository was added
helm repo list | grep kyverno
```

### Step 2: Update Repository Index

```bash
# Update all Helm repositories to get latest chart versions
helm repo update

# Or update only Kyverno repo
helm repo update kyverno
```

### Step 3: List Available Versions

```bash
# List all available Kyverno chart versions
helm search repo kyverno/kyverno --versions

# Show top 10 versions
helm search repo kyverno/kyverno --versions | head -11
```

### Step 4: Download Specific Version

```bash
# Navigate to helm-charts directory
cd helm-charts/

# Download Kyverno 3.6.1 (current)
helm pull kyverno/kyverno --version 3.6.1

# Download other versions
helm pull kyverno/kyverno --version 3.6.0
helm pull kyverno/kyverno --version 3.5.2

# Download latest version
helm pull kyverno/kyverno
```

### Step 5: Verify Downloaded Chart

```bash
# List downloaded charts
ls -lh *.tgz

# Verify chart contents
tar -tzf kyverno-3.6.1.tgz | head -20

# Check chart metadata
helm show chart kyverno-3.6.1.tgz
```

### Step 6: Commit to Repository

```bash
# Add to git
git add kyverno-*.tgz

# Update README if needed
vi README.md

# Commit
git commit -m "Add Kyverno chart version X.Y.Z"

# Push
git push
```

---

## Comparison: Offline Chart vs Online Installation

| Method | Pros | Cons |
|--------|------|------|
| **Offline Chart (tar.gz)** | Works in air-gapped, no proxy issues, version controlled | Still needs image registry access, manual updates |
| **Online Helm** | Easy updates, latest versions | Requires internet/proxy access, may be blocked |
| **Plain YAML** | Simple, no Helm required | Hard to customize, no templating, difficult upgrades |

---

## Chart Information

```bash
# View chart metadata
helm show chart kyverno-3.6.1.tgz

# View chart values
helm show values kyverno-3.6.1.tgz

# View chart README
helm show readme kyverno-3.6.1.tgz
```

---

## For OpenShift Environments

Kyverno works with OpenShift's default `restricted-v2` SCC. No additional security configuration needed.

For complete OpenShift-specific instructions, see:
`kyverno-policies/kyverno-install-guide.md`

---

## Hack: Patching Operator Image via OLM CSV

> **WARNING**: This is a temporary hack. OLM may revert the image if the subscription is set to `Automatic`. Always set approval to `Manual` first.

Use this when you need to override the operator image in the CSV (e.g. to test a custom build from a personal registry) without going through a full OLM upgrade.

### Step 1: Check the current CSV and container images

```bash
oc get csv -n namespace-configuration-operator -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.install.spec.deployments[*].spec.template.spec.containers[*].name}{"\t"}{.spec.install.spec.deployments[*].spec.template.spec.containers[*].image}{"\n"}{end}'
```

Expected output (containers at index 0=kube-rbac-proxy, 1=manager):
```
namespace-configuration-operator.v1.2.6   kube-rbac-proxy manager   quay.io/redhat-cop/kube-rbac-proxy@sha256:... quay.io/redhat-cop/namespace-configuration-operator@sha256:...
```

### Step 2: Set subscription to Manual (prevents OLM from reverting the change)

```bash
oc patch subscription namespace-configuration-operator \
  -n namespace-configuration-operator \
  --type merge \
  -p '{"spec":{"installPlanApproval":"Manual"}}'
```

### Step 3: Patch the CSV manager image

```bash
oc patch csv namespace-configuration-operator.v1.2.6 \
  -n namespace-configuration-operator \
  --type='json' \
  -p='[{"op":"replace","path":"/spec/install/spec/deployments/0/spec/template/spec/containers/1/image","value":"quay.io/ephico2real/namespace-configuration-operator:latest"}]'
```

### Step 4: Verify the deployment picked up the new image

```bash
# Check the deployment image
oc get deployment namespace-configuration-operator-controller-manager \
  -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="manager")].image}'

# Check pod is running
oc get pods -n namespace-configuration-operator
```

---

## Notes

- **Chart Version**: 3.6.1
- **App Version**: v1.16.1  
- **Size**: 504 KB
- **Downloaded**: March 16, 2026
- **Source**: https://github.com/kyverno/kyverno
- **ClusterPolicy Support**: Full support (not deprecated)
- **Prepared for 1.17+**: MutatingPolicy versions available in `kyverno-policies/mutating-*.yaml`
