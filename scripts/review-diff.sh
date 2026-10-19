#!/usr/bin/env bash
# Write the diff under review: changes of the working tree since <base>, untracked
# files included (as new files), ignored files excluded. Used by the workflow
# sdlc-review at the start of each round, after the fixer of the previous round.
# The output file must be outside the repository, or it and the diffs of earlier
# rounds would appear in the diff. Its folder is created when missing.
# Usage: scripts/review-diff.sh <base> <out-file>
set -euo pipefail

base=${1:-}
out=${2:-}
[ -n "$out" ] || { echo "Usage: scripts/review-diff.sh <base> <out-file>"; exit 2; }
cd "$(git rev-parse --show-toplevel)"
[[ $base != -* ]] && git rev-parse --verify --quiet "$base^{commit}" >/dev/null \
  || { echo "Error: '$base' is not a commit"; exit 2; }
case $(realpath -m -- "$out") in
  "$(pwd -P)"/*) echo "Error: '$out' is inside the repository; write it outside"; exit 2 ;;
esac
mkdir -p -- "$(dirname -- "$out")"

{
  git diff --no-color --no-ext-diff "$base" --
  # git diff --no-index exits 1 when the files differ, which they always do here.
  git ls-files -z --others --exclude-standard \
    | while IFS= read -r -d '' f; do
        git diff --no-color --no-ext-diff --no-index -- /dev/null "$f" || [ $? -eq 1 ]
      done
} >"$out"
echo "$(grep -c '^diff --git' "$out" || true) files changed since ${base}: $out"
