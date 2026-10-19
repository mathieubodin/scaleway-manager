#!/usr/bin/env bash
# Print the release note of the next version, from the migration files of
# docs/migrations/next/ added, changed or renamed since the previous version (all of
# them when no tag exists).
# Usage: scripts/release-note.sh [version]. The previous version is the latest v* tag
# reachable from HEAD; for a final version (no "-", or no argument) prereleases are
# skipped, so that v0.3.0 lists everything since v0.2.0, v0.3.0-rc.1 included.
# Only no-impact entries (or none): a single "No impact." line. Otherwise the
# no-impact entries are dropped and each other entry is printed under its issue,
# its headings moved two levels down (outside code fences) to fit under it.
# Fails on a file that is not named <issue>.md: the folder holds nothing else.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
version=${1:-}
exclude=(--exclude 'v*-*')
[[ $version == *-* ]] && exclude=()
prev=$(git describe --tags --abbrev=0 --match 'v*' "${exclude[@]}" 2>/dev/null || true)
dir=docs/migrations/next
if [ -n "$prev" ]; then
  mapfile -t files < <(git diff --name-only --diff-filter=AMR "$prev" HEAD -- "$dir" | sort -V)
else
  mapfile -t files < <(git ls-tree -r --name-only HEAD -- "$dir" | sort -V)
fi

impact=()
for f in "${files[@]}"; do
  [[ $(basename "$f") =~ ^[1-9][0-9]*\.md$ ]] || { echo "Error: $f is not named <issue>.md" >&2; exit 1; }
  [ "$(git show "HEAD:$f")" = "Migration: no impact" ] || impact+=("$f")
done

echo "## Migrations"
echo
if [ ${#impact[@]} -eq 0 ]; then
  echo "No impact."
  exit 0
fi
for f in "${impact[@]}"; do
  n=$(basename "$f" .md)
  echo "### #$n"
  echo
  git show "HEAD:$f" | awk '
    /^(```|~~~)/ { fence = !fence; print; next }
    !fence && /^# Migration for #[0-9]+[ \t]*$/ { next }
    !fence && /^#+ / { sub(/^#+/, "&##") }
    { print }
  ' | sed '/./,$!d'
  echo
done
