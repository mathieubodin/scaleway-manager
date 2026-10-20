#!/usr/bin/env bash
# Story 2 of #41: scripts/check-dockerfile.sh passes on one fixture and fails on
# one fixture per broken rule (tests/image/fixtures/*.Dockerfile).
# Usage: tests/image/check-dockerfile.test.sh (exit 0 or 1).
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
checker=$root/scripts/check-dockerfile.sh
fixtures=$root/tests/image/fixtures
failures=0

# $1: fixture name (without .Dockerfile); $2: expected exit code.
expect() {
	local name=$1 want=$2 got=0 out
	out=$("$checker" "$fixtures/$name.Dockerfile" 2>&1) || got=$?
	if ((got == want)); then
		echo "ok: $name: exit $got"
	else
		echo "FAIL: $name: exit $got, want $want; output: $out"
		failures=$((failures + 1))
	fi
}

expect pass 0
expect missing-digest 1
expect uppercase-digest 1
expect wrong-order 1
expect third-from 1
expect no-buildplatform 1
expect no-cross-compile 1
expect run-split 1

if ((failures > 0)); then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
