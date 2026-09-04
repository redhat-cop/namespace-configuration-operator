# CI/CD Version Injection

This document explains how version information (`VERSION`, `COMMIT`, and `BUILD_DATE`) is injected during CI/CD builds, including GitHub Actions workflows and different Dockerfile scenarios.

## Overview

Version injection works differently depending on the build context:
1. **Local builds** - Makefiles inject version info
2. **CI/CD builds** - GitHub Actions workflows inject version info
3. **Different Dockerfiles** - `Dockerfile` (full build) vs `ci.Dockerfile` (pre-built binary)

## Dockerfile Types

### Dockerfile (Full Build)

**Location:** `Dockerfile` (root directory)

**Purpose:** Complete build from source, includes Go build step

**How version injection works:**
1. Makefile/PodmanMakefile passes `--build-arg VERSION=... COMMIT=... BUILD_DATE=...`
2. Dockerfile receives as `ARG VERSION`, `ARG COMMIT`, `ARG BUILD_DATE`
3. Dockerfile passes to `go build` via `-ldflags`:
   ```dockerfile
   RUN CGO_ENABLED=0 GOOS=linux go build -a \
       -ldflags "-X ...Version=${VERSION} -X ...Commit=${COMMIT} -X ...BuildDate=${BUILD_DATE}" \
       -o manager main.go
   ```

**Used by:**
- Local builds via `make docker-build` or `make -f PodmanMakefile podman-build`
- Production builds that build from source

### ci.Dockerfile (Pre-built Binary)

**Location:** `ci.Dockerfile` (root directory)

**Purpose:** Minimal image that copies pre-built binary (used by Tilt for local development)

**Content:**
```dockerfile
FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /
COPY bin/manager .
USER 65532:65532
ENTRYPOINT ["/manager"]
```

**How version injection works:**
- **Version info must be injected during `go build` step** (before Docker build)
- The binary is built with version info via `make build` or direct `go build` with ldflags
- Dockerfile just copies the already-built binary

**Used by:**
- Tiltfile for local development
- CI/CD workflows that build binary separately

## GitHub Actions Workflows

### Workflow Structure

The project uses shared workflows from `redhat-cop/github-workflows-operators`:

**Files:**
- `.github/workflows/push.yaml` - Triggers on push to main/master and tags
- `.github/workflows/pr.yaml` - Triggers on pull requests

**Shared Workflow:** `redhat-cop/github-workflows-operators/.github/workflows/release-operator.yml`

### Which Dockerfile is Used in CI/CD?

**Answer: it depends on the workflow.**

- The shared `release-operator.yml` / `pr-operator.yml` from `redhat-cop/github-workflows-operators` (used by
  `push.yaml` / `pr.yaml`) build the binary with `make` and then package it with **`ci.Dockerfile`**
  (`file: "./ci.Dockerfile"` in those workflows), which only copies `bin/manager` onto the base image. That is
  why `ci.Dockerfile` carries the same `ZAP_*` ENV as the `Dockerfile`.
- The fork's own `.github/workflows/image.yaml` builds the full **`Dockerfile`** (Go build stage included) and
  passes `VERSION`, `COMMIT` and `BUILD_DATE` as build args.

**Why `Dockerfile` and not `ci.Dockerfile`?**
- `Dockerfile` is the standard production Dockerfile with full build process
- `ci.Dockerfile` is minimal and expects a pre-built binary (used by Tilt for fast local iteration)
- CI/CD workflows need a complete, reproducible build from source

### How Version Injection Works in CI/CD

The shared workflow typically:
1. **Detects version from git:**
   - Uses `git describe --tags --always --dirty` for version
   - Uses `git rev-parse --short HEAD` for commit
   - Uses `date -u +"%Y-%m-%dT%H:%M:%SZ"` for build date

2. **Builds Docker image with build args:**
   ```bash
   docker build --build-arg VERSION=${VERSION} --build-arg COMMIT=${COMMIT} --build-arg BUILD_DATE=${BUILD_DATE} -t ${IMAGE} .
   ```
   - Uses the root `Dockerfile` (default)
   - Passes version info as build args
   - Dockerfile receives args and passes to `go build` via ldflags

3. **Dockerfile builds binary with version info:**
   ```dockerfile
   RUN CGO_ENABLED=0 GOOS=linux go build -a \
       -ldflags "-X ...Version=${VERSION} -X ...Commit=${COMMIT} -X ...BuildDate=${BUILD_DATE}" \
       -o manager main.go
   ```

### Example CI/CD Build Command

The shared workflow would execute something like:
```bash
# Set version variables
VERSION=$(git describe --tags --always --dirty || echo "dev")
COMMIT=$(git rev-parse --short HEAD || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build binary with version info
go build -ldflags "-X github.com/redhat-cop/namespace-configuration-operator/internal/version.Version=${VERSION} -X github.com/redhat-cop/namespace-configuration-operator/internal/version.Commit=${COMMIT} -X github.com/redhat-cop/namespace-configuration-operator/internal/version.BuildDate=${BUILD_DATE}" -o bin/manager main.go

# Build Docker image (if using Dockerfile with build args)
docker build --build-arg VERSION=${VERSION} --build-arg COMMIT=${COMMIT} --build-arg BUILD_DATE=${BUILD_DATE} -t ${IMAGE} .

# Or build Docker image (if using ci.Dockerfile - binary already has version)
docker build -f ci.Dockerfile -t ${IMAGE} .
```

## Makefile docker-build (Updated)

The `Makefile` `docker-build` target now injects version information:

```makefile
.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	@BUILD_VERSION=$${VERSION:-$$(git describe --tags --always --dirty 2>/dev/null || echo "$(VERSION)")}; \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	echo "Building with version info: VERSION=$$BUILD_VERSION, COMMIT=$$COMMIT, BUILD_DATE=$$BUILD_DATE"; \
	docker build --build-arg VERSION=$$BUILD_VERSION --build-arg COMMIT=$$COMMIT --build-arg BUILD_DATE=$$BUILD_DATE -t ${IMG} .
```

**Features:**
- ✅ Automatic version detection from git
- ✅ Passes build args to Dockerfile
- ✅ Works with `docker=podman` alias
- ✅ Consistent with PodmanMakefile approach

## Comparison: Makefile vs PodmanMakefile

| Feature | Makefile | PodmanMakefile |
|---------|----------|----------------|
| **Version injection** | ✅ Yes (updated) | ✅ Yes |
| **Container runtime** | Docker only | Podman/Docker auto-detect |
| **Build args** | ✅ Passes to docker build | ✅ Passes to podman/docker build |
| **Version display** | Shows during build | Shows during build |

## CI/CD Best Practices

### For GitHub Actions Workflows

1. **Use git environment variables:**
   ```yaml
   env:
     VERSION: ${{ github.ref_name }}
     COMMIT: ${{ github.sha }}
     BUILD_DATE: ${{ github.event.head_commit.timestamp }}
   ```

2. **Or detect from git in workflow:**
   ```yaml
   - name: Set version variables
     run: |
       echo "VERSION=$(git describe --tags --always --dirty)" >> $GITHUB_ENV
       echo "COMMIT=$(git rev-parse --short HEAD)" >> $GITHUB_ENV
       echo "BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")" >> $GITHUB_ENV
   ```

3. **Build with version info:**
   ```yaml
   - name: Build binary
     run: |
       go build -ldflags "-X ...Version=${VERSION} -X ...Commit=${COMMIT} -X ...BuildDate=${BUILD_DATE}" -o bin/manager main.go
   ```

4. **Build Docker image:**
   ```yaml
   - name: Build Docker image
     run: |
       docker build --build-arg VERSION=${VERSION} --build-arg COMMIT=${COMMIT} --build-arg BUILD_DATE=${BUILD_DATE} -t ${IMAGE} .
   ```

### For Custom CI/CD Pipelines

1. **Always inject version info** - Don't rely on Dockerfile defaults
2. **Use git for version detection** - Most reliable source
3. **Pass build args explicitly** - Don't assume defaults
4. **Verify version in image** - Check startup banner or binary strings

## Tiltfile (Local Development)

The `Tiltfile` uses `ci.Dockerfile` for local development:

```python
custom_build(
  image,
  'podman build -t $EXPECTED_REF --ignorefile ci.Dockerfile.dockerignore -f ./ci.Dockerfile .  && podman push $EXPECTED_REF $EXPECTED_REF',
  entrypoint=['/manager'],
  deps=['./bin'],
  ...
)
```

**How version injection works:**
1. Tiltfile compiles binary: `go build -o bin/manager main.go`
2. **Version info should be added to compile command:**
   ```python
   compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X ...Version=${VERSION} -X ...Commit=${COMMIT} -X ...BuildDate=${BUILD_DATE}" -o bin/manager main.go'
   ```
3. `ci.Dockerfile` copies the pre-built binary

**Note:** Current Tiltfile doesn't inject version info. To add it, update the `compile_cmd` to include ldflags.

## Verification

### Check Version in Built Image

```bash
# Run container and check startup banner
docker run --rm <image> /manager

# Or check binary strings
docker run --rm <image> strings /manager | grep -E "(v[0-9]|abc1234|2025-12)"
```

### Check Version in CI/CD Logs

Look for:
- Version info in build logs
- Startup banner in container logs
- Image metadata

## Troubleshooting

### Version Shows "dev" or "unknown" in CI/CD

**Problem:** Version info not being injected in CI/CD.

**Solutions:**
1. **Check workflow variables:**
   ```yaml
   - name: Debug version
     run: |
       echo "VERSION=${VERSION}"
       echo "COMMIT=${COMMIT}"
       echo "BUILD_DATE=${BUILD_DATE}"
   ```

2. **Verify git is available:**
   ```yaml
   - name: Check git
     run: |
       git describe --tags --always --dirty
       git rev-parse --short HEAD
   ```

3. **Check build command includes ldflags:**
   ```bash
   go build -ldflags "-X ...Version=${VERSION} ..." -o bin/manager main.go
   ```

### ci.Dockerfile Not Getting Version Info

**Problem:** Using `ci.Dockerfile` but binary doesn't have version info.

**Solution:** Ensure the `go build` step (before Docker build) includes ldflags:
```bash
go build -ldflags "-X ...Version=${VERSION} -X ...Commit=${COMMIT} -X ...BuildDate=${BUILD_DATE}" -o bin/manager main.go
```

## Related Documentation

- [MAKEFILE_VERSION_INJECTION.md](./MAKEFILE_VERSION_INJECTION.md) - How Makefiles inject version info
- [DOCKERFILE_ENHANCEMENTS.md](./DOCKERFILE_ENHANCEMENTS.md) - Dockerfile build args and version info
- [BUILD-RUN.md](../BUILD-RUN.md) - Build and run instructions
