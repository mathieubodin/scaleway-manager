#!/usr/bin/env bash
# Checks the Dockerfile rules of story 2 of #41 before any build:
# - exactly two FROM lines, in order: the build stage, then the runtime stage,
#   each the exact line of the story with any digest;
# - each FROM pinned by @sha256:<64 lowercase hex>;
# - the build stage FROM runs on --platform=$BUILDPLATFORM;
# - a RUN line of the build stage cross-compiles with
#   CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build (one physical line).
# Every line matching ^[[:space:]]*FROM[[:space:]] (any case) counts as a FROM line.
# Each failure prints the rule and the line.
# Usage: scripts/check-dockerfile.sh <file> (exit 0 or 1).
set -euo pipefail

if (($# != 1)); then
	echo "Usage: $0 <Dockerfile>" >&2
	exit 1
fi
file=$1
[[ -f $file ]] || { echo "Error: $file is not a file" >&2; exit 1; }

digest_re='@sha256:[0-9a-f]{64}( |$)'
build_re='^FROM --platform=\$BUILDPLATFORM golang:1\.27\.2@sha256:[0-9a-f]{64} AS build$'
runtime_re='^FROM gcr\.io/distroless/static-debian13:nonroot@sha256:[0-9a-f]{64}$'
cross='CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build'

failures=0
# $1: line number; $2: rule; $3: line.
fail() { echo "$file:$1: $2: $3"; failures=$((failures + 1)); }

from_nums=()
from_lines=()
run_found=0
n=0
while IFS= read -r line || [[ -n $line ]]; do
	n=$((n + 1))
	if [[ $line =~ ^[[:space:]]*[Ff][Rr][Oo][Mm][[:space:]] ]]; then
		from_nums+=("$n")
		from_lines+=("$line")
		continue
	fi
	# The build stage: after the first FROM line, before the second.
	if ((${#from_nums[@]} == 1)) && [[ $line =~ ^[[:space:]]*RUN[[:space:]] && $line == *"$cross"* ]]; then
		run_found=1
	fi
done <"$file"

for i in "${!from_lines[@]}"; do
	if ! [[ ${from_lines[$i]} =~ $digest_re ]]; then
		fail "${from_nums[$i]}" "FROM not pinned by @sha256:<64 lowercase hex>" "${from_lines[$i]}"
	fi
done

if ((${#from_lines[@]} != 2)); then
	echo "$file: want exactly two FROM lines (build stage, then runtime stage), found ${#from_lines[@]}"
	for i in "${!from_lines[@]}"; do echo "$file:${from_nums[$i]}: ${from_lines[$i]}"; done
	failures=$((failures + 1))
fi

if ((${#from_lines[@]} >= 1)); then
	line=${from_lines[0]}
	if [[ $line != *'--platform=$BUILDPLATFORM'* ]]; then
		fail "${from_nums[0]}" 'build stage FROM without --platform=$BUILDPLATFORM' "$line"
	fi
	if ! [[ $line =~ $build_re ]]; then
		fail "${from_nums[0]}" 'first FROM is not "FROM --platform=$BUILDPLATFORM golang:1.27.2@sha256:<digest> AS build"' "$line"
	fi
fi
if ((${#from_lines[@]} >= 2)); then
	line=${from_lines[1]}
	if ! [[ $line =~ $runtime_re ]]; then
		fail "${from_nums[1]}" 'second FROM is not "FROM gcr.io/distroless/static-debian13:nonroot@sha256:<digest>"' "$line"
	fi
fi

if ((run_found == 0)); then
	echo "$file: no RUN line in the build stage containing: $cross"
	failures=$((failures + 1))
fi

if ((failures > 0)); then
	echo "$file: $failures Dockerfile rule(s) broken"
	exit 1
fi
echo "$file: Dockerfile rules hold"
