#!/usr/bin/env bash
# Tests for hack/lib.sh. Run directly or through hack/local-ci.sh.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=lib.sh
. "$here/lib.sh"

fail() { printf 'lib_test: FAILED: %s\n' "$*" >&2; exit 1; }

tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
cd "$tmp"
git init -q && git config user.email t@example.com && git config user.name t
mkdir -p controllers api internal
printf 'package main\n' > main.go; touch go.mod go.sum controllers/a.go api/a.go internal/a.go
git add -A && git commit -q -m init

tree_is_clean || fail "a committed tree must be clean"

touch controllers/zz_probe_test.go   # untracked, inside a shipped path
tree_is_clean && fail "an untracked file under controllers/ must make the tree dirty"
dirty_summary | grep -q 'controllers/zz_probe_test.go' || fail "dirty_summary must name the file"
rm controllers/zz_probe_test.go

touch NOTES.md                        # untracked, outside the shipped paths
tree_is_clean || fail "an untracked file outside the shipped paths must not matter"

echo 'x' >> main.go                   # modified tracked file
tree_is_clean && fail "a modified shipped file must make the tree dirty"
git checkout -q main.go

cat > s.sh <<'H'
#!/usr/bin/env bash
# Usage line one.
#   indented detail
#
# Last comment line.
set -euo pipefail
echo hi
H
out=$(print_header s.sh)
[ "$(printf '%s' "$out" | head -1)" = "Usage line one." ] || fail "print_header must start after the shebang"
printf '%s' "$out" | grep -q 'set -euo' && fail "print_header must stop before code"
[ "$(printf '%s' "$out" | tail -1)" = "Last comment line." ] || fail "print_header must keep the last comment line"

# Every remote form measured in review: scp-like, https, https with credentials, ssh with and without
# a port, git://, with and without .git and a trailing slash, and an insteadOf alias (git remote
# get-url expands it). A non-GitHub form (file://) must fail rather than guess.
for url in git@github.com:ephico2real2/namespace-configuration-operator.git https://github.com/ephico2real2/namespace-configuration-operator.git https://github.com/ephico2real2/namespace-configuration-operator https://user:tok@github.com/ephico2real2/namespace-configuration-operator.git ssh://git@github.com/ephico2real2/namespace-configuration-operator.git ssh://git@github.com:22/ephico2real2/namespace-configuration-operator.git git://github.com/ephico2real2/namespace-configuration-operator.git https://github.com/ephico2real2/namespace-configuration-operator.git/ git@github.com:ephico2real2/namespace-configuration-operator.git/; do
  git remote remove origin 2>/dev/null || true
  git remote add origin "$url"
  [ "$(origin_repo)" = "ephico2real2/namespace-configuration-operator" ] || fail "origin_repo failed for $url: $(origin_repo)"
done
git config url.git@github.com:.insteadOf gh:
git remote remove origin; git remote add origin gh:ephico2real2/namespace-configuration-operator
[ "$(origin_repo)" = "ephico2real2/namespace-configuration-operator" ] || fail "origin_repo failed for an insteadOf alias: $(origin_repo)"
git config --unset url.git@github.com:.insteadOf
git remote remove origin; git remote add origin file:///tmp/not-github.git
origin_repo >/dev/null 2>&1 && fail "origin_repo must fail for a non-GitHub remote"

echo "lib_test: ok"
