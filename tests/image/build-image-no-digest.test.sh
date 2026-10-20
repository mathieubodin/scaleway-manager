#!/usr/bin/env bash
# Acceptance (story 2 of #41): with a digest removed from the repository Dockerfile,
# `make build-image` exits non-zero before any build.
# Runs `make build-image` for real on a copy of the working tree (tracked and
# untracked files, ignored ones excluded), so the repository Dockerfile is never
# touched. A `docker` stub first on PATH records every call: no `docker build`,
# no `docker run` may happen.
# Usage: tests/image/build-image-no-digest.test.sh (exit 0 or 1).
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
failures=0
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

[[ -f $root/Dockerfile ]] || { echo "FAIL: $root/Dockerfile does not exist"; exit 1; }

stub=$work/stub
mkdir -p "$stub"
cat >"$stub/docker" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"$DOCKER_STUB_LOG"
case ${1:-} in
build | run | buildx | image | container) exit 1 ;;
esac
exit 0
STUB
chmod +x "$stub/docker"

# $1: case name; $2: sed expression applied to the Dockerfile copy.
run_case() {
	local name=$1 expr=$2 tree=$work/$1 log=$work/$1.log out=$work/$1.out
	mkdir -p "$tree"
	(cd "$root" && git ls-files -z --cached --others --exclude-standard \
		| xargs -0 -I{} sh -c '[ -e "$1" ] && printf "%s\0" "$1"' _ {} \
		| tar --null -T - -cf -) | tar -xf - -C "$tree"
	sed -E -i "$expr" "$tree/Dockerfile"
	if cmp -s "$root/Dockerfile" "$tree/Dockerfile"; then
		fail "$name: the sed expression did not change the Dockerfile"
		return
	fi
	: >"$log"
	local status=0
	DOCKER_STUB_LOG=$log PATH="$stub:$PATH" make -C "$tree" --no-print-directory build-image >"$out" 2>&1 || status=$?
	if ((status == 0)); then
		cat "$out"
		fail "$name: make build-image exits 0"
	else
		echo "ok: $name: make build-image exits $status"
	fi
	if grep -qE '^(build|buildx|run)( |$)' "$log"; then
		fail "$name: docker was called to build or run: $(tr '\n' ';' <"$log")"
	else
		echo "ok: $name: no docker build or run"
	fi
}

# The digest of the build stage FROM line removed.
run_case build-from-no-digest '/^FROM --platform=\$BUILDPLATFORM golang:/ s/@sha256:[0-9a-f]{64}//'
# The digest of the runtime stage FROM line removed.
run_case runtime-from-no-digest '/^FROM gcr\.io\/distroless\// s/@sha256:[0-9a-f]{64}//'

if ((failures > 0)); then
	echo "$failures failure(s)"
	exit 1
fi
echo "all passed"
