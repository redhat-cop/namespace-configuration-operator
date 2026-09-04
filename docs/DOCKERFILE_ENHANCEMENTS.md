# Dockerfile Enhancements

This document describes the enhancements made to the operator's Dockerfile for production-ready builds, version information, and logging configuration.

## Overview

The Dockerfile includes several enhancements:
1. **Version Information Build Args** - Embed version, commit, and build date into the binary
2. **Log Level Environment Variables** - Set production defaults for logging configuration

## Version Information Build Args

The Dockerfile supports build-time arguments for embedding version information into the operator binary. This information is displayed in the operator's startup banner and helps with debugging and identifying deployed operator versions.

### Build Arguments

```dockerfile
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
```

**Arguments:**
- `VERSION`: Version string (typically from `git describe --tags --always --dirty`)
- `COMMIT`: Git commit hash (typically from `git rev-parse --short HEAD`)
- `BUILD_DATE`: Build timestamp in ISO 8601 format (typically from `date -u +%Y-%m-%dT%H:%M:%SZ`)

### Implementation

The build args are passed to the Go compiler via `-ldflags` to set values in the `internal/version` package:

```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build -a \
    -ldflags "-X github.com/redhat-cop/namespace-configuration-operator/internal/version.Version=${VERSION} \
              -X github.com/redhat-cop/namespace-configuration-operator/internal/version.Commit=${COMMIT} \
              -X github.com/redhat-cop/namespace-configuration-operator/internal/version.BuildDate=${BUILD_DATE}" \
    -o manager main.go
```

### Usage

#### Manual Build with Version Info

```bash
# Manual build with version info (local builds only)
podman build --build-arg VERSION=$(git describe --tags --always --dirty) \
             --build-arg COMMIT=$(git rev-parse --short HEAD) \
             --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
             -t namespace-configuration-operator:latest .
```

> **Important:** Manual build commands are for **local development builds only**. For production builds, always use the Makefile targets which handle version injection automatically.

#### Using Makefiles (Recommended)

The `Makefile` and `PodmanMakefile` automatically detect and pass version information.

**For binary builds:**
```bash
# Using Makefile
make build

# Using PodmanMakefile
make -f PodmanMakefile build
```

**For container builds:**
```bash
# Using PodmanMakefile (recommended - automatic version injection)
make -f PodmanMakefile podman-build

# The standard Makefile docker-build injects the same version info (build args); so does hack/push-quay.sh
```

The Makefiles automatically:
- Detect version from git tags or use "dev"
- Get commit hash from git
- Generate build date timestamp
- Pass all values as build args (PodmanMakefile) or ldflags (both)

**Example PodmanMakefile output:**
```
Building with version info: VERSION=v1.0.0, COMMIT=abc1234, BUILD_DATE=2025-12-10T10:30:00Z
```

**For detailed information about how Makefiles inject version information, see [MAKEFILE_VERSION_INJECTION.md](./MAKEFILE_VERSION_INJECTION.md).**

### Benefits

1. **Version Tracking**: Operator displays version information in startup banner
2. **Debugging**: Easy to identify which operator version is deployed
3. **Build Traceability**: Build date helps track when operator was built
4. **Compliance**: Version information helps with audit and compliance requirements

### Version Information Display

The operator displays version information in the startup banner:

```
╔══════════════════════════════════════════════════════════════════════════════╗
║  NAMESPACE CONFIGURATION OPERATOR                                           ║
║  VERSION:  v1.2.6-84-gbc60603                                                ║
║  COMMIT:   bc60603                                                           ║
║  BUILD:    2026-09-04T18:00:37Z                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
```
(the exact layout is `internal/version/version.go`'s `PrintStartupBanner`; it goes to stderr)

This information is available via:
- Operator logs (startup banner)
- `internal/version` package functions:
  - `GetVersion()` - Returns version string
  - `GetCommitHash()` - Returns commit hash
  - `GetBuildDate()` - Returns build date

## Log Level Environment Variables

The Dockerfile sets default environment variables for log configuration. These defaults provide sensible production settings but can be overridden at runtime.

### Default Environment Variables

```dockerfile
# Set default log level via environment variables
# These can be overridden at runtime via Deployment env section or ConfigMap
# See: https://sdk.operatorframework.io/docs/building-operators/golang/references/logging/
# Production defaults: info level, JSON format (ZAP_DEVEL=false)
ENV ZAP_LOG_LEVEL=info
ENV ZAP_DEVEL=false
```

**Environment Variables:**
- `ZAP_LOG_LEVEL`: Log verbosity level (default: `info`)
  - Valid values: `error`, `info`, `debug`, or numeric levels `0-10`
- `ZAP_DEVEL`: Development mode flag (default: `false`)
  - `false`: JSON format (production, works with ELK)
  - `true`: Console format (development, human-readable)

### Why Set Defaults in Dockerfile?

1. **Production-Ready Defaults**: Provides sensible defaults (info level, JSON format)
2. **Consistency**: Ensures consistent behavior if not explicitly configured
3. **Best Practices**: Follows Operator SDK recommendations for logging configuration
4. **Override Capability**: Can be overridden at runtime via:
   - Subscription `config.env` (for OLM-managed deployments)
   - Kyverno policies (for policy-based configuration)
   - Deployment spec (for manual deployments)

### Configuration Priority

The log level configuration follows this priority (highest to lowest):

1. **Subscription/Kyverno Environment Variables** - Runtime configuration (recommended for OLM)
2. **Dockerfile ENV Defaults** - Fallback if not explicitly configured
3. **Operator SDK Defaults** - Built-in defaults (debug level if `ZAP_DEVEL=true`)

**Important:** For OLM-managed deployments, always use Subscription or Kyverno policy to configure log levels. The Dockerfile defaults serve as a fallback but should be overridden for production use.

### Overriding Dockerfile Defaults

#### For OLM-Managed Deployments

**Method 1: Update Subscription (Recommended)**
```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: namespace-configuration-operator
  namespace: openshift-operators
spec:
  config:
    env:
    - name: ZAP_LOG_LEVEL
      value: "error"  # Override Dockerfile default
    - name: ZAP_DEVEL
      value: "false"
```

**Method 2: Use Kyverno Policy**
```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: configure-operator-log-level
spec:
  rules:
    - name: inject-log-level-env
      mutate:
        patchStrategicMerge:
          spec:
            template:
              spec:
                containers:
                  - name: manager
                    env:
                      - name: ZAP_LOG_LEVEL
                        value: "error"  # Override Dockerfile default
                      - name: ZAP_DEVEL
                        value: "false"
```

#### For Manual Deployments

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: namespace-configuration-operator-controller-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: ZAP_LOG_LEVEL
          value: "error"  # Override Dockerfile default
        - name: ZAP_DEVEL
          value: "false"
```

### Log Level Options

| ZAP_LOG_LEVEL | Shows | Use Case |
|---------------|-------|----------|
| `error` | Only errors | Production (minimal logging, reduces ELK volume) |
| `info` | Info and errors | Production (normal operations, includes deletion tracking) |
| `1` or `debug` | V(1) + info + errors | Development (shows skipping logs, retry success) |
| `2` | V(2) + V(1) + info + errors | Troubleshooting (shows template filtering details) |

**Enhanced Logging Features:**
- **V(1) skipping logs**: Visible with `ZAP_LOG_LEVEL=1` or higher
- **V(2) template filtering logs**: Visible with `ZAP_LOG_LEVEL=2` or higher
- **Info-level deletion tracking**: Always visible (info level)
- **V(1) retry success logs**: Visible with `ZAP_LOG_LEVEL=1` or higher

See [LOG_LEVEL_CONFIGURATION.md](./LOG_LEVEL_CONFIGURATION.md) for detailed log level configuration options.

## Base Image

The Dockerfile uses a minimal base image for security and size optimization:

```dockerfile
FROM registry.access.redhat.com/ubi9/ubi-minimal
```

**Benefits:**
- Minimal attack surface
- Smaller image size
- Red Hat certified base image
- Suitable for production use

## Security

The Dockerfile follows security best practices:

1. **Non-root User**: Runs as user `65532:65532` (non-root)
2. **Minimal Base Image**: Uses UBI minimal for reduced attack surface
3. **No Shell**: Distroless-style approach (no shell in final image)
4. **Build-time Args**: Version info passed at build time, not runtime

## Related Documentation

- [LOG_LEVEL_CONFIGURATION.md](./LOG_LEVEL_CONFIGURATION.md) - Detailed log level configuration guide
- [BUILD-RUN.md](../BUILD-RUN.md) - Build and run instructions
- [Resolved Issues Tracker](../resolved-issues-tracker/resolved-issues-tracker.md) - Version information system documentation

## Example: Complete Build with Version Info (Local Builds Only)

> **Important:** This example shows manual build commands for **local development builds only**. For production builds, use Makefile targets in your CI/CD pipeline.

```bash
# Get version information
VERSION=$(git describe --tags --always --dirty || echo "dev")
COMMIT=$(git rev-parse --short HEAD || echo "unknown")
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Build with version info (local builds only)
podman build \
  --build-arg VERSION="${VERSION}" \
  --build-arg COMMIT="${COMMIT}" \
  --build-arg BUILD_DATE="${BUILD_DATE}" \
  -t namespace-configuration-operator:${VERSION} \
  -t namespace-configuration-operator:latest \
  .

# Verify version info in image: --version prints the banner and exits 0 (no kubeconfig needed)
podman run --rm --entrypoint /manager namespace-configuration-operator:latest --version
```

**For production builds:** Use Makefile targets in your CI/CD pipeline:
```bash
# In CI/CD pipeline
make -f PodmanMakefile podman-build
# or
make -f PodmanMakefile external-deploy
```

## Troubleshooting

### Version Information Shows "dev" or "unknown"

**Problem:** Build args not being passed correctly.

**Solution:**
1. Check if using Makefile (it handles this automatically)
2. For manual builds, ensure build args are passed:
   ```bash
   podman build --build-arg VERSION=$(git describe --tags --always --dirty) ...
   ```
3. Verify build args in build output

### Log Level Not Taking Effect

**Problem:** Dockerfile ENV defaults are being used instead of runtime configuration.

**Solution:**
1. For OLM deployments, use Subscription or Kyverno policy (see [LOG_LEVEL_CONFIGURATION.md](./LOG_LEVEL_CONFIGURATION.md))
2. Verify environment variables in Deployment:
   ```bash
   oc get deployment namespace-configuration-operator-controller-manager \
     -n namespace-configuration-operator \
     -o jsonpath='{.spec.template.spec.containers[0].env}'
   ```
3. Check pod environment variables:
   ```bash
   oc exec -n namespace-configuration-operator \
     deployment/namespace-configuration-operator-controller-manager \
     -- env | grep ZAP
   ```
