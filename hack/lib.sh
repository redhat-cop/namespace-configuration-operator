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

# origin_repo prints owner/name for the `origin` remote, ssh or https form, with or without .git.
# Used instead of `gh repo view`, which on a fork resolves to the PARENT repository unless a default
# has been set with `gh repo set-default`; the workflows live in the fork.
origin_repo() {
  local url
  url=$(git remote get-url origin 2>/dev/null) || return 1
  url=${url%/}
  url=${url%.git}
  url=${url%/}
  case $url in
    git@*:*) printf '%s\n' "${url#*:}" ;;
    https://*|http://*|ssh://*|git://*) url=${url#*://}; url=${url#*/}; printf '%s\n' "$url" ;;
    *) return 1 ;;
  esac
}
