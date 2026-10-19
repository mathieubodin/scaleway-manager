#!/usr/bin/env bash
# Print the issue of a PR: the number of the first "Closes #<n>" of its body, HTML
# comments left out. For long work the PR body lists the epic first.
# Fails when there is none.
# Usage: scripts/pr-issue.sh < body.md (CI: gh pr view <pr> --json body -q .body | ...)
set -euo pipefail

n=$(awk '
  { line = $0; out = "" }
  {
    while (line != "") {
      if (incomment) {
        i = index(line, "-->"); if (!i) { line = ""; break }
        line = substr(line, i + 3); incomment = 0
      } else {
        i = index(line, "<!--"); if (!i) { out = out line; line = ""; break }
        out = out substr(line, 1, i - 1); line = substr(line, i + 4); incomment = 1
      }
    }
    print out
  }' | grep -oE '(^|[^[:alnum:]_])Closes #[1-9][0-9]*([^[:alnum:]_]|$)' | head -n 1 | grep -oE '[0-9]+' || true)
[ -n "$n" ] || { echo "Error: the PR body has no 'Closes #<n>'" >&2; exit 1; }
echo "$n"
