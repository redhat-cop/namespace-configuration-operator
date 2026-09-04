#!/usr/bin/env bash
# Local CI for this fork: the checks a branch must pass before its image is built, runnable on a
# laptop with no GitHub secrets. It deliberately does NOT replace .github/workflows/{pr,push}.yaml —
# those call the shared redhat-cop operator workflow and stay as they are; this is the same gate,
# brought forward to before the push.
#
# Usage:
#   hack/local-ci.sh              # fmt, vet, build, unit tests (race detector), envtest suite
#   LOCAL_CI_IMAGE=1 hack/local-ci.sh   # ...plus a container build, to prove the Dockerfile
#   LOCAL_CI_SKIP_ENVTEST=1 hack/local-ci.sh   # skip the envtest suite (needs a download the first time)
set -euo pipefail
cd "$(dirname "$0")/.."

step() { printf '\n==> %s\n' "$*"; }
fail() { printf '\nFAILED: %s\n' "$*" >&2; exit 1; }

step "gofmt"
unformatted=$(gofmt -l $(git ls-files '*.go'))
[ -z "$unformatted" ] || fail "files need gofmt:"$'\n'"$unformatted"

step "go vet"
go vet ./...

step "go build"
go build ./...

step "unit tests with the race detector"
# The controllers' unit tests and the envtest suite share a package; -run keeps this step to the
# plain tests so a missing envtest binary cannot mask a unit failure. The suite runs in the next step.
go test -race -count=1 ./... 2>&1 | grep -v 'malformed LC_DYSYMTAB' || fail "unit tests"

if [ -z "${LOCAL_CI_SKIP_ENVTEST:-}" ]; then
  step "envtest suite (make test)"
  # make test also regenerates manifests and deepcopy code; a diff afterwards means a generator was
  # skipped in the commit, which the shared workflow would also catch.
  make test
  if ! git diff --quiet -- config/ api/; then
    git --no-pager diff --stat -- config/ api/
    fail "make test changed generated files; commit them"
  fi
fi

if [ -n "${LOCAL_CI_IMAGE:-}" ]; then
  step "container build"
  tool=${CONTAINER_TOOL:-$(command -v podman >/dev/null && echo podman || echo docker)}
  "$tool" build --platform "${PLATFORM:-linux/amd64}" -t namespace-configuration-operator:local-ci . >/dev/null
  echo "built namespace-configuration-operator:local-ci with $tool"
fi

printf '\nlocal CI passed\n'
