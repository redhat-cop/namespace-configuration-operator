#!/usr/bin/env bash
# The laptop side of .github/workflows/image.yaml: start a build, watch it, and pull what it built.
#
#   hack/ci-image.sh run [--latest] [--platforms linux/amd64,linux/arm64]
#       Dispatch the workflow for the CURRENT branch (pushed) and follow it. --latest moves :latest.
#   hack/ci-image.sh status
#       The last few runs of the workflow for this repository.
#   hack/ci-image.sh pull [tag]
#       Pull the image CI built — default tag is `git describe` for HEAD, i.e. this commit's build —
#       and print what it is (labels, digest, version stamp), so a local check or a cluster can use a
#       CI-built image instead of one built here. Refuses a dirty tree for the default tag, because
#       CI never built the uncommitted changes.
#
# Environment: IMAGE_REPO (default quay.io/ephico2real/namespace-configuration-operator),
# CONTAINER_TOOL (podman if installed, else docker). Needs `gh auth login` for run/status.
set -euo pipefail
cd "$(dirname "$0")/.."

IMAGE_REPO=${IMAGE_REPO:-quay.io/ephico2real/namespace-configuration-operator}
WORKFLOW=image.yaml
tool=${CONTAINER_TOOL:-$(command -v podman >/dev/null && echo podman || echo docker)}
repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null || echo ephico2real2/namespace-configuration-operator)

fail() { printf 'FAILED: %s\n' "$*" >&2; exit 1; }

cmd_run() {
  local latest=false platforms=linux/amd64 branch
  while [ $# -gt 0 ]; do
    case $1 in
      --latest) latest=true ;;
      --platforms) platforms=$2; shift ;;
      *) fail "unknown option $1" ;;
    esac
    shift
  done
  branch=$(git rev-parse --abbrev-ref HEAD)
  git diff --quiet HEAD -- || fail "working tree is dirty; CI builds the pushed branch, commit and push first"
  git fetch -q origin "$branch" || fail "branch $branch is not on origin yet; push it first"
  [ "$(git rev-parse HEAD)" = "$(git rev-parse "origin/$branch")" ] || fail "HEAD differs from origin/$branch; push first so CI builds what you have"
  printf '==> dispatching %s on %s (push_latest=%s, platforms=%s)\n' "$WORKFLOW" "$branch" "$latest" "$platforms"
  gh workflow run "$WORKFLOW" -R "$repo" --ref "$branch" -f push_latest="$latest" -f platforms="$platforms"
  # The run id is not returned by dispatch; wait for it to appear, then follow it.
  local id=""
  for _ in $(seq 1 30); do
    id=$(gh run list -R "$repo" --workflow "$WORKFLOW" --branch "$branch" --event workflow_dispatch -L 1 --json databaseId --jq '.[0].databaseId // empty')
    [ -n "$id" ] && break
    sleep 2
  done
  [ -n "$id" ] || fail "the run did not appear within 60s; check 'gh run list -R $repo'"
  gh run watch "$id" -R "$repo" --exit-status
  gh run view "$id" -R "$repo" --json conclusion,url --jq '"\(.conclusion)  \(.url)"'
}

cmd_status() {
  gh run list -R "$repo" --workflow "$WORKFLOW" -L 10
}

cmd_pull() {
  local tag=${1:-}
  if [ -z "$tag" ]; then
    git diff --quiet HEAD -- || fail "working tree is dirty, so no CI build matches it; pass the tag you want explicitly"
    tag=$(git describe --tags --always)
  fi
  local image="$IMAGE_REPO:$tag"
  printf '==> pulling %s with %s\n' "$image" "$tool"
  "$tool" pull -q "$image" >/dev/null || fail "pull failed: has CI built $tag? (hack/ci-image.sh status)"
  printf '\n%s\n' "$image"
  "$tool" inspect --format \
    'digest   {{ index .RepoDigests 0 }}
version  {{ index .Labels "org.opencontainers.image.version" }}
revision {{ index .Labels "org.opencontainers.image.revision" }}
ref      {{ index .Labels "org.opencontainers.image.ref.name" }}
created  {{ index .Labels "org.opencontainers.image.created" }}' "$image"
  # The binary's own stamp, from the startup banner, without starting the manager for real: the
  # banner prints before the manager looks for a kubeconfig, so the exit is expected and ignored.
  printf 'binary   '
  "$tool" run --rm --entrypoint /manager "$image" --help 2>&1 | grep -m1 -o 'VERSION:[^║]*' | sed 's/ *$//' || echo "(no banner)"
}

case ${1:-} in
  run) shift; cmd_run "$@" ;;
  status) cmd_status ;;
  pull) shift; cmd_pull "$@" ;;
  *) sed -n '2,17p' "$0"; exit 2 ;;
esac
