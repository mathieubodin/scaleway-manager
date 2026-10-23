#!/usr/bin/env bash
# Checks cmd/fake-operator/Dockerfile, then builds scaleway-manager-fake-operator:local
# for the host (CAP-9). Test-only image: never pushed, never run here (the binary
# needs an API server).
# Usage: scripts/build-fake-operator-image.sh (exit 0 or 1); run from
# `make build-fake-operator-image`.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

scripts/check-dockerfile.sh cmd/fake-operator/Dockerfile

DOCKER_BUILDKIT=1 docker build -f cmd/fake-operator/Dockerfile -t scaleway-manager-fake-operator:local .
