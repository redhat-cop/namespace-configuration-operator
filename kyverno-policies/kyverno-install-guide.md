# Kyverno 3.6.1 Installation Guide for OpenShift

This guide provides step-by-step instructions for installing Kyverno 3.6.1 (app version v1.16.1) on OpenShift clusters.

## Table of Contents
- [Prerequisites](#prerequisites)
- [OpenShift Considerations](#openshift-considerations)
- [Installation Steps](#installation-steps)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)
- [Uninstallation](#uninstallation)

---

## Prerequisites

### Required Tools
- `oc` CLI (OpenShift command-line tool)
- `helm` v3.x
- Cluster admin access

### Minimum Requirements
- OpenShift 4.10+ (Kubernetes 1.23+)
- Cluster admin permissions
- Adequate cluster resources:
  - CPU: 2 cores
  - Memory: 4 GB
  - Storage: 10 GB

---

## OpenShift Considerations

### SecurityContextConstraints (SCC)
**Good News**: Kyverno works with OpenShift's default **`restricted-v2`** SCC out of the box. No custom SCC is required.

Verification from our running cluster:
```bash
$ oc get pods -n kyverno -o jsonpath='{.items[0].metadata.annotations.openshift\.io/scc}'
restricted-v2
```

### Network Policies
Kyverno requires webhook access. OpenShift's default network policies allow this, but if you have custom network policies, ensure:
- Webhook traffic on port 9443 is allowed
- API server can reach Kyverno pods

### Pod Security Standards
OpenShift enforces Pod Security Standards. Kyverno is compatible with the `restricted` profile.

---

## Installation Method: Helm Chart Installation

### Step 1: Add Kyverno Helm Repository

```bash
# Add the Kyverno Helm repository
helm repo add kyverno https://kyverno.github.io/kyverno/

# Update the repository
helm repo update

# List available Kyverno versions
helm search repo kyverno/kyverno --versions | head -20

# Expected output:
# NAME                 CHART VERSION  APP VERSION  DESCRIPTION
# kyverno/kyverno      3.6.1          v1.16.1      Kubernetes Native Policy Management
# kyverno/kyverno      3.6.0          v1.16.0      Kubernetes Native Policy Management
# kyverno/kyverno      3.5.2          v1.15.2      Kubernetes Native Policy Management
# ...

# Note: Helm chart 3.6.1 provides v1.16.1
# Newer releases like v1.16.2 and v1.16.3 exist in git but may not have Helm charts yet
```

### Step 2: Create Kyverno Namespace

```bash
# Create the namespace
oc create namespace kyverno

# Verify namespace creation
oc get namespace kyverno
```

### Step 3: Install Kyverno

#### Option A: Default Installation (Recommended)

```bash
helm install kyverno kyverno/kyverno \
  --namespace kyverno \
  --version 3.6.1 \
  --create-namespace
```

#### Option B: Custom Values Installation

Create a `kyverno-values.yaml` file:

```yaml
# kyverno-values.yaml

# Replicas for high availability (optional)
replicaCount: 3

# Resource limits
resources:
  limits:
    cpu: 2000m
    memory: 4Gi
  requests:
    cpu: 250m
    memory: 500Mi

# Admission controller configuration
admissionController:
  replicas: 3

# Background controller configuration
backgroundController:
  replicas: 2

# Reports controller configuration
reportsController:
  replicas: 2

# Cleanup controller configuration
cleanupController:
  replicas: 2
```

Install with custom values:

```bash
helm install kyverno kyverno/kyverno \
  --namespace kyverno \
  --version 3.6.1 \
  --create-namespace \
  --values kyverno-values.yaml
```

### Step 4: Wait for Deployment

```bash
# Watch the pods come up
oc get pods -n kyverno -w

# Wait for all pods to be ready
oc wait --for=condition=ready pod -l app.kubernetes.io/instance=kyverno -n kyverno --timeout=300s
```

Expected pods:
- `kyverno-admission-controller-*` (1-3 replicas)
- `kyverno-background-controller-*` (1-2 replicas)
- `kyverno-cleanup-controller-*` (1-2 replicas)
- `kyverno-reports-controller-*` (1-2 replicas)

---

## Verification

### Verify Installation

```bash
# Check Helm release
helm list -n kyverno

# Expected output:
# NAME     NAMESPACE  REVISION  UPDATED                              STATUS    CHART          APP VERSION
# kyverno  kyverno    1         2025-12-04 13:47:24.330646 -0600 CST deployed  kyverno-3.6.1  v1.16.1

# Check all pods are running
oc get pods -n kyverno

# Check Kyverno version
oc get deploy -n kyverno -o jsonpath='{.items[0].spec.template.spec.containers[0].image}'
```

### Verify Webhook Configuration

```bash
# Check ValidatingWebhookConfiguration
oc get validatingwebhookconfiguration | grep kyverno

# Check MutatingWebhookConfiguration
oc get mutatingwebhookconfiguration | grep kyverno

# Verify webhook endpoints
oc get svc -n kyverno
```

### Test with a Sample Policy

Create a test policy:

```bash
cat <<EOF | oc apply -f -
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: Audit
  rules:
  - name: check-for-labels
    match:
      any:
      - resources:
          kinds:
          - Pod
    validate:
      message: "Label 'app' is required"
      pattern:
        metadata:
          labels:
            app: "?*"
EOF
```

Verify the policy:

```bash
# Check policy status
oc get cpol require-labels

# Should show READY: True
```

Clean up test policy:

```bash
oc delete cpol require-labels
```

---

## Offline Installation (Air-Gapped / Proxy-Restricted Environments)

For environments with proxy restrictions or limited internet access, use the pre-downloaded Helm chart:

```bash
# Install from local tar.gz file
helm install kyverno /path/to/kyverno-3.6.1.tgz \
  --namespace kyverno \
  --create-namespace
```

**Pre-downloaded charts are available in `helm-charts/` directory.**

For complete offline installation instructions, see:
- `helm-charts/README.md` - Comprehensive offline installation guide
- Includes chart download procedures
- Image mirroring strategies
- Troubleshooting for restricted environments

---

## Troubleshooting

### Pods Not Starting

**Check pod status:**
```bash
oc describe pod -n kyverno <pod-name>
```

**Common issues:**
- Image pull errors: Check network/registry access
- Resource constraints: Increase node resources
- SCC violations: Verify pods use `restricted-v2` SCC

### Webhook Failures

**Check webhook configuration:**
```bash
oc get validatingwebhookconfiguration -o yaml | grep -A 20 kyverno
```

**Check Kyverno service:**
```bash
oc get svc -n kyverno
oc get endpoints -n kyverno
```

**Test webhook connectivity:**
```bash
oc run test-pod --image=busybox --rm -it -- wget -O- https://kyverno-svc.kyverno.svc:443
```

### Certificate Issues

Kyverno auto-generates certificates. If you see certificate errors:

```bash
# Check certificate secrets
oc get secrets -n kyverno | grep tls

# Restart Kyverno to regenerate certificates
oc rollout restart deployment -n kyverno
```

### View Logs

```bash
# Admission controller logs
oc logs -n kyverno -l app.kubernetes.io/component=admission-controller --tail=100 -f

# Background controller logs
oc logs -n kyverno -l app.kubernetes.io/component=background-controller --tail=100 -f

# Reports controller logs
oc logs -n kyverno -l app.kubernetes.io/component=reports-controller --tail=100 -f
```

---

## Uninstallation

### Step 1: Delete Policies First

```bash
# Delete all ClusterPolicies
oc delete cpol --all

# Delete all Policies
oc delete pol --all -A

# Delete any PolicyExceptions
oc delete polexceptions --all -A
```

### Step 2: Uninstall Helm Release

```bash
# Uninstall Kyverno
helm uninstall kyverno -n kyverno

# Delete the namespace
oc delete namespace kyverno
```

### Step 3: Clean Up Webhooks (if necessary)

Sometimes webhook configurations remain after uninstallation:

```bash
# Delete validating webhooks
oc delete validatingwebhookconfiguration -l webhook.kyverno.io/managed-by=kyverno

# Delete mutating webhooks
oc delete mutatingwebhookconfiguration -l webhook.kyverno.io/managed-by=kyverno
```

---

## Upgrade Path

### Current: Kyverno 1.16.1 (Chart 3.6.1)
- ClusterPolicy: **Fully supported**
- MutatingPolicy (CEL): **Beta**

### Future: Kyverno 1.17+ (Chart 3.7.x+)
- ClusterPolicy: **Deprecated** (still functional)
- MutatingPolicy (CEL): **GA/Stable**

**Migration Path:**
1. Stay on 3.6.1 until ready to migrate policies
2. Prepare CEL-based MutatingPolicy versions (see `mutating-*.yaml` files)
3. Test MutatingPolicies in dev environment
4. Upgrade Helm chart to 3.7.x+
5. Delete old ClusterPolicies
6. Apply new MutatingPolicies
7. Verify all policies work correctly

---

## Additional Resources

- [Kyverno Documentation](https://kyverno.io/docs/)
- [OpenShift Documentation](https://docs.openshift.com/)
- [Kyverno GitHub](https://github.com/kyverno/kyverno)
- [Kyverno Slack](https://slack.k8s.io/) - #kyverno channel
- [Migration to CEL Guide](https://kyverno.io/blog/2026/02/02/announcing-kyverno-release-1.17/)

---

## Version Strategy

### Why We're on 3.6.1 (v1.16.1)

**Current Status**: We are running Kyverno **3.6.1 (v1.16.1)** installed via Helm.

**Note**: Newer patch versions exist in git (v1.16.2, v1.16.3) but corresponding Helm charts may not be available yet. For most use cases, v1.16.1 is sufficient.

**Reasons to stay on this version:**

1. **ClusterPolicy Support**: Full support for ClusterPolicy-based policies (our current implementation)
2. **Stability**: Proven stable in production (running for 99+ days)
3. **No Breaking Changes**: ClusterPolicy works perfectly without deprecation warnings
4. **Migration Preparation**: Gives us time to prepare and test CEL-based MutatingPolicy versions

### Future Migration to 1.17+

**When to Upgrade**: When ready to migrate to the new CEL-based policy engine

**Kyverno 1.17+ Changes**:
- **ClusterPolicy**: Deprecated (but still functional)
- **MutatingPolicy**: GA/Stable (CEL-based, replaces mutate rules)
- **ValidatingPolicy**: GA/Stable (CEL-based, replaces validate rules)
- **GeneratingPolicy**: GA/Stable (CEL-based, replaces generate rules)

**Migration Steps**:
1. ✅ Prepare MutatingPolicy versions (already done - see `mutating-*.yaml` files)
2. Test MutatingPolicies in dev environment
3. Upgrade Helm chart: `helm upgrade kyverno kyverno/kyverno --version 3.7.x+`
4. Apply new MutatingPolicy resources
5. Delete old ClusterPolicy resources
6. Verify all policies work correctly

**Benefits of CEL-based Policies**:
- Better performance
- Native Kubernetes ValidatingAdmissionPolicy integration
- Standardized expression language
- Future-proof architecture

**Risk Assessment**:
- **Low Risk**: Stay on 3.6.1 (stable, supported)
- **Medium Risk**: Upgrade to 1.17+ without testing (ClusterPolicy deprecated)
- **Recommended**: Upgrade when ready, after thorough testing

---

## Notes

- This installation was performed on **OpenShift 4.12+**
- Kyverno runs successfully with OpenShift's default **restricted-v2** SCC
- No custom SCC or security modifications required
- Installation date: December 4, 2025
- Current status: Stable, running for 99+ days
- **Current version**: 3.6.1 (v1.16.1) - ClusterPolicy fully supported
- **Prepared for**: 3.7.x+ (v1.17+) - MutatingPolicy/ValidatingPolicy ready
