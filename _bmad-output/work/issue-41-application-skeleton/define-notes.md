# Define notes: #41 application skeleton

## Parallax

Run `wf_2be9c3e6-f48`, 3 readers, none dead. Source: `_bmad-output/forge/application-skeleton/forged-idea.md`.

### Important

#### P1. Chart values layout for the Gateway, hostnames and listen address

- Reader 1: `httpRoute.gateway.{name,namespace,sectionName}`, `httpRoute.hostnames`, top-level `listenAddr`.
- Reader 2: top-level `gateway.{name,namespace,sectionName}`, top-level `hostnames`, top-level `listenAddr`.
- Reader 3: `httpRoute.parentRef.{name,namespace,sectionName}`, `httpRoute.hostnames`, `env.LISTEN_ADDR`.
- Proposal: `httpRoute.gateway.{name,namespace,sectionName}`, `httpRoute.hostnames` and `listenAddr`; the required-value error messages name these keys.
- Answer: as proposed: `httpRoute.gateway.{name,namespace,sectionName}`, `httpRoute.hostnames`, `listenAddr`.

#### P2. Whether `make check` runs the new Go and chart targets

- Readers 1, 2: `make check` adds the Go lint, Go test, AD-1 and chart targets, not the image build.
- Reader 3: `make check` is not extended.
- Proposal: yes, add the Go, AD-1 and chart targets to `make check`; leave the image build out.
- Answer: as proposed: `make check` adds the Go lint, Go test, AD-1 and chart targets; the image build stays out.

#### P3. prek hooks for Go and chart

- Reader 2: local prek hooks run `make lint-go` on Go files and `make lint-chart` on chart files.
- Readers 1, 3: not addressed; prek only gets the `check-yaml` exclude.
- Proposal: no; prek keeps only the `check-yaml` exclude.
- Answer: yes, against the parallax proposal, to follow the `prek.toml` convention (one hook per file type, calling the CI make target): local hooks run `make lint-go` on `.go` files and `go.mod`, and `make lint-chart` on `charts/`; no `test-go` at commit.

#### P4. HTTP server `ReadHeaderTimeout`

- Readers 2, 3: `ReadHeaderTimeout` 10 s, no `WriteTimeout`.
- Reader 1: not addressed.
- Proposal: yes, 10 s `ReadHeaderTimeout`; no `WriteTimeout`, because of the future SSE stream.
- Answer: as proposed: `ReadHeaderTimeout` 10 s (Slowloris), no `WriteTimeout`.

#### P5. What the AD-1 check covers besides `require`

- Reader 1: fails on `require` (line or block); a separate criterion requires `go.sum` to be absent.
- Reader 2: fails only when `go mod edit -json` shows a non-empty `Require`.
- Reader 3: fails on a missing `go.mod`, a non-null `Require`, or a non-empty `go.sum`.
- Proposal: parse with `go mod edit -json`, fail on any `Require` and on a non-empty `go.sum`.
- Answer: as proposed; the failure message names the offending file.

#### P6. Whether the image is checked by running the container

- Reader 2: the image started with `--read-only --cap-drop ALL` answers `/healthz` 200, by hand or in the image job.
- Reader 1: the image run with no environment serves `/healthz` on 8080 as uid 65532; where it runs is not stated.
- Reader 3: the started container answers 200 `ok` on `/healthz`.
- Proposal: the CI image job runs the container with `--read-only --cap-drop ALL --user 65532` and requests `/healthz`.
- Answer: yes, inside `make build-image` (same locally and in CI): build, run the container `--read-only --cap-drop ALL --user 65532`, wait for `/healthz` 200 `ok` from the host on the published port (distroless has no curl), stop it.

#### P7. Tool versions (found by `sdlc-work`, not by the readers)

- Local machine: Go 1.24.4 (the `go 1.27.1` line makes Go fetch 1.27.1 with the default `GOTOOLCHAIN=auto`), Helm v3.17.2; the spine pins Helm 4.3.0.
- Question: how do the make targets and CI pin Go 1.27.1 and Helm 4.3.0?
- Answer: Go through the `go 1.27.1` line (`GOTOOLCHAIN=auto` locally, `setup-go` with `go-version-file: go.mod` in CI, `golang:1.27.1` pinned by digest in the Dockerfile). Helm: one Makefile variable `HELM_VERSION=4.3.0`; `lint-chart` fails with an install hint when `helm version` differs; CI installs exactly that version.

### Details

| # | Topic | Positions | Proposal |
| --- | --- | --- | --- |
| D1 | Make target names | R1: `lint-go`, `test-go`, `check-gomod`, `lint-chart`, `image`; R2: `lint-go`, `test-go`, `check-go-deps`, `lint-chart`, `build-image`; R3: `test-go` (fmt, vet, test), `check-go-mod`, `lint-chart`, `build-image` | `lint-go`, `test-go`, `check-go-mod`, `lint-chart`, `build-image` |
| D2 | CI jobs | R1: one per target; R2: go, chart, image; R3: one per target | One job per make target (existing convention) |
| D3 | Race detector | R2: `-race`; R1, R3: without | Plain `go test ./...` |
| D4 | Negative chart check | R2, R3: in `lint-chart`; R1: criterion only | `lint-chart` checks that rendering without the Gateway fails with the message |
| D5 | Chart test values | R3: `ci/test-values.yaml`; R2: `--set`; R1: unstated | `charts/scaleway-manager/ci/test-values.yaml` |
| D6 | `go:embed` file | R1, R3: `web/embed.go`; R2: module root | Module root, so `web/` holds only what AD-8 allows |
| D7 | `listenAddr` without a port | R2: rendering fails; R1, R3: unstated | Rendering fails with a clear message |
| D8 | Shutdown over 10 s | R1: ERROR, forced close, non-zero; R2: stop line with error, exit 1; R3: ERROR, exit 1 | One ERROR line, exit 1 |
| D9 | Bind failure exit code | R1: non-zero; R2, R3: 1 | Exit 1 |
| D10 | Second signal during shutdown | R3: ignored; R1, R2: unstated | Ignored; the 10 s timeout applies |
| D11 | Placeholder sentence, viewport | R1: "scaleway-manager is running; no feature is available yet.", no viewport; R2: same, viewport; R3: "scaleway-manager has no feature yet.", viewport | R1's sentence, with a viewport meta |
| D12 | Liveness timings | R2: `initialDelaySeconds` 5; R1, R3: defaults | Kubernetes defaults |
| D13 | Fullname helper | R1: release name; R2: standard Helm rule; R3: unstated | Standard Helm fullname rule |
| D14 | `.dockerignore` | R1: `node_modules`, `_bmad*`, `.claude`, `.git`; R2: plus `coverage`, `docs`, `tests/workflows`; R3: in between | Exclude everything the build does not need |
| D15 | Extra criteria | R2, R3: in-flight request completes after SIGTERM, byte-identical body; R3: `image.tag` override; R2: `make help` lists targets, no non-std import | Keep in-flight, byte-identical, `image.tag`, `make help`; drop the import criterion (AD-1 check covers it) |
| D16 | Missing tool | R1, R3: each target fails naming the tool; R2: unstated | Fail with an install hint, like `check-node` |

Answer: all 16 proposals validated together by the human.

## Premortem

`bmad-advanced-elicitation`, Pre-mortem Analysis on `forged-idea.md` and the parallax answers: "three months from now #41 failed, write the postmortem".

### Important

#### R1. New CI jobs are not required checks

- Cause: branch protection of `main` lists its required checks explicitly (`conventional-title`, `lint-docs`, `lint-workflows`, `test-workflows`, `check-frozen`, `check-migration`); a new job that is not listed does not block a merge.
- Proposal: the spec requires adding the new jobs to the required checks, with the `gh api` command; the human runs it (admin rights).
- Answer: right after the merge (adding them earlier would block other open PRs): the spec holds the `gh api` command, `sdlc-work` reminds the human at the merge, the human runs it; a criterion then checks the list.

#### R2. No convention for chart and image tests

- Cause: AD-15 names Go tests, `node:test` and Gherkin only; the Rails test writer has nothing to follow for `helm template` and `build-image` checks.
- Proposal: decide where these tests live and in which tool.
- Answer: the `helm-unittest` plugin, pinned, tests as YAML in `charts/scaleway-manager/tests/` (excluded from the package by `.helmignore`), run by `lint-chart`. Image checks stay in `make build-image` (P6).
- Open (external fact): does `helm-unittest` support Helm 4.3.0, and at which version?

#### R3. Rolling update runs two pods

- Cause: the default `RollingUpdate` strategy starts the new pod before stopping the old one; sessions are in memory and AD-11 requires exactly one replica.
- Proposal: Deployment strategy `Recreate`.
- Answer: `Recreate`: never two pods, a short outage at each upgrade.

### Details

| # | Risk | Proposal |
| --- | --- | --- |
| R4 | A Dockerfile that cannot cross-compile forces #18 to rewrite it or use slow QEMU | `FROM --platform=$BUILDPLATFORM`, `GOOS`/`GOARCH` from `TARGETOS`/`TARGETARCH`; local build stays host-arch |
| R5 | Base images pinned by tag only | `golang` and `distroless` pinned by tag and digest |
| R6 | Port parsed from `listenAddr` wrong with IPv6 (`[::]:8080`) | Port taken after the last `:`; an IPv6 test case |
| R7 | `appVersion` `0.0.0` gives `ImagePullBackOff` until #18 publishes | Documented in `values.yaml`; #16 overrides `image.tag` |
| R8 | `GOTOOLCHAIN=auto` needs network on first run | The D16 install hint names Go 1.27.1 |
| R9 | Fixed ClusterRole names (AD-12) collide with a second release in one cluster | Accepted: one install per cluster, documented in the chart README |

Answer: R4 to R9 validated together by the human.

## Research

- Skipped by the human: whether `helm-unittest` supports Helm 4.3.0, and at which version, stays open in the spec and is settled in Plan (frameworks exploration). If it does not, R2 is reopened.

## Spec-lint

### Round 1 (`wf_332b5a19-2d8`)

All 5 criteria passed "says how to verify" (3/3) and failed "only one reading" (0/3).

- CAP-1. Answer: `GET /healthz` 200, body exactly `ok` with no trailing newline; `HEAD /healthz` 200, empty body; both AD-8 headers checked on `GET` and `HEAD /healthz`, `GET /` and `GET` of an unknown path (404).
- CAP-2. Answer: SIGTERM and SIGINT start the shutdown; a second SIGTERM or SIGINT changes neither the shutdown nor the exit code; `LISTEN_ADDR` unset or empty gives `:8080`; the 10 s run from the first signal to the end of `Shutdown`; every line (start, stop, timeout ERROR, bind ERROR) is `log/slog` JSON on stdout; a clean stop exits 0.
- CAP-3. Answer: `200 ok` means status 200 and body `ok`. The Dockerfile is checked mechanically inside `make build-image`, before the build: every `FROM` carries `tag@sha256:`, the build stage carries `--platform=$BUILDPLATFORM` and uses `TARGETOS`/`TARGETARCH`; the target fails otherwise.
- CAP-4. Answer: each contract field is asserted at its exact value; the port of `listenAddr` (text after the last `:`) goes to `containerPort`, both probes and the Service `targetPort`; rendering fails, with a message holding the full key path (`httpRoute.gateway.name`, `listenAddr`), when `listenAddr` has no `:` or the text after its last `:` is not an integer from 1 to 65535.
- CAP-5. Answer: each target checks only its own tool (`go` for `lint-go`, `test-go`, `check-go-mod`; `helm` at 4.3.0 and the helm-unittest plugin for `lint-chart`; `docker` for `build-image`). The AD-1 check is a small Go program (standard library, outside the shipped binary) run by `make check-go-mod`; Go tests on `testdata` fixtures cover a `require` line, a `require` block, a non-empty `go.sum` and a clean module, through `test-go`.

### Round 2 (`wf_534a49b9-08f`)

CAP-1 passed. CAP-2 to CAP-5 failed "only one reading" (0/3).

- CAP-2. Answer: the start line is written only once listening succeeds, so a bind failure writes only its ERROR line; a clean stop writes the stop line; a shutdown over 10 s writes the ERROR line instead of the stop line; "one ERROR line" means exactly one line at level ERROR.
- CAP-3. Answer: the Dockerfile has exactly two stages, build (`FROM` golang) then runtime (`FROM` distroless); the build stage passes `TARGETOS` and `TARGETARCH` to `go build` as `GOOS` and `GOARCH`, and the check looks for these assignments, not an `ARG` alone; the `/healthz` body is exactly `ok`.
- CAP-4. Answer: the container port is named `http` and carries the number (`containerPort: 8080`); both probes and the Service `targetPort` refer to `http`; the message holds the path without `.Values` (`httpRoute.gateway.name`); a valid port is decimal digits only, no sign, no leading zero, from 1 to 65535 (`08080`, `+80`, a leading space before `8080` are rejected).
- CAP-5. Answer: a wrong Helm version gives a hint naming `helm` 4.3.0; a missing plugin gives a hint naming helm-unittest and its pinned version; the version is compared exactly: `helm version --template '{{.Version}}'` must be `v4.3.0`; after the merge the required checks hold at least the five jobs, the six current ones stay.

### Round 3 (`wf_f6f01e4a-3c0`)

CAP-1 and CAP-5 passed. CAP-2, CAP-3, CAP-4 failed "only one reading". Third red round: the human chose to apply the proposals below and run a fourth round.

- CAP-2. Answer: the shutdown-timeout case and the bind-failure case each write exactly one line at level ERROR; every other case writes none.
- CAP-3. Answer: the two `FROM` lines are exactly `FROM --platform=$BUILDPLATFORM golang:1.27.1@sha256:<digest> AS build` and `FROM gcr.io/distroless/static-debian13:nonroot@sha256:<digest>`; the check looks for `GOOS=$TARGETOS GOARCH=$TARGETARCH go build` in a `RUN` line of the build stage; the `/healthz` body is `ok` with no trailing newline.
- CAP-4. Answer: names are asserted for the release name `scaleway-manager`, which gives `scaleway-manager`; the probes set no timing field; `resources` equals the default of `values.yaml`, whose numbers are set in Plan; an empty Gateway key covers absent, `null` and `""`.

### Round 4 (`wf_ca1e55b6-3e3`)

CAP-1 and CAP-5 passed. CAP-2, CAP-3, CAP-4 failed "only one reading" again, on finer points each round. Fourth red round: the human chose to apply the three fixes below and go to the spec gate without a fifth round; the gate page lists the rounds and the round 4 remarks.

- CAP-2. Answer: the start line is level INFO, `msg` `"listening"`, with an `addr` field, written once, after listening succeeds; the stop line is level INFO, `msg` `"stopped"`; "none" means no ERROR line; a second signal writes no line.
- CAP-3. Answer: `make build-image` fails when `/healthz` has not returned status 200 and body `ok` within 30 s of the container start; `<digest>` is 64 lowercase hexadecimal characters.
- CAP-4. Answer: "each field" means each row of the contract's "Objects" table; optional fields (`sectionName`, `hostnames`) are tested both set and unset; the `values.yaml` notes and the README statement are not asserted.

Remarks of round 4 left to the gate: "first fails unless" could be read as "the first run fails"; a voter asked whether a start line is written "only after" or "a single time" (answered above: both).
