#!/usr/bin/env bash
# Local CI for this fork: the checks a branch must pass before its image is built, runnable on a
# laptop with no GitHub secrets. It deliberately does NOT replace .github/workflows/{pr,push}.yaml —
# those call the shared redhat-cop operator workflow and stay as they are; this is the same gate,
# brought forward to before the push.
#
# Usage:
#   hack/local-ci.sh                       # gofmt, go vet, go build, go test -race
#   LOCAL_CI_IMAGE=1 hack/local-ci.sh      # ...plus a container build, to prove the Dockerfile
#   LOCAL_CI_GENERATORS=1 hack/local-ci.sh # ...plus `make manifests generate` — needs Go 1.21, see below
#
# WHAT IS NOT HERE, AND WHY.
#   - `make test`. Its extra work over `go test ./...` is (a) the code generators and (b) envtest
#     assets for the Ginkgo suite. The Ginkgo suite in controllers/ has no specs, so Ginkgo skips its
#     BeforeSuite and envtest never starts — (b) is a no-op today. (a) is real but the pinned
#     controller-gen v0.11.1 panics in go/types under Go 1.22 and later
#     (kubernetes-sigs/controller-tools#880, fixed in v0.14.0), so on a current toolchain it cannot
#     run; the shared workflow pins Go ~1.21 and keeps that check. Opt in with LOCAL_CI_GENERATORS=1
#     when running Go 1.21 locally.
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
# The macOS linker prints an LC_DYSYMTAB warning for every -race test binary; it is noise, not a result.
go test -race -count=1 ./... 2>&1 | grep -v 'malformed LC_DYSYMTAB'
[ "${PIPESTATUS[0]}" -eq 0 ] || fail "unit tests"

if [ -n "${LOCAL_CI_GENERATORS:-}" ]; then
  step "generated files are current (make manifests generate)"
  make manifests generate
  if ! git diff --quiet -- config/ api/; then
    git --no-pager diff --stat -- config/ api/
    fail "the generators changed committed files; commit them"
  fi
fi

if [ -n "${LOCAL_CI_IMAGE:-}" ]; then
  step "container build"
  tool=${CONTAINER_TOOL:-$(command -v podman >/dev/null && echo podman || echo docker)}
  "$tool" build --platform "${PLATFORM:-linux/amd64}" -t namespace-configuration-operator:local-ci . >/dev/null
  echo "built namespace-configuration-operator:local-ci with $tool"
fi

printf '\nlocal CI passed\n'
