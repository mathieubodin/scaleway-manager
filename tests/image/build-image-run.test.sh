#!/usr/bin/env bash
# Acceptance (story 2 of #41): `make build-image` exits 0, the image
# scaleway-manager:local runs as user 65532, and no container of it is left behind.
# Needs docker and curl; builds the real image.
# Usage: tests/image/build-image-run.test.sh (exit 0 or 1).
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
image=scaleway-manager:local
failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

command -v docker >/dev/null || { echo "FAIL: docker not found"; exit 1; }

# Start from no image, so the inspection below sees the image this run built.
docker image rm -f "$image" >/dev/null 2>&1 || true

out=$(mktemp)
trap 'rm -f "$out"' EXIT

if make -C "$root" --no-print-directory build-image >"$out" 2>&1; then
	echo "ok: make build-image exits 0"
else
	cat "$out"
	fail "make build-image exits non-zero"
fi

if user=$(docker image inspect --format '{{.Config.User}}' "$image" 2>/dev/null); then
	if [[ $user == "65532" ]]; then
		echo "ok: image User is 65532"
	else
		fail "image User is '$user', want '65532'"
	fi
else
	fail "image $image does not exist after make build-image"
fi

if docker image inspect "$image" >/dev/null 2>&1; then
	left=$(docker ps -aq --filter "ancestor=$image")
	if [[ -z $left ]]; then
		echo "ok: no container of $image left"
	else
		fail "containers of $image left after make build-image: $left"
	fi

	# Never: a shell, curl or HEALTHCHECK inside the image.
	healthcheck=$(docker image inspect --format '{{json .Config.Healthcheck}}' "$image")
	[[ $healthcheck == "null" ]] && echo "ok: no HEALTHCHECK" || fail "image has a HEALTHCHECK: $healthcheck"

	entrypoint=$(docker image inspect --format '{{json .Config.Entrypoint}}' "$image")
	[[ $entrypoint == '["/scaleway-manager"]' ]] && echo "ok: ENTRYPOINT is /scaleway-manager" \
		|| fail "ENTRYPOINT is $entrypoint, want [\"/scaleway-manager\"]"

	for absent in /bin/sh /bin/bash /usr/bin/curl /check-go-mod; do
		if docker run --rm --entrypoint "$absent" "$image" >/dev/null 2>&1; then
			fail "$absent runs inside the image"
		else
			echo "ok: $absent does not run inside the image"
		fi
	done
fi

if ((failures > 0)); then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
