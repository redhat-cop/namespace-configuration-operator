#!/usr/bin/env bash
# Shared helpers for the hack scripts. Source it; do not run it.

# shipped_paths are what the Dockerfile COPYs into the image. A build is only as clean as these.
shipped_paths=(main.go go.mod go.sum api controllers internal)

# tree_is_clean succeeds when nothing in the shipped paths is modified OR untracked. `git diff` and
# `git describe --dirty` both ignore untracked files, and the Dockerfile copies whole directories,
# so an untracked file would ship under a tag that says the tree was clean.
tree_is_clean() {
  [ -z "$(git status --porcelain --untracked-files=all -- "${shipped_paths[@]}")" ]
}

# dirty_summary lists what tree_is_clean objected to.
dirty_summary() {
  git status --porcelain --untracked-files=all -- "${shipped_paths[@]}"
}

# print_header prints a script's leading comment block (its usage), stopping at the first line that
# is not a comment, so the shebang and `set` lines never leak into the usage text.
print_header() {
  awk 'NR == 1 { next } /^#/ { sub(/^# ?/, ""); print; next } { exit }' "$1"
}
