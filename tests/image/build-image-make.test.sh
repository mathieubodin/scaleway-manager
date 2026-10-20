#!/usr/bin/env bash
# Acceptance (story 2 of #41): `make help` lists build-image; `make check` does not
# run it. Also the Makefile pattern of story 1: build-image, check-docker and
# check-curl are .PHONY, and build-image depends on check-docker and check-curl.
# Reads make's database (make -pRrq) and a dry run (make -n); runs no check.
# Usage: tests/image/build-image-make.test.sh (exit 0 or 1).
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

help=$(make -C "$root" --no-print-directory help)
if grep -qE '^[[:space:]]+build-image[[:space:]]+[^[:space:]]' <<<"$help"; then
	echo "ok: make help lists build-image with a description"
else
	fail "make help does not list build-image; output: $help"
fi

db=$(make -C "$root" --no-print-directory -pRrq : 2>/dev/null || true)

# $1: target; prints its prerequisites as listed in make's database.
prereqs() { awk -v t="$1:" '$1 == t { $1 = ""; print; exit }' <<<"$db"; }

# Every target reachable from check, prerequisites followed transitively.
declare -A seen=()
queue=(check)
while ((${#queue[@]} > 0)); do
	t=${queue[0]}
	queue=("${queue[@]:1}")
	[[ -n ${seen[$t]:-} ]] && continue
	seen[$t]=1
	for p in $(prereqs "$t"); do queue+=("$p"); done
done
if [[ -z ${seen[check]:-} || -z $(prereqs check) ]]; then
	fail "target check not found in make's database"
elif [[ -n ${seen[build-image]:-} ]]; then
	fail "make check reaches build-image: ${!seen[*]}"
else
	echo "ok: build-image is not a prerequisite of check"
fi

dry=$(make -C "$root" --no-print-directory -n check 2>&1 || true)
if grep -qE 'build-image|docker' <<<"$dry"; then
	fail "make -n check runs build-image or docker: $(grep -E 'build-image|docker' <<<"$dry")"
else
	echo "ok: make -n check runs neither build-image nor docker"
fi

build_prereqs=" $(prereqs build-image) "
for g in check-docker check-curl; do
	if [[ $build_prereqs == *" $g "* ]]; then
		echo "ok: build-image depends on $g"
	else
		fail "build-image does not depend on $g (prerequisites:$build_prereqs)"
	fi
done

phony=" $(prereqs .PHONY) "
for t in build-image check-docker check-curl; do
	if [[ $phony == *" $t "* ]]; then
		echo "ok: $t is .PHONY"
	else
		fail "$t is not .PHONY"
	fi
done

if ((failures > 0)); then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
