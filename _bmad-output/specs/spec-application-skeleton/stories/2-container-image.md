---
title: 'Container image'
type: 'feature'
created: '2026-10-20'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-application-skeleton/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-application-skeleton/skeleton-contract.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The server of story 1 has no image, so nothing proves it runs under the chart's restricted envelope, and #18 needs a Dockerfile it can build for amd64 and arm64.

**Approach:** A two-stage Dockerfile cross-compiling the static binary onto distroless, and `make build-image`, which checks the Dockerfile, builds the image for the host, and probes `/healthz` on a container run read-only with no capability as uid 65532 (CAP-3).

## Boundaries & Constraints

**Always:**

- Exactly these two `FROM` lines, in order:
  - `FROM --platform=$BUILDPLATFORM golang:1.27.2@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8 AS build`
  - `FROM gcr.io/distroless/static-debian13:nonroot@sha256:516757106de35205efb70b91ba03f6f1298424a0678c603255cf2852a7000087`
- Build stage: `ARG TARGETOS` and `ARG TARGETARCH`; `ENV GOTOOLCHAIN=local`; a `RUN` line containing `CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build` of `./cmd/scaleway-manager` only.
- Runtime stage: the binary at `/scaleway-manager`, `USER 65532`, `ENTRYPOINT ["/scaleway-manager"]`.
- `make build-image` follows story 1's Makefile pattern (`##` comment, `.PHONY`, tool guards `check-docker` and `check-curl` printing exactly `Error: docker not found. Install with: https://docs.docker.com/engine/install/` or `Error: curl not found. Install with: https://curl.se/download.html` and exiting 1) and stays out of `make check`; CI job `build-image` follows story 1's jobs (checkout pinned, `persist-credentials: false`), with no extra action: plain `docker build` under BuildKit.
- The container is always removed, on success or failure.

**Never:** multi-arch build, push, signing or attestation (#18); `cmd/check-go-mod` in the image; a shell, `curl` or `HEALTHCHECK` inside the image; `--privileged` on the probe run.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Happy | repository Dockerfile | checker passes, image built, `/healthz` 200 `ok` within 30 s | exit 0 |
| Missing digest | a `FROM` without `@sha256:` + 64 lowercase hex | checker fails naming the line | exit 1, no build |
| Wrong order or count | runtime `FROM` first, or a third `FROM` | checker fails | exit 1, no build |
| No BUILDPLATFORM | build `FROM` without `--platform=$BUILDPLATFORM` | checker fails | exit 1, no build |
| No cross-compile | no `RUN` with `GOOS=$TARGETOS GOARCH=$TARGETARCH go build` in the build stage | checker fails | exit 1, no build |
| Not healthy | no 200 with body exactly `ok` (no newline) within 30 s | container stopped and removed | exit 1, prints `healthz: status=<code> body=<body>` of the last attempt |

</frozen-after-approval>

## Code Map

- `cmd/scaleway-manager` -- built by the Dockerfile; story 1's contract: reads only `LISTEN_ADDR`, `GET /healthz` 200 `ok`.
- `Makefile` -- story 1 pattern (`check-go`, `lint-go`); add `check-docker`, `check-curl`, `build-image`.
- `.github/workflows/pr-checks.yml` -- story 1's Go jobs as model; Docker and curl are on `ubuntu-latest`.
- `.gitignore`, `biome.json` -- unchanged.

## Tasks & Acceptance

**Execution:**

- [ ] `Dockerfile` -- per Always; `WORKDIR /src`, `COPY . .`, output `/out/scaleway-manager`, `-trimpath`.
- [ ] `.dockerignore` -- exclude `.git`, `node_modules`, `_bmad`, `_bmad-output`, `.claude`, `coverage`, `docs`, `tests`, `charts`, `scripts`, `skills`, `*.md`; keep what `go build ./cmd/scaleway-manager` needs (`go.mod`, `embed.go`, `web/`, `internal/`, `cmd/`).
- [ ] `scripts/check-dockerfile.sh` -- `check-dockerfile.sh <file>`, exit 0 or 1: the checks of the matrix rows Missing digest to No cross-compile, each failure printing the rule and the line; bash with `set -euo pipefail`. Every line matching `^[[:space:]]*[Ff][Rr][Oo][Mm][[:space:]]` counts as a `FROM` line; there must be exactly two, matched case-sensitively against the two exact lines of Always, with any 64-lowercase-hex digest; the `RUN` substring is matched on a single physical line.
- [ ] `tests/image/check-dockerfile.test.sh` and `tests/image/fixtures/*.Dockerfile` -- runs the checker on one passing fixture and one failing fixture per checker row, asserting each exit code; exits 1 on any mismatch.
- [ ] `scripts/build-image.sh` -- runs the fixture test, then the checker on `Dockerfile`, then `DOCKER_BUILDKIT=1 docker build -t scaleway-manager:local .`, then `docker run -d --rm --read-only --cap-drop ALL --user 65532 -p 127.0.0.1::8080`, reads the port with `docker port`, polls `/healthz` with `curl` once per second for up to 30 s, compares status and exact body, removes the container through a `trap`.
- [ ] `Makefile` -- `build-image: check-docker check-curl ## …` runs `scripts/build-image.sh`.
- [ ] `.github/workflows/pr-checks.yml` -- job `build-image`: checkout, `make build-image`.

**Acceptance Criteria:**

- Given this story merged, when `make build-image` runs locally and in CI, then it exits 0 and `docker image inspect scaleway-manager:local` shows `User` `65532`.
- Given the repository `Dockerfile` with its digest removed, when `make build-image` runs, then it exits non-zero before any build (real run, then reverted).
- Given `docker` absent from `PATH`, when `make build-image` runs, then it prints the install hint and exits 1.
- Given `make help`, then `build-image` is listed; `make check` does not run it.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Verification

**Commands:**

- `tests/image/check-dockerfile.test.sh` -- expected: exit 0.
- `make build-image` -- expected: exit 0, `/healthz` 200 `ok` reported.
- `docker buildx build --platform linux/arm64 -o type=cacheonly .` -- expected: exit 0 (cross-compilation works; not part of the target).
