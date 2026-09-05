#!/usr/bin/env bash
# Local CI for this fork: the checks a branch must pass before its image is built, runnable on a
# laptop with no GitHub secrets. It deliberately does NOT replace .github/workflows/{pr,push}.yaml —
# those call the shared redhat-cop operator workflow and stay as they are; this is the same gate,
# brought forward to before the push.
#
# Usage:
#   hack/local-ci.sh                       # gofmt, go vet, go build, go test -race
#   LOCAL_CI_IMAGE=1 hack/local-ci.sh      # ...plus a container build, to prove the Dockerfile
#   LOCAL_CI_SKIP_GENERATORS=1 hack/local-ci.sh   # skip `make manifests generate` (first run downloads controller-gen)
#
# WHAT IS NOT HERE, AND WHY. `make test` adds only envtest assets over `go test ./...`, and the Ginkgo
# suite in controllers/ has no specs, so Ginkgo skips its BeforeSuite and envtest never starts; the
# generator check that `make test` also implies runs here on its own. controller-gen is pinned to
# v0.19.0, the version that produced the committed CRDs, so it runs on a current toolchain.
set -euo pipefail
cd "$(dirname "$0")/.."

step() { printf '\n==> %s\n' "$*"; }
fail() { printf '\nFAILED: %s\n' "$*" >&2; exit 1; }

step "gofmt"
# NUL-separated so a path with a space is checked rather than silently skipped.
unformatted=$(git ls-files -z '*.go' | xargs -0 gofmt -l)
[ -z "$unformatted" ] || fail "files need gofmt:"$'\n'"$unformatted"

step "hack scripts"
for f in hack/*.sh; do bash -n "$f" || fail "$f does not parse"; done
bash hack/lib_test.sh

step "go vet"
go vet ./...

step "go build"
go build ./...

step "unit tests with the race detector"
# The macOS linker prints an LC_DYSYMTAB warning for every -race test binary; it is noise, not a
# result. The pipeline sits inside `if` so `set -e` cannot abort before the message; with pipefail
# its status is go test's, not grep's.
if ! { go test -race -count=1 ./... 2>&1 | grep -v 'malformed LC_DYSYMTAB'; }; then
  fail "unit tests"
fi

step "the binary answers --version"
go build -o bin/manager .
bin/manager --version 2>&1 | grep -q 'VERSION:' || fail "bin/manager --version did not print the banner"
# Building the package (not main.go) is what lets Go record vcs.* settings; internal/version reads
# them when no ldflags were given. A file argument would silently lose them again.
go version -m bin/manager | grep -q 'vcs.revision' || fail "bin/manager carries no vcs.revision: was it built from main.go instead of the package?"

step "rendered manifests carry the log flags"
# Container.Args has no strategic-merge key, so a patch that touches args replaces the whole list;
# this is how the zap flags once vanished from the rendered Deployment. kubectl ships kustomize.
if command -v kubectl >/dev/null; then
  kustomize_err=$(mktemp)
  rendered=$(kubectl kustomize config/default 2>"$kustomize_err") \
    || fail "kubectl kustomize config/default failed:"$'\n'"$(cat "$kustomize_err")"
  rm -f "$kustomize_err"
  echo "$rendered" | grep -q -- '--zap-log-level=info' || fail "config/default lost --zap-log-level"
  echo "$rendered" | grep -q -- '--zap-devel=false' || fail "config/default lost --zap-devel"
  echo "config/default: manager args intact"
  # The image-override overlay must retag the MANAGER container, not the proxy at containers[0].
  # Parsed rather than grepped: kustomize prints keys alphabetically, so `image:` precedes `name:`.
  kubectl kustomize config/overlays/image-override 2>/dev/null | python3 -c '
import sys, yaml
for d in yaml.safe_load_all(sys.stdin):
    if d and d.get("kind") == "Deployment":
        by = {c["name"]: c for c in d["spec"]["template"]["spec"]["containers"]}
        m, p = by["manager"], by["kube-rbac-proxy"]
        assert m["image"].startswith("quay.io/ephico2real/namespace-configuration-operator:"), m["image"]
        assert m.get("imagePullPolicy") == "Always", m.get("imagePullPolicy")
        assert p.get("imagePullPolicy") != "Always", "overlay touched the proxy"
        print("config/overlays/image-override: manager retagged, proxy untouched")
' || fail "config/overlays/image-override does not retag the manager container (see assertion above)"
else
  echo "kubectl not found; skipping the kustomize render checks"
fi

if [ -z "${LOCAL_CI_SKIP_GENERATORS:-}" ]; then
  step "generated files are current (make manifests generate)"
  # controller-gen is pinned to the version that produced the committed files, so any diff here is a
  # real drift: a marker edited without regenerating, or a generator bump without committing.
  make -s manifests generate
  if ! git diff --quiet -- config/ api/; then
    git --no-pager diff --stat -- config/ api/
    fail "the generators changed committed files; commit them"
  fi
  echo "generated files are current"
fi

if [ -n "${LOCAL_CI_IMAGE:-}" ]; then
  step "container build"
  tool=${CONTAINER_TOOL:-$(command -v podman >/dev/null && echo podman || echo docker)}
  "$tool" build --platform "${PLATFORM:-linux/amd64}" -t namespace-configuration-operator:local-ci . >/dev/null
  echo "built namespace-configuration-operator:local-ci with $tool"
fi

printf '\nlocal CI passed\n'
