# OLM Catalog Image Override Guide

This guide explains how to override the operator image in an OLM-managed installation to use your custom image from Quay.io.

## Problem

When using OLM (Operator Lifecycle Manager), the operator image is defined in the **ClusterServiceVersion (CSV)** which is managed by OLM. Direct changes to the Deployment are reverted by OLM.

## Current Image

```bash
# Check current CSV image
oc get csv -n namespace-configuration-operator namespace-configuration-operator.v1.2.6 \
  -o jsonpath='{.spec.install.spec.deployments[0].spec.template.spec.containers[1].image}'

# Current: quay.io/redhat-cop/namespace-configuration-operator@sha256:...
# Desired: quay.io/ephico2real/namespace-configuration-operator:latest
```

## Solutions

### Option 1: Patch the CSV Directly (Temporary)

This works but OLM may revert it on updates.

```bash
# Patch the CSV to use custom image
oc patch csv namespace-configuration-operator.v1.2.6 \
  -n namespace-configuration-operator \
  --type='json' \
  -p='[{
    "op": "replace",
    "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/1/image",
    "value": "quay.io/ephico2real/namespace-configuration-operator:latest"
  }]'

# Force restart the deployment
oc rollout restart deployment namespace-configuration-operator-controller-manager \
  -n namespace-configuration-operator
```

**Limitations:**
- Changes may be reverted on CSV updates
- Not persistent across operator upgrades

---

### Option 2: Use Kyverno MutatingPolicy (Recommended)

This is the **most reliable** approach for OLM-managed operators.

**Already implemented in your cluster!**

```bash
# Check existing Kyverno policy
oc get cpol replace-operator-image-to-dockerhub

# The policy automatically replaces images on Deployment CREATE/UPDATE
```

See `kyverno-policies/replace-operator-image-to-dockerhub.yaml` for details.

**Benefits:**
- Survives OLM updates
- Automatic enforcement
- Already configured

---

### Option 3: Create Custom Catalog with Modified CSV

This is the **proper OLM-native way** but requires more setup.

#### Step 1: Clone and Modify the Bundle

```bash
# Clone the operator repository
git clone https://github.com/redhat-cop/namespace-configuration-operator.git
cd namespace-configuration-operator

# Checkout the version you're using
git checkout v1.2.6

# Edit the CSV
vi bundle/manifests/namespace-configuration-operator.clusterserviceversion.yaml
```

#### Step 2: Modify the Image in CSV

Find and replace the image:

```yaml
# In bundle/manifests/namespace-configuration-operator.clusterserviceversion.yaml
spec:
  install:
    spec:
      deployments:
        - spec:
            template:
              spec:
                containers:
                  - name: manager
                    image: quay.io/ephico2real/namespace-configuration-operator:latest  # Change this
                    imagePullPolicy: Always  # Add this
```

#### Step 3: Build Custom Bundle Image

```bash
# Build the bundle image
podman build -f bundle.Dockerfile -t quay.io/ephico2real/namespace-configuration-operator-bundle:v1.2.6-custom .

# Push to registry
podman push quay.io/ephico2real/namespace-configuration-operator-bundle:v1.2.6-custom
```

#### Step 4: Create Custom Catalog

Create `custom-catalog.yaml`:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: custom-namespace-operator-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: quay.io/ephico2real/namespace-configuration-operator-index:v1.2.6
  displayName: Custom Namespace Configuration Operator
  publisher: Custom
  updateStrategy:
    registryPoll:
      interval: 10m
```

#### Step 5: Build and Push Index/Catalog Image

```bash
# Use opm (Operator Package Manager) to create index
opm index add \
  --bundles quay.io/ephico2real/namespace-configuration-operator-bundle:v1.2.6-custom \
  --tag quay.io/ephico2real/namespace-configuration-operator-index:v1.2.6

# Push index
podman push quay.io/ephico2real/namespace-configuration-operator-index:v1.2.6
```

#### Step 6: Apply Custom Catalog

```bash
# Create the custom catalog source
oc apply -f custom-catalog.yaml

# Wait for catalog to be ready
oc get catalogsource -n openshift-marketplace

# Update subscription to use custom catalog
oc patch subscription namespace-configuration-operator \
  -n namespace-configuration-operator \
  --type='merge' \
  -p='{
    "spec": {
      "source": "custom-namespace-operator-catalog",
      "sourceNamespace": "openshift-marketplace"
    }
  }'
```

**Benefits:**
- Proper OLM way
- Persists across updates
- Version controlled

**Drawbacks:**
- Complex setup
- Requires maintaining custom catalog
- Need to rebuild for each version

---

### Option 4: Use Subscription's relatedImages Override

Some operators support this, but it's not guaranteed.

```bash
# Edit subscription
oc edit subscription namespace-configuration-operator -n namespace-configuration-operator

# Add this to spec.config:
spec:
  config:
    env:
      - name: RELATED_IMAGE_MANAGER
        value: quay.io/ephico2real/namespace-configuration-operator:latest
```

**Note**: This works only if the operator code is designed to consume this environment variable. Most operators don't support this.

---

## Recommended Approach

**For your use case, stick with Option 2 (Kyverno) because:**

1. ✅ **Already implemented and working**
2. ✅ **Survives OLM updates automatically**
3. ✅ **Simple to maintain**
4. ✅ **No custom catalog needed**
5. ✅ **Works across all OLM operators**

## Verification

After any method, verify the image:

```bash
# Check Deployment
oc get deployment namespace-configuration-operator-controller-manager \
  -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[1].image}'

# Check actual running pod
oc get pods -n namespace-configuration-operator \
  -o jsonpath='{.items[*].spec.containers[1].image}'

# Both should show: quay.io/ephico2real/namespace-configuration-operator:latest
```

## Troubleshooting

### CSV Patch Not Taking Effect

```bash
# Check CSV status
oc get csv -n namespace-configuration-operator -o yaml

# Force reconciliation
oc delete pod -n namespace-configuration-operator -l app.kubernetes.io/name=namespace-configuration-operator
```

### Kyverno Policy Not Working

```bash
# Check policy status
oc get cpol replace-operator-image-to-dockerhub

# Check Kyverno logs
oc logs -n kyverno -l app.kubernetes.io/component=admission-controller --tail=50

# Restart deployment to trigger policy
oc rollout restart deployment namespace-configuration-operator-controller-manager \
  -n namespace-configuration-operator
```

### Custom Catalog Not Appearing

```bash
# Check catalog source
oc get catalogsource -n openshift-marketplace custom-namespace-operator-catalog

# Check catalog pod
oc get pods -n openshift-marketplace | grep custom-namespace-operator

# Check logs
oc logs -n openshift-marketplace <catalog-pod-name>
```

## Summary

| Method | Complexity | Persistence | OLM-Native | Recommended |
|--------|-----------|-------------|------------|-------------|
| **CSV Patch** | Low | Temporary | No | ❌ Testing only |
| **Kyverno** | Low | Permanent | No | ✅ **Best for you** |
| **Custom Catalog** | High | Permanent | Yes | ⚠️ If you need OLM-native |
| **Subscription Config** | Low | Varies | Yes | ❌ Rarely works |

## Current Status

✅ **You already have Kyverno policies in place** that handle this automatically!

Your Kyverno policy `replace-operator-image-to-dockerhub` is doing exactly what you need.
