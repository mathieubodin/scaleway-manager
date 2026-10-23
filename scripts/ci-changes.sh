#!/usr/bin/env bash
# Print "run=false" when a PR push only changes process memlogs and cost.json of work
# folders (a step-end pushed after the PR) on top of a commit whose skippable jobs all
# passed or were skipped; "run=true" otherwise. The jobs of pr-checks.yml that need
# `changes` are skipped on run=false; a skipped required job counts as passed.
# Anything else, or any doubt (not a synchronize, no usable before, a force-push,
# an empty diff, checks of before failed, cancelled, still running or unreadable),
# gives run=true.
# Usage: ACTION=<event action> BEFORE=<sha> AFTER=<sha> REPO=<owner/name> \
#        GH_TOKEN=<token> scripts/ci-changes.sh
set -euo pipefail

# The jobs of pr-checks.yml that need `changes`; ci-changes.test.js checks the list.
SKIPPABLE="lint-docs lint-workflows test-workflows check-frozen lint-go test-go check-go-mod lint-chart build-image"

# True when every skippable job has a latest check run on $BEFORE that passed or was skipped.
before_passed() {
	local runs name
	[[ ${REPO:-} =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || return 1
	runs=$(gh api "repos/$REPO/commits/$BEFORE/check-runs?filter=latest&per_page=100" \
		--jq '.check_runs[] | "\(.name) \(.conclusion)"') || return 1
	for name in $SKIPPABLE; do
		printf '%s\n' "$runs" | grep -qxE "$name (success|skipped)" || return 1
	done
}

run=true
if [ "${ACTION:-}" = synchronize ] &&
	[[ ${BEFORE:-} =~ ^[0-9a-f]{40}$ ]] && [[ ${AFTER:-} =~ ^[0-9a-f]{40}$ ]] &&
	git merge-base --is-ancestor "$BEFORE" "$AFTER" 2>/dev/null; then
	files=$(git diff --name-only --no-renames "$BEFORE" "$AFTER")
	if [ -n "$files" ] &&
		! printf '%s\n' "$files" | grep -qvE '^_bmad-output/work/[^/]+/(\.memlog\.md|cost\.json)$' &&
		before_passed; then
		run=false
	fi
fi
echo "run=$run"
