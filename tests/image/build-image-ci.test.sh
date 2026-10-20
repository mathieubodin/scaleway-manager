#!/usr/bin/env bash
# Acceptance (story 2 of #41): `make build-image` runs in CI. The pr-checks job
# build-image follows story 1's jobs: ubuntu-latest, checkout pinned by a 40-hex
# SHA with persist-credentials: false, no other action, a step running
# `make build-image`. Needs yq (mikefarah, v4).
# Usage: tests/image/build-image-ci.test.sh (exit 0 or 1).
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
wf=$root/.github/workflows/pr-checks.yml
failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

command -v yq >/dev/null || { echo "FAIL: yq not found"; exit 1; }

job=$(yq -o=json '.jobs["build-image"]' "$wf")
if [[ $job == "null" ]]; then
	echo "FAIL: no job build-image in $wf"
	exit 1
fi

runs_on=$(yq '.jobs["build-image"]."runs-on"' "$wf")
[[ $runs_on == "ubuntu-latest" ]] && echo "ok: runs-on ubuntu-latest" || fail "runs-on is '$runs_on'"

mapfile -t uses < <(yq '.jobs["build-image"].steps[] | select(has("uses")) | .uses' "$wf")
if ((${#uses[@]} == 1)) && [[ ${uses[0]} =~ ^actions/checkout@[0-9a-f]{40}$ ]]; then
	echo "ok: one action, actions/checkout pinned by SHA"
else
	fail "actions used: ${uses[*]:-none}; want only actions/checkout@<40-hex SHA>"
fi

persist=$(yq '.jobs["build-image"].steps[] | select(.uses | test("^actions/checkout@")) | .with."persist-credentials"' "$wf")
[[ $persist == "false" ]] && echo "ok: persist-credentials: false" || fail "persist-credentials is '$persist'"

if yq '.jobs["build-image"].steps[] | select(has("run")) | .run' "$wf" | grep -qE '(^|[[:space:]])make build-image([[:space:]]|$)'; then
	echo "ok: a step runs make build-image"
else
	fail "no step runs make build-image"
fi

# Never: push, signing or attestation, or a multi-arch build (#18).
if yq '.jobs["build-image"].steps[] | select(has("run")) | .run' "$wf" | grep -qE 'docker (push|login)|cosign|attest|--platform'; then
	fail "job build-image pushes, signs, attests or builds for another platform"
else
	echo "ok: no push, signing, attestation or multi-arch build"
fi

if ((failures > 0)); then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
