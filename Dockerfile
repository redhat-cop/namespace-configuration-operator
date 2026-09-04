# Build the manager binary
# Supported Go line, pinned by the multi-arch index digest (golang:1.21 was EOL and last published
# 2024-08-13). Renovate keeps the digest current.
FROM docker.io/library/golang:1.26@sha256:9d2f36f06329b2a141b9db99ffa32765cf695ee57b813ca29e245e8670bcbfff AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/

# Build with version information. The PACKAGE (`.`) is built, not main.go: a file argument compiles
# as `command-line-arguments` and carries no vcs.* build settings. There is no .git in this context,
# so the ldflags below are the only source of the stamps here; the fallbacks matter for `make build`.
# Note: These args should be passed at build time for accurate version info.
# The Makefile handles this automatically. For manual builds, use:
#   podman build --build-arg VERSION=$(git describe --tags --always --dirty) \
#                --build-arg COMMIT=$(git rev-parse --short HEAD) \
#                --build-arg BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
#                -t myimage .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -a \
    -ldflags "-X github.com/redhat-cop/namespace-configuration-operator/internal/version.Version=${VERSION} \
              -X github.com/redhat-cop/namespace-configuration-operator/internal/version.Commit=${COMMIT} \
              -X github.com/redhat-cop/namespace-configuration-operator/internal/version.BuildDate=${BUILD_DATE}" \
    -o manager .

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.6@sha256:34880b64c07f28f64d95737f82f891516de9a3b43583f39970f7bf8e4cfa48b7
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

# Set default log level via environment variables
# These can be overridden at runtime via Deployment env section or ConfigMap
# See: https://sdk.operatorframework.io/docs/building-operators/golang/references/logging/
# Production defaults: info level, JSON format (ZAP_DEVEL=false)
ENV ZAP_LOG_LEVEL=info
ENV ZAP_DEVEL=false

ENTRYPOINT ["/manager"]
