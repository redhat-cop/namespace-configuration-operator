#!/bin/bash
# Build wrapper script that automatically sets version, commit, and build date
# Usage: ./build.sh [go build arguments...]
#
# This script automatically injects version information via ldflags.
# You can pass any additional go build arguments after the script name.
#
# Examples:
#   ./build.sh -o bin/manager main.go
#   ./build.sh -race -o bin/manager main.go
#   ./build.sh -tags debug -o bin/manager main.go

set -e

# Get version information
BUILD_VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "0.0.1")}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

# Build ldflags
LDFLAGS="-X github.com/redhat-cop/namespace-configuration-operator/internal/version.Version=${BUILD_VERSION} \
         -X github.com/redhat-cop/namespace-configuration-operator/internal/version.Commit=${COMMIT} \
         -X github.com/redhat-cop/namespace-configuration-operator/internal/version.BuildDate=${BUILD_DATE}"

# Show what we're building with (unless quiet mode)
if [[ "$*" != *"-q"* ]] && [[ "$*" != *"--quiet"* ]]; then
    echo "Building with version info:"
    echo "  VERSION:    ${BUILD_VERSION}"
    echo "  COMMIT:     ${COMMIT}"
    echo "  BUILD_DATE: ${BUILD_DATE}"
    echo ""
fi

# Execute go build with ldflags and any additional arguments
exec go build -buildvcs -ldflags "${LDFLAGS}" "$@"

