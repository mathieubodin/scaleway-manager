#!/usr/bin/env bash
# Acceptance (story 2 of #41): with `docker` absent from PATH, `make build-image`
# prints the docker install hint and its guard exits 1. Same for `curl` (Always:
# tool guards check-docker and check-curl).
# PATH is replaced by a directory of links to every command of the current PATH
# but the removed one. make itself exits 2 when a recipe fails, so the guard's
# exit 1 is read from make's "Error 1" line.
# Usage: tests/image/build-image-missing-tool.test.sh (exit 0 or 1).
set -euo pipefail
# make prints "Error 1" in the C locale only.
export LC_ALL=C

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# $1: command left out; prints the directory to use as PATH.
path_without() {
	local missing=$1 dir=$work/path-without-$1 d f name
	mkdir -p "$dir"
	IFS=: read -r -a dirs <<<"$PATH"
	for d in "${dirs[@]}"; do
		[[ -d $d ]] || continue
		for f in "$d"/*; do
			name=${f##*/}
			[[ $name == "$missing" ]] && continue
			[[ -x $f && ! -d $f && ! -e $dir/$name ]] && ln -s "$f" "$dir/$name"
		done
	done
	printf '%s' "$dir"
}

# $1: command left out; $2: guard target; $3: exact hint.
run_case() {
	local missing=$1 guard=$2 hint=$3 out=$work/$1.out status=0 path
	path=$(path_without "$missing")
	if PATH=$path command -v "$missing" >/dev/null 2>&1; then
		fail "$missing: still on the reduced PATH"
		return
	fi
	PATH=$path make -C "$root" --no-print-directory build-image >"$out" 2>&1 || status=$?
	if ((status == 0)); then
		fail "$missing absent: make build-image exits 0"
	else
		echo "ok: $missing absent: make build-image exits $status"
	fi
	if grep -qxF "$hint" "$out"; then
		echo "ok: $missing absent: hint printed"
	else
		fail "$missing absent: hint '$hint' not printed; output: $(cat "$out")"
	fi
	if grep -qE "\[(Makefile:[0-9]+: )?$guard\] Error 1\$" "$out"; then
		echo "ok: $missing absent: $guard exits 1"
	else
		fail "$missing absent: no '$guard ... Error 1' line; output: $(cat "$out")"
	fi
}

run_case docker check-docker 'Error: docker not found. Install with: https://docs.docker.com/engine/install/'

# curl alone missing: docker must stay reachable so that check-docker passes.
if command -v docker >/dev/null 2>&1; then
	run_case curl check-curl 'Error: curl not found. Install with: https://curl.se/download.html'
else
	fail "docker not found: the curl case needs docker on PATH"
fi

if ((failures > 0)); then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
