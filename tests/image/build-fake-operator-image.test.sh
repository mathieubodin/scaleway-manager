#!/usr/bin/env bash
# Acceptance (story 7 of #42): with Docker, `make build-fake-operator-image` exits 0
# and `docker image inspect scaleway-manager-fake-operator:local` succeeds. The image
# runs as user 65532 with ENTRYPOINT ["/fake-operator"]. No container is run.
# Needs docker; builds the real image.
# Usage: tests/image/build-fake-operator-image.test.sh (exit 0 or 1).
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
image=scaleway-manager-fake-operator:local
failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

command -v docker >/dev/null || { echo "FAIL: docker not found"; exit 1; }

# Start from no image, so the inspection below sees the image this run built.
docker image rm -f "$image" >/dev/null 2>&1 || true

out=$(mktemp)
trap 'rm -f "$out"' EXIT

if make -C "$root" --no-print-directory build-fake-operator-image >"$out" 2>&1; then
	echo "ok: make build-fake-operator-image exits 0"
else
	cat "$out"
	fail "make build-fake-operator-image exits non-zero"
fi

if docker image inspect "$image" >/dev/null 2>&1; then
	echo "ok: docker image inspect $image succeeds"
	user=$(docker image inspect --format '{{.Config.User}}' "$image")
	[[ $user == "65532" ]] && echo "ok: image User is 65532" || fail "image User is '$user', want '65532'"
	entrypoint=$(docker image inspect --format '{{json .Config.Entrypoint}}' "$image")
	[[ $entrypoint == '["/fake-operator"]' ]] && echo "ok: ENTRYPOINT is /fake-operator" \
		|| fail "ENTRYPOINT is $entrypoint, want [\"/fake-operator\"]"
else
	fail "image $image does not exist after make build-fake-operator-image"
fi

left=$(docker ps -aq --filter "ancestor=$image" 2>/dev/null || true)
[[ -z $left ]] && echo "ok: no container of $image" || fail "containers of $image exist: $left"

if ((failures > 0)); then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
