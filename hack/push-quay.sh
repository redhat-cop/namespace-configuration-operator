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
# Requires a prior `podman login quay.io` (or docker login). Refuses a tree with modified OR untracked
# files under the paths the Dockerfile ships unless ALLOW_DIRTY=1, because those files would be baked
# into an image whose tag claims a clean commit.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=lib.sh
. hack/lib.sh

IMAGE_REPO=${IMAGE_REPO:-quay.io/ephico2real/namespace-configuration-operator}
PLATFORM=${PLATFORM:-linux/amd64}
tool=${CONTAINER_TOOL:-$(command -v podman >/dev/null && echo podman || echo docker)}
registry=${IMAGE_REPO%%/*}

fail() { printf 'FAILED: %s\n' "$*" >&2; exit 1; }

if [ -z "${ALLOW_DIRTY:-}" ] && ! tree_is_clean; then
  dirty_summary
  fail "shipped paths have modified or untracked files; commit them, or set ALLOW_DIRTY=1 to bake them in on purpose"
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

# The digest worth recording is the one the REGISTRY assigns on push. Podman's RepoDigests is the
# local-storage digest and differs from it (podman converts the manifest on push), so a values
# file pinned to that would fail to pull. --digestfile captures the registry's answer.
digest=""
printf '==> pushing %s\n' "$image"
if [ "$tool" = podman ]; then
  digestfile=$(mktemp); trap 'rm -f "$digestfile"' EXIT
  podman push --digestfile "$digestfile" "$image"
  digest=$(cat "$digestfile")
else
  docker push "$image"
  digest=$(docker inspect --format '{{ index .RepoDigests 0 }}' "$image" 2>/dev/null | sed 's/.*@//' || true)
fi

if [ -n "${PUSH_LATEST:-}" ]; then
  printf '==> pushing %s:latest\n' "$IMAGE_REPO"
  "$tool" tag "$image" "$IMAGE_REPO:latest"
  "$tool" push "$IMAGE_REPO:latest"
fi

printf '\npushed %s\n' "$image"
[ -z "$digest" ] || printf 'digest %s@%s   (registry digest; pin this, a tag can move)\n' "$IMAGE_REPO" "$digest"
[ -z "${PUSH_LATEST:-}" ] || printf 'latest now points at this build\n'
