#!/usr/bin/env bash
# Fail unless the branch adds or changes docs/migrations/next/<issue>.md, in a valid format.
# Usage: scripts/check-migration.sh <issue> [base]. Base defaults to origin/main.
# Only commits made on the branch since it left the base count (git merge-base):
# a file inherited from the base, or changed on the base since, does not count,
# and uncommitted changes are not seen. The file is read as committed in HEAD.
# Format: exactly "Migration: no impact", or "## Upgrade" and "## Rollback"
# sections, each with text. See docs/development-process.md, "Migration files".
# CI needs the history down to the merge-base (actions/checkout fetch-depth: 0).
set -euo pipefail

issue=${1:-}
base=${2:-origin/main}
[[ $issue =~ ^[1-9][0-9]*$ ]] || { echo "Error: issue must be a number, got '$issue'"; exit 2; }

cd "$(git rev-parse --show-toplevel)"
file=docs/migrations/next/$issue.md
[[ $base != -* ]] && git rev-parse --verify --quiet "$base^{commit}" >/dev/null \
  || { echo "Error: '$base' is not a commit (in CI, fetch the history: fetch-depth: 0)"; exit 2; }
fork=$(git merge-base "$base" HEAD) \
  || { echo "Error: no common commit between '$base' and HEAD (shallow clone?)"; exit 2; }

if [ -z "$(git diff --name-only --diff-filter=ACMR "$fork" HEAD -- "$file")" ]; then
  echo "Error: $file is not added or changed by the commits of this branch (since ${fork:0:7}, merge-base with $base)."
  exit 1
fi

content=$(git show "HEAD:$file")
[ "$content" = "Migration: no impact" ] && { echo "OK: $file (no impact)"; exit 0; }

# Each required "## " heading must exist once, outside code fences, and have text
# before the next "# " or "## " heading ("###" subheadings and code stay in the section).
# Fence lines, blank lines and one-line HTML comments are not text.
missing=$(awk '
  { sub(/[ \t\r]+$/, "") }
  /^(```|~~~)/ { fence = !fence; next }
  fence { if (NF && cur != "") text[cur] = 1; next }
  /^## / { cur = $0; seen[cur]++; next }
  /^# /  { cur = ""; next }
  /^<!--.*-->$/ { next }
  NF && cur != "" { text[cur] = 1 }
  END {
    split("## Upgrade|## Rollback", req, "|")
    for (i = 1; i <= 2; i++) {
      h = req[i]
      if (seen[h] != 1) print h " (" (seen[h] ? "repeated" : "missing") ")"
      else if (!text[h]) print h " (empty)"
    }
  }' <<<"$content")
if [ -n "$missing" ]; then
  echo "Error: $file is neither exactly 'Migration: no impact' nor complete:"
  while IFS= read -r line; do echo "  - $line"; done <<<"$missing"
  exit 1
fi
echo "OK: $file (upgrade and rollback)"
