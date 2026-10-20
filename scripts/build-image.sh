#!/usr/bin/env bash
# Checks the Dockerfile, builds scaleway-manager:local for the host, then probes
# GET /healthz on a container run read-only, with no capability, as uid 65532
# (CAP-3). The container is removed on success or failure.
# Usage: scripts/build-image.sh (exit 0 or 1); run from `make build-image`.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"
image=scaleway-manager:local

tests/image/check-dockerfile.test.sh
scripts/check-dockerfile.sh Dockerfile

DOCKER_BUILDKIT=1 docker build -t "$image" .

container=
cleanup() {
	if [[ -n $container ]]; then
		docker rm -f "$container" >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT

# No --rm: logs stay readable if the binary exits at start; cleanup removes it.
container=$(docker run -d --read-only --cap-drop ALL --user 65532 -p 127.0.0.1::8080 "$image")
hostport=$(docker port "$container" 8080/tcp 2>/dev/null | head -n 1 || true)
if [[ -z $hostport ]]; then
	docker logs "$container" 2>&1 || true
	echo "healthz: status=000 body="
	exit 1
fi
url="http://$hostport/healthz"
echo "Probing $url"

body_file=$(mktemp)
trap 'cleanup; rm -f "$body_file"' EXIT

code=000
body=
deadline=$((SECONDS + 30))
while ((SECONDS < deadline)); do
	code=$(curl -s -o "$body_file" -w '%{http_code}' --max-time 1 "$url" || true)
	body=$(cat "$body_file")
	# Exact body "ok", no newline: compare the file size too.
	if [[ $code == 200 && $body == ok && $(wc -c <"$body_file") -eq 2 ]]; then
		echo "healthz: status=200 body=ok"
		exit 0
	fi
	sleep 1
done

echo "healthz: status=$code body=$body"
exit 1
