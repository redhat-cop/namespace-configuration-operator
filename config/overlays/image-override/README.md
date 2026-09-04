# Image Override Kustomize Overlay

This overlay allows you to override the namespace-configuration-operator manager image without using Kyverno policies.

## Purpose

Replace the default operator image with a custom image from Quay.io or any other registry.

## Usage

### Option 1: Direct Apply

```bash
# Apply the overlay directly
oc apply -k config/overlays/image-override/
```

### Option 2: Preview Changes First

```bash
# Preview what will be applied
oc kustomize config/overlays/image-override/ | less

# Or save to a file
oc kustomize config/overlays/image-override/ > /tmp/operator-custom-image.yaml
oc apply -f /tmp/operator-custom-image.yaml
```

### Option 3: Build and Apply

```bash
# Build with kustomize CLI
kustomize build config/overlays/image-override/ | oc apply -f -
```

## Customization

To change the image, edit `kustomization.yaml`:

```yaml
images:
  - name: controller
    newName: quay.io/YOUR_USERNAME/namespace-configuration-operator
    newTag: YOUR_TAG  # e.g., v1.2.6, latest, dev
```

### Using a Specific Version Tag

```yaml
images:
  - name: controller
    newName: quay.io/ephico2real/namespace-configuration-operator
    newTag: v1.2.6
```

### Using a Digest

```yaml
images:
  - name: controller
    newName: quay.io/ephico2real/namespace-configuration-operator
    digest: sha256:49ed7d6155342adaa2b12fd80c6761c3081d8e6149d187cb7ff91a247cdf2e7a
```

## How It Works

1. **Base Reference**: Uses `../../default` as the base configuration
2. **Image Override**: Replaces the `controller` image placeholder with your custom image
3. **ImagePullPolicy Patch**: Sets `imagePullPolicy: Always` for `latest` tag to ensure fresh pulls

## Verification

After applying, verify the image change:

```bash
# Check the deployment. Address the container by NAME: containers[0] is kube-rbac-proxy.
oc get deployment namespace-configuration-operator-controller-manager \
  -n namespace-configuration-operator \
  -o jsonpath='{.spec.template.spec.containers[?(@.name=="manager")].image}{" "}{.spec.template.spec.containers[?(@.name=="manager")].imagePullPolicy}'

# Should output: quay.io/ephico2real/namespace-configuration-operator:latest Always

# Check pods are using the new image
oc get pods -n namespace-configuration-operator \
  -o jsonpath='{.items[*].spec.containers[?(@.name=="manager")].image}'

# Or, without a cluster, render the overlay:
kubectl kustomize config/overlays/image-override | grep -B1 -A2 'name: manager'
```

## Rollback

To rollback to the default image:

```bash
# Reapply the default configuration
oc apply -k config/default/
```

## Comparison with Kyverno

| Method | Pros | Cons |
|--------|------|------|
| **Kustomize Overlay** | Direct control, no dependencies, GitOps-friendly | Requires reapply for changes, OLM may revert |
| **Kyverno Policy** | Automatic enforcement, survives OLM updates | Requires Kyverno, additional complexity |
| **Subscription Config** | OLM-native, simple | Limited to env vars, image override not guaranteed |

## When to Use This

✅ **Use Kustomize Overlay when:**
- You don't have Kyverno installed
- You want direct, explicit image control
- You're using GitOps (ArgoCD, Flux)
- You're testing custom builds

❌ **Don't use when:**
- Operator is OLM-managed (use Subscription config or Kyverno instead)
- You need automatic enforcement across updates

## Notes

- This overlay is designed for **non-OLM deployments**
- For OLM-managed operators, use Kyverno policies or Subscription configuration
- The `imagePullPolicy: Always` ensures latest images are always pulled
- Consider using specific tags (not `latest`) for production
