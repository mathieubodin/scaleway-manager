---
id: SPEC-application-skeleton
companions:
  - skeleton-contract.md
  - ../../planning-artifacts/architecture/architecture-scaleway-manager-2026-10-19/ARCHITECTURE-SPINE.md
sources:
  - ../../forge/application-skeleton/forged-idea.md
  - ../../work/issue-41-application-skeleton/define-notes.md
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Application skeleton (#41)

## Why

A mandate and a blocker: the repository has no application code, and #14 (code tests), #16 (manual cluster), #18 (publish) and #42 (fake operator) all need a binary, an image and a chart to exist. This work ships the smallest runnable scaleway-manager under the architecture spine's rules (AD-1, AD-3, AD-8, AD-11, AD-12), with the checks that keep those rules true from the first commit.

## Capabilities

- **CAP-1**
  - **intent:** The server answers health checks and serves a placeholder page, with the AD-8 security headers on every response.
  - **success:** Go tests (`make test-go`) show: `GET /healthz` returns status 200 and the body `ok` exactly, with no trailing newline; `HEAD /healthz` returns status 200 and an empty body; `GET /` returns status 200 and the bytes of `web/index.html`; `GET` of an unknown path returns 404; each of these four responses carries `Content-Security-Policy: default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'` and `X-Content-Type-Options: nosniff`.
- **CAP-2**
  - **intent:** The server starts on a configurable address and stops cleanly.
  - **success:** Go tests (`make test-go`) show: with `LISTEN_ADDR` unset or empty the server listens on `:8080`, otherwise on its value; the start line (level INFO, `msg` `"listening"`, field `addr`) is written once, after listening succeeds; on the first SIGTERM or SIGINT, a request already in flight completes, the stop line (level INFO, `msg` `"stopped"`) is written and the process exits 0; when `Shutdown` has not ended 10 s after that first signal, the process writes one ERROR line instead of the stop line and exits 1; a second SIGTERM or SIGINT during shutdown writes no line and changes neither the shutdown nor the exit code; when the address cannot be bound, the process writes one ERROR line, no start line, and exits 1; when `Serve` stops with an error before any signal, the process writes one ERROR line and exits 1; the shutdown-timeout, bind-failure and serve-failure cases each write exactly one line at level ERROR, and every other case writes no ERROR line; every line is one `log/slog` JSON object on stdout, the internal errors of `net/http` included (level WARN), and serving a request writes no line.
- **CAP-3**
  - **intent:** A container image runs the binary under the restricted envelope of the chart, and its Dockerfile is ready for the multi-arch build of #18.
  - **success:** `make build-image`, run locally and as a CI job, exits non-zero before building unless the Dockerfile has exactly two `FROM` lines, in this order, `FROM --platform=$BUILDPLATFORM golang:1.27.2@sha256:<digest> AS build` and `FROM gcr.io/distroless/static-debian13:nonroot@sha256:<digest>`, each `<digest>` being 64 lowercase hexadecimal characters, and a `RUN` line of the build stage contains `CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build`; it then builds the image for the host architecture, runs it with `--read-only --cap-drop ALL --user 65532` and a published port, requests `/healthz` from the host, stops the container, and exits non-zero unless it got status 200 and the body `ok`, with no trailing newline, within 30 s of the container start.
- **CAP-4**
  - **intent:** A Helm chart installs the server as exactly one pod reachable through a Gateway the installer names, with no Kubernetes API rights.
  - **success:** helm-unittest tests (`make lint-chart`) assert each row of the "Objects" table of `skeleton-contract.md` at its exact value, `sectionName` and `hostnames` both set and unset, with the release name `scaleway-manager` (namespaced objects are named `scaleway-manager`), no timing field in either probe, and `resources` equal to the default of `values.yaml` (requests `cpu: 10m`, `memory: 32Mi`; limits `memory: 64Mi`; no CPU limit); the container port is named `http` with `containerPort` the port of `listenAddr` (text after its last `:`), so `[::]:8080` gives 8080, and both probes and the Service `targetPort` refer to `http`; rendering fails, with a message holding the key path without `.Values` (`httpRoute.gateway.name`, `httpRoute.gateway.namespace`, `listenAddr`), when either Gateway key is absent, `null` or `""`, when `listenAddr` has no `:`, or when the text after its last `:` is not decimal digits without sign or leading zero with a value from 1 to 65535 (`08080`, `+80` and a leading space before `8080` fail); `image.tag: x` renders the image `ghcr.io/mathieubodin/scaleway-manager:x`; `helm lint` with `ci/test-values.yaml` passes.
- **CAP-5**
  - **intent:** Every PR is checked for Go, the AD-1 rule, the chart and the image, locally and in CI, and those checks block a merge once #41 is in `main`.
  - **success:** On the PR, the CI jobs `lint-go`, `test-go`, `check-go-mod`, `lint-chart` and `build-image` are green; `make check` runs the first four, not `build-image`; Go tests (`make test-go`) of the AD-1 check program on `testdata` fixtures show it fails naming `go.mod` for a `require` line and for a `require` block, fails naming `go.sum` for a non-empty `go.sum`, and passes on a module without dependencies; `make help` lists the five targets; each target fails with an install hint when its tool is missing: `go` for the three Go targets, `docker` and `curl` for `build-image`, `helm` for `lint-chart`; `lint-chart` also fails with a hint naming `helm` 4.3.0 when `helm version --template '{{.Version}}'` is not exactly `v4.3.0`, and with a hint naming helm-unittest and its pinned version when the plugin is missing; after the merge, `gh api repos/mathieubodin/scaleway-manager/branches/main/protection/required_status_checks` lists at least these five jobs, alongside the six already required.

## Constraints

- Go standard library only: `go.mod` (Go 1.27.2) has no `require`, `go.sum` is absent or empty; test and build tools are pinned outside `go.mod` and `web/` (AD-1).
- `web/` holds only HTML, CSS and ES modules (AD-8): the `go:embed` file sits at the module root; the page has no script and no inline style.
- Exactly one pod at any time: one replica, strategy `Recreate` (sessions in memory, AD-11).
- The pod has no Kubernetes API rights: dedicated ServiceAccount, `automountServiceAccountToken: false`, no binding (AD-3, AD-12).
- No capability is added to the container; a port below 1024 is documented as unsupported in `values.yaml`, not rejected.
- The chart ships no Gateway, no Gateway API CRD and no operator CRD (AD-10, AD-11).
- Helm is exactly 4.3.0, from one Makefile variable `HELM_VERSION`; helm-unittest is 1.2.1, from `HELM_UNITTEST_VERSION`.
- The process files that list the checks stay true: `docs/development-process.md`, `scripts/evidence.sh`, and the `sdlc-work` deliver reference, which reminds the human to add a new CI job to the required checks at the merge.

## Non-goals

- Coverage, mutation testing and thresholds (#14).
- A kind cluster or any in-cluster run (#16).
- Multi-arch builds, cosign signing, provenance attestation, publication to ghcr.io, release versions (#18); the chart stays at `0.0.0`.
- The `scaleway-manager-credentials` Role in `scaleway-system` (first feature that writes a Secret).
- The `/api/events` HTTPRoute rule (stream feature).
- Any rule in the two ClusterRoles; any binding.
- Any Kubernetes API call, login, session or Scaleway readiness check.
- Rejecting a port below 1024 in the chart.
- The Go race detector.

## Success signal

On the #41 PR, the five new CI jobs are green and `make build-image` shows the container answering `/healthz` under `--read-only --cap-drop ALL --user 65532`; after the merge, those jobs are required checks of `main`, and #14, #16 and #42 can start from `main`.

## Assumptions

- SIGINT stops the server like SIGTERM (the parallax named both for the second signal; the forge named only SIGTERM).
- The AD-1 check program lives at `cmd/check-go-mod`; it is not built into the image.
- `image.pullPolicy` defaults to `IfNotPresent` (the forge decided only that it is configurable).
