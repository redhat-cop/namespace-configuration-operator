#!/usr/bin/env bash
# Build the operator image for linux/amd64 with the same version stamps the Makefile uses, and push
# it to quay. Separate from the Makefile's docker-build/docker-push on purpose: those hard-code
# `docker`, run the full test target first, and push one tag; this uses podman (or docker), assumes
# hack/local-ci.sh already passed, pushes an immutable tag and — only when asked — moves `latest`.
#
# Usage:
#   hack/push-quay.sh                 # tag = git describe (e.g. v1.2.6-83-gabc1234), pushes that tag only
#   hack/push-quay.sh v1.3.0          # explicit tag
#   PUSH_LATEST=1 hack/push-quay.sh   # also retag and push :latest — this is what running clusters pull
#
# Environment:
#   IMAGE_REPO      default quay.io/ephico2real/namespace-configuration-operator
#   CONTAINER_TOOL  podman (default if installed) or docker
#   PLATFORM        linux/amd64 (OpenShift nodes), even when building on arm64
#   PUSH_LATEST     set to 1 to also push :latest
#
# Requires a prior `podman login quay.io` (or docker login). Refuses a dirty tree unless ALLOW_DIRTY=1,
# because git describe stamps "-dirty" into the tag and the binary and that is rarely what you meant.
set -euo pipefail
cd "$(dirname "$0")/.."

IMAGE_REPO=${IMAGE_REPO:-quay.io/ephico2real/namespace-configuration-operator}
PLATFORM=${PLATFORM:-linux/amd64}
tool=${CONTAINER_TOOL:-$(command -v podman >/dev/null && echo podman || echo docker)}
registry=${IMAGE_REPO%%/*}

fail() { printf 'FAILED: %s\n' "$*" >&2; exit 1; }

if [ -z "${ALLOW_DIRTY:-}" ] && ! git diff --quiet HEAD --; then
  git --no-pager status --short
  fail "working tree is dirty; commit first, or set ALLOW_DIRTY=1 to stamp a -dirty build on purpose"
fi

version=$(git describe --tags --always --dirty)
tag=${1:-$version}
commit=$(git rev-parse --short HEAD)
build_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
image="$IMAGE_REPO:$tag"

if [ "$tool" = podman ]; then
  podman login --get-login "$registry" >/dev/null 2>&1 || fail "not logged in to $registry: run 'podman login $registry'"
fi

printf '==> building %s (VERSION=%s COMMIT=%s BUILD_DATE=%s PLATFORM=%s) with %s\n' "$image" "$version" "$commit" "$build_date" "$PLATFORM" "$tool"
# The same OCI labels .github/workflows/image.yaml stamps, so hack/ci-image.sh pull reads either build alike.
"$tool" build --platform "$PLATFORM" \
  --build-arg VERSION="$version" --build-arg COMMIT="$commit" --build-arg BUILD_DATE="$build_date" \
  --label org.opencontainers.image.version="$version" \
  --label org.opencontainers.image.revision="$(git rev-parse HEAD)" \
  --label org.opencontainers.image.created="$build_date" \
  --label org.opencontainers.image.ref.name="$(git rev-parse --abbrev-ref HEAD)" \
  --label org.opencontainers.image.source="local:$(hostname -s)" \
  -t "$image" .

printf '==> pushing %s\n' "$image"
"$tool" push "$image"

if [ -n "${PUSH_LATEST:-}" ]; then
  printf '==> pushing %s:latest\n' "$IMAGE_REPO"
  "$tool" tag "$image" "$IMAGE_REPO:latest"
  "$tool" push "$IMAGE_REPO:latest"
fi

# The digest is what to record in a change log or a values file; a tag can move, a digest cannot.
digest=$("$tool" inspect --format '{{ index .RepoDigests 0 }}' "$image" 2>/dev/null || true)
printf '\npushed %s\n' "$image"
[ -z "$digest" ] || printf 'digest %s\n' "$digest"
[ -z "${PUSH_LATEST:-}" ] || printf 'latest now points at this build\n'
