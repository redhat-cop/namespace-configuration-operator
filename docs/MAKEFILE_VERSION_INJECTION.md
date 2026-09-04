# Makefile Version Information Injection

This document explains how the `Makefile` and `PodmanMakefile` automatically detect and inject version information (`VERSION`, `COMMIT`, and `BUILD_DATE`) into the operator binary during the build process.

## Overview

Both Makefiles automatically:
1. **Detect version information** from git (or use defaults)
2. **Pass build args** to the Dockerfile build process
3. **Embed version info** into the binary via Go ldflags

This ensures that every build includes accurate version, commit, and build date information without manual intervention.

## How It Works

### Version Detection Logic

Both Makefiles use the same logic to detect version information:

```makefile
BUILD_VERSION=$${VERSION:-$$(git describe --tags --always --dirty 2>/dev/null || echo "$(VERSION)")}
COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ")
```

**Priority order:**
1. **VERSION**: Uses `VERSION` environment variable if set, otherwise tries `git describe --tags --always --dirty`, falls back to Makefile `VERSION` variable (default: `0.0.1`)
2. **COMMIT**: Uses `git rev-parse --short HEAD`, falls back to `"unknown"` if git is unavailable
3. **BUILD_DATE**: Always generated from current UTC time in ISO 8601 format

### Makefile Implementation

#### Binary Build (`make build`)

The `build` target in both Makefiles injects version info directly into the Go binary:

**Makefile (lines 142-145):**
```makefile
.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	@BUILD_VERSION=$${VERSION:-$$(git describe --tags --always --dirty 2>/dev/null || echo "$(VERSION)")}; \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	go build -buildvcs -ldflags "-X github.com/redhat-cop/namespace-configuration-operator/internal/version.Version=$$BUILD_VERSION -X github.com/redhat-cop/namespace-configuration-operator/internal/version.Commit=$$COMMIT -X github.com/redhat-cop/namespace-configuration-operator/internal/version.BuildDate=$$BUILD_DATE" -o bin/manager main.go
```

**PodmanMakefile (lines 207-210):**
```makefile
.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	@BUILD_VERSION=$${VERSION:-$$(git describe --tags --always --dirty 2>/dev/null || echo "$(VERSION)")}; \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	go build -buildvcs -ldflags "-X github.com/redhat-cop/namespace-configuration-operator/internal/version.Version=$$BUILD_VERSION -X github.com/redhat-cop/namespace-configuration-operator/internal/version.Commit=$$COMMIT -X github.com/redhat-cop/namespace-configuration-operator/internal/version.BuildDate=$$BUILD_DATE" -o bin/manager main.go
```

**How it works:**
1. Sets shell variables `BUILD_VERSION`, `COMMIT`, and `BUILD_DATE`
2. Passes them to `go build` via `-ldflags` to set package variables at link time
3. The `internal/version` package receives these values

#### Container Image Build

For container builds, the Makefiles use a different approach via the `container_build` function (PodmanMakefile) or direct docker build (Makefile).

**PodmanMakefile `container_build` function (lines 108-119):**
```makefile
define container_build
	$(call detect_container_runtime)
	@BUILD_VERSION=$${VERSION:-$$(git describe --tags --always --dirty 2>/dev/null || echo "$(VERSION)")}; \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	echo "Building with version info: VERSION=$$BUILD_VERSION, COMMIT=$$COMMIT, BUILD_DATE=$$BUILD_DATE"; \
	if podman info >/dev/null 2>&1; then \
		podman build --build-arg VERSION=$$BUILD_VERSION --build-arg COMMIT=$$COMMIT --build-arg BUILD_DATE=$$BUILD_DATE -t "$(1)" .; \
	elif docker info >/dev/null 2>&1; then \
		docker build --build-arg VERSION=$$BUILD_VERSION --build-arg COMMIT=$$COMMIT --build-arg BUILD_DATE=$$BUILD_DATE -t "$(1)" .; \
	fi
endef
```

**How it works:**
1. Detects container runtime (podman or docker)
2. Sets shell variables with version information
3. Prints the version info being used (for visibility)
4. Passes build args to `podman build` or `docker build`
5. Dockerfile receives these as `ARG VERSION`, `ARG COMMIT`, `ARG BUILD_DATE`

**Makefile `docker-build` target:**
```makefile
.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	@BUILD_VERSION=$${VERSION:-$$(git describe --tags --always --dirty 2>/dev/null || echo "$(VERSION)")}; \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ"); \
	docker build --build-arg VERSION=$$BUILD_VERSION --build-arg COMMIT=$$COMMIT --build-arg BUILD_DATE=$$BUILD_DATE -t ${IMG} .
```

**Note:** `docker-build` passes the three build args itself (it has since commit 2e80123). `hack/push-quay.sh`
does the same with podman, and `.github/workflows/image.yaml` in CI. Both `build` and the Dockerfile compile the
package (`.`), not `main.go`, so a build without ldflags still reads the commit from Go's VCS build settings.

## Variable Injection Flow

### For Binary Builds (`make build`)

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Makefile detects version info from git                    │
│    BUILD_VERSION=$(git describe --tags --always --dirty)     │
│    COMMIT=$(git rev-parse --short HEAD)                      │
│    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")              │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Pass to go build via -ldflags                            │
│    -X internal/version.Version=${BUILD_VERSION}             │
│    -X internal/version.Commit=${COMMIT}                     │
│    -X internal/version.BuildDate=${BUILD_DATE}              │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Go linker sets package variables at link time            │
│    internal/version.Version = "v1.0.0"                      │
│    internal/version.Commit = "abc1234"                      │
│    internal/version.BuildDate = "2025-12-10T10:30:00Z"     │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Binary contains version info (displayed in startup)      │
└─────────────────────────────────────────────────────────────┘
```

### For Container Builds (`make -f PodmanMakefile podman-build`)

```
┌─────────────────────────────────────────────────────────────┐
│ 1. PodmanMakefile detects version info from git             │
│    BUILD_VERSION=$(git describe --tags --always --dirty)   │
│    COMMIT=$(git rev-parse --short HEAD)                    │
│    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")            │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Pass to container build as --build-arg                   │
│    podman build --build-arg VERSION=${BUILD_VERSION}        │
│              --build-arg COMMIT=${COMMIT}                   │
│              --build-arg BUILD_DATE=${BUILD_DATE}           │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Dockerfile receives as ARG variables                     │
│    ARG VERSION=dev                                           │
│    ARG COMMIT=unknown                                        │
│    ARG BUILD_DATE=unknown                                    │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ 4. Dockerfile passes to go build via -ldflags                │
│    -ldflags "-X ...Version=${VERSION} ..."                  │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ 5. Binary contains version info (displayed in startup)      │
└─────────────────────────────────────────────────────────────┘
```

## Usage Examples

> **Important:** All manual commands shown in this document are for **local builds only**. For CI/CD pipelines, production builds, or automated builds, use the Makefile targets which handle version injection automatically.

### Build Binary with Version Info

```bash
# Using Makefile
make build

# Using PodmanMakefile (same command)
make -f PodmanMakefile build
```

**Output:**
- Binary created at `bin/manager`
- Version info embedded via ldflags
- No visible output (version info is in binary)

**Note:** These commands are for local development builds only.

### Build Container Image with Version Info

```bash
# Using PodmanMakefile (automatic version injection)
make -f PodmanMakefile podman-build

# Output shows version info:
# Building with version info: VERSION=v1.0.0, COMMIT=abc1234, BUILD_DATE=2025-12-10T10:30:00Z
```

**Note:** This command is for local development builds only. For production builds, use your CI/CD pipeline which should call the Makefile targets.

### Override Version Information

You can override version information via environment variables (for local builds only):

```bash
# Override VERSION only
VERSION=v2.0.0 make -f PodmanMakefile podman-build

# Override all variables (not recommended - COMMIT and BUILD_DATE should be auto-detected)
VERSION=v2.0.0 COMMIT=xyz789 BUILD_DATE=2025-12-11T00:00:00Z make -f PodmanMakefile podman-build
```

**Note:** 
- `COMMIT` and `BUILD_DATE` are typically auto-detected. Only override `VERSION` if needed.
- These override commands are for **local development builds only**. Production builds should use CI/CD pipelines with proper version management.

## Version Detection Details

### VERSION Variable

**Detection priority:**
1. `VERSION` environment variable (if set)
2. `git describe --tags --always --dirty` (if git repo available)
3. Makefile `VERSION` variable (default: `0.0.1`)

**Examples:**
- Tagged release: `v1.0.0`
- Tagged with commits: `v1.0.0-5-gabc1234`
- No tags: `abc1234-dirty` (commit hash with -dirty if uncommitted changes)
- No git: `0.0.1` (Makefile default)

### COMMIT Variable

**Detection:**
- `git rev-parse --short HEAD` (7-character commit hash)
- Falls back to `"unknown"` if git unavailable

**Examples:**
- `abc1234` (short commit hash)
- `unknown` (if not a git repo)

### BUILD_DATE Variable

**Detection:**
- Always generated: `date -u +"%Y-%m-%dT%H:%M:%SZ"`
- UTC timezone, ISO 8601 format

**Examples:**
- `2025-12-10T10:30:00Z`
- `2025-12-10T15:45:23Z`

## Differences Between Makefiles

| Feature | Makefile | PodmanMakefile |
|---------|----------|----------------|
| **Binary build** | ✅ Automatic version injection | ✅ Automatic version injection |
| **Container build** | ✅ Automatic version injection (build args) | ✅ Automatic version injection |
| **Container runtime** | Docker only | Podman/Docker auto-detect |
| **Version display** | No build output | Shows version info during build |

**Recommendation:** Use `PodmanMakefile` for container builds to get automatic version injection.

## Integration with Dockerfile

The Makefiles work seamlessly with the Dockerfile's build args:

**Dockerfile (lines 25-32):**
```dockerfile
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -a \
    -ldflags "-X github.com/redhat-cop/namespace-configuration-operator/internal/version.Version=${VERSION} \
              -X github.com/redhat-cop/namespace-configuration-operator/internal/version.Commit=${COMMIT} \
              -X github.com/redhat-cop/namespace-configuration-operator/internal/version.BuildDate=${BUILD_DATE}" \
    -o manager main.go
```

**Flow:**
1. Makefile/PodmanMakefile passes `--build-arg VERSION=...` etc.
2. Dockerfile receives as `ARG VERSION=...` (overrides defaults)
3. Dockerfile uses `${VERSION}` in ldflags
4. Go linker sets package variables

## Troubleshooting

### Version Shows "dev" or "unknown"

**Problem:** Version information not being detected.

**Solutions:**
1. **Check git repository:**
   ```bash
   git status
   git describe --tags --always --dirty
   git rev-parse --short HEAD
   ```

2. **Verify Makefile is being used:**
   ```bash
   # Use PodmanMakefile for container builds (local builds only)
   make -f PodmanMakefile podman-build
   ```

3. **Check build output:**
   ```bash
   # PodmanMakefile shows version info (local builds only)
   make -f PodmanMakefile podman-build
   # Should show: "Building with version info: VERSION=..."
   ```

4. **Manual override (local builds only):**
   ```bash
   VERSION=v1.0.0 make -f PodmanMakefile podman-build
   ```

**Note:** For production builds, ensure your CI/CD pipeline uses Makefile targets and has access to git repository for version detection.

### Container Build Not Using Version Info

**Problem:** Using standard Makefile `docker-build` which doesn't inject version info.

**Solution:** Use PodmanMakefile instead (for local builds):
```bash
# Instead of:
make docker-build

# Use (local builds only):
make -f PodmanMakefile podman-build
```

**For production builds:** Ensure your CI/CD pipeline uses PodmanMakefile targets or manually passes build args.

### Version Info Not in Binary

**Problem:** Binary doesn't show version in startup banner.

**Solutions:**
1. **Verify binary was built with version info:**
   ```bash
   # Check if version package has values
   strings bin/manager | grep -E "(v[0-9]|abc1234|2025-12)"
   ```

2. **Rebuild with explicit version:**
   ```bash
   make clean
   make build
   ```

3. **Check internal/version package:**
   ```bash
   go run -ldflags "-X github.com/redhat-cop/namespace-configuration-operator/internal/version.Version=test" main.go
   ```

## Best Practices

1. **For local development:** Use PodmanMakefile for container builds - Automatic version injection
2. **For production builds:** Use Makefile targets in CI/CD pipelines - Ensures consistent version injection
3. **Don't override COMMIT or BUILD_DATE** - Let Makefiles auto-detect
4. **Use VERSION override only when needed** - For local testing or specific version requirements
5. **Verify version info after build** - Check startup banner or binary strings
6. **Use git tags for releases** - Enables `git describe` to work correctly
7. **CI/CD pipelines should use Makefile targets** - Don't use manual build commands in production

## Related Documentation

- [DOCKERFILE_ENHANCEMENTS.md](./DOCKERFILE_ENHANCEMENTS.md) - Dockerfile build args and version info
- [BUILD-RUN.md](../BUILD-RUN.md) - Build and run instructions
- [Resolved Issues Tracker](../resolved-issues-tracker/resolved-issues-tracker.md) - Version information system

## Code References

- **Makefile**: Lines 142-145 (build target)
- **PodmanMakefile**: 
  - Lines 108-119 (`container_build` function)
  - Lines 207-210 (build target)
- **Dockerfile**: Lines 25-32 (ARG declarations and ldflags)
