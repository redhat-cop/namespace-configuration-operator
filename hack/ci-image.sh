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
# shellcheck source=lib.sh
. hack/lib.sh

IMAGE_REPO=${IMAGE_REPO:-quay.io/ephico2real/namespace-configuration-operator}
WORKFLOW=image.yaml
tool=${CONTAINER_TOOL:-$(command -v podman >/dev/null && echo podman || echo docker)}
repo=$(origin_repo || echo ephico2real2/namespace-configuration-operator)

fail() { printf 'FAILED: %s\n' "$*" >&2; exit 1; }

newest_dispatch_run() {
  gh run list -R "$repo" --workflow "$WORKFLOW" --branch "$1" --event workflow_dispatch -L 1 --json databaseId --jq '.[0].databaseId // empty'
}

cmd_run() {
  local latest=false platforms=linux/amd64 branch
  while [ $# -gt 0 ]; do
    case $1 in
      --latest) latest=true ;;
      --platforms)
        [ $# -ge 2 ] && [ -n "$2" ] || fail "--platforms requires a value"
        platforms=$2; shift ;;
      *) fail "unknown option $1" ;;
    esac
    shift
  done
  branch=$(git rev-parse --abbrev-ref HEAD)
  tree_is_clean || { dirty_summary; fail "shipped paths have modified or untracked files; CI builds the pushed branch, commit and push first"; }
  git fetch -q origin "$branch" || fail "branch $branch is not on origin yet; push it first"
  [ "$(git rev-parse HEAD)" = "$(git rev-parse "origin/$branch")" ] || fail "HEAD differs from origin/$branch; push first so CI builds what you have"
  # Dispatch returns no run id, so remember the newest run BEFORE dispatching and wait for a
  # different one; otherwise a previous dispatch on this branch would be followed instead.
  local before id=""
  before=$(newest_dispatch_run "$branch")
  printf '==> dispatching %s on %s (push_latest=%s, platforms=%s)\n' "$WORKFLOW" "$branch" "$latest" "$platforms"
  gh workflow run "$WORKFLOW" -R "$repo" --ref "$branch" -f push_latest="$latest" -f platforms="$platforms"
  for _ in $(seq 1 30); do
    id=$(newest_dispatch_run "$branch")
    [ -n "$id" ] && [ "$id" != "$before" ] && break
    id=""
    sleep 2
  done
  [ -n "$id" ] || fail "the new run did not appear within 60s; check 'gh run list -R $repo'"
  printf '==> following run %s\n' "$id"
  gh run watch "$id" -R "$repo" --exit-status
  gh run view "$id" -R "$repo" --json conclusion,url --jq '"\(.conclusion)  \(.url)"'
}

cmd_status() {
  gh run list -R "$repo" --workflow "$WORKFLOW" -L 10
}

cmd_pull() {
  local tag=${1:-}
  if [ -z "$tag" ]; then
    tree_is_clean || { dirty_summary; fail "shipped paths have modified or untracked files, so no CI build matches them; pass the tag you want explicitly"; }
    tag=$(git describe --tags --always)
  fi
  local image="$IMAGE_REPO:$tag"
  printf '==> pulling %s with %s\n' "$image" "$tool"
  "$tool" pull -q "$image" >/dev/null || fail "pull failed: has CI built $tag? (hack/ci-image.sh status)"
  printf '\n%s\n' "$image"
  # With provenance attestations off in the workflow the tag is a single manifest, so the one
  # RepoDigests entry IS the registry digest of the tag.
  "$tool" inspect --format \
    'digest   {{ index .RepoDigests 0 }}
version  {{ index .Labels "org.opencontainers.image.version" }}
revision {{ index .Labels "org.opencontainers.image.revision" }}
ref      {{ index .Labels "org.opencontainers.image.ref.name" }}
created  {{ index .Labels "org.opencontainers.image.created" }}' "$image"
  # The binary's own stamp: --version prints the startup banner and exits before any kubeconfig is read.
  printf 'binary   '
  "$tool" run --rm --entrypoint /manager "$image" --version 2>&1 | grep -m1 -o 'VERSION:[^║]*' | sed 's/ *$//' || echo "(no banner: image predates --version)"
}

case ${1:-} in
  run) shift; cmd_run "$@" ;;
  status) cmd_status ;;
  pull) shift; cmd_pull "$@" ;;
  *) print_header "$0"; exit 2 ;;
esac
