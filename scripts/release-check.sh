#!/usr/bin/env bash
# Checks run by the "Deliver" workflow before anything is created.
# Usage: scripts/release-check.sh <version> [open-issues]
#   version: the tag to create, semver with a leading v (v1.2.3, v1.2.3-rc.1).
#   open-issues: open issues of the milestone titled <version>, when one exists
#     (the workflow reads it from the GitHub API); empty when there is none.
# Fails when the version is not valid semver, when the tag already exists
# (local tags: the checkout fetches them all), or when the milestone has open issues.
set -euo pipefail

version=${1:-}
open=${2:-}
num='(0|[1-9][0-9]*)'
ident='[0-9A-Za-z-]+'
pre='(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)'
# Prerelease identifiers: no leading zero in a numeric one (semver.org); build ones: any.
semver="^v$num\.$num\.$num(-$pre(\.$pre)*)?(\+$ident(\.$ident)*)?$"

[[ $version =~ $semver ]] || { echo "Error: '$version' is not a semver version such as v1.2.3"; exit 1; }
cd "$(git rev-parse --show-toplevel)"
if git rev-parse --verify --quiet "refs/tags/$version" >/dev/null; then
  echo "Error: tag $version already exists"
  exit 1
fi
if [ -n "$open" ] && [ "$open" != 0 ]; then
  echo "Error: milestone $version still has $open open issue(s)"
  exit 1
fi
echo "OK: $version can be delivered"
