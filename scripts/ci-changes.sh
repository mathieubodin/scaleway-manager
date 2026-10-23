#!/usr/bin/env bash
# Print "run=false" when a PR push only changes process memlogs and cost.json of work
# folders (a step-end pushed after the PR), "run=true" otherwise. The heavy jobs of
# pr-checks.yml are skipped on run=false; a skipped required job counts as passed.
# Anything else, or any doubt (not a synchronize, no usable before, a force-push,
# an empty diff), gives run=true.
# Usage: ACTION=<event action> BEFORE=<sha> AFTER=<sha> scripts/ci-changes.sh
set -euo pipefail

run=true
if [ "${ACTION:-}" = synchronize ] &&
	[[ ${BEFORE:-} =~ ^[0-9a-f]{40}$ ]] && [[ ${AFTER:-} =~ ^[0-9a-f]{40}$ ]] &&
	git merge-base --is-ancestor "$BEFORE" "$AFTER" 2>/dev/null; then
	files=$(git diff --name-only "$BEFORE" "$AFTER")
	if [ -n "$files" ] &&
		! printf '%s\n' "$files" | grep -qvE '^_bmad-output/work/[^/]+/(\.memlog\.md|cost\.json)$'; then
		run=false
	fi
fi
echo "run=$run"
