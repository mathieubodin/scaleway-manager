# Plan: #41 application skeleton

Spec: `_bmad-output/specs/spec-application-skeleton/SPEC.md`, contract `skeleton-contract.md`, stories `stories.yaml`. Exploration: run `wf_fc97b61f-618`, readers code, practices, frameworks, none failed. MemPalace was not searched: its MCP server was disconnected.

## Explorations

### Code

- No application code exists: every file of the contract is new, except `Makefile`, `prek.toml` and `.github/workflows/pr-checks.yml`. `frozen-tests.sha256` does not exist yet. Source: `git ls-files`, `Makefile`.
- Makefile conventions: bash with `-eu -o pipefail`; `help` lists only targets with a `##` comment; every target is in `.PHONY`; tool guards are prerequisite targets printing `Error: <tool> not found. Install with: <cmd>` (`check-node`, `check-markdownlint`). Source: `Makefile`.
- CI conventions: one job per make target, `contents: read`, actions pinned by full SHA with a version comment, `persist-credentials: false`, PR values through `env`. Source: `.github/workflows/pr-checks.yml`.
- Files that list the checks and go stale: `docs/development-process.md` ("CI"), `scripts/evidence.sh` (`checks=(...)` and the `not_run` note on code tests), `skills/sdlc-work/references/deliver.md` (no required-check reminder). Source: those files.
- `biome.json` covers every `**/*.json`; markdownlint covers the chart `README.md`; `.gitignore` has no Go build output entry. Source: `biome.json`, `.markdownlint-cli2.yaml`, `.gitignore`.

### Practices

- Lesson: run each check for real on a deliberately broken input (a `require` in `go.mod`, a broken Dockerfile, a wrong Helm version) and read the CI logs, not only the status. Source: `docs/solutions/workflow-issues/real-runs-catch-what-harness-tests-and-review-miss.md`.
- Lesson: verify tool behavior by running it, not from its documentation. Source: `docs/solutions/best-practices/verify-operator-behavior-in-the-code-that-writes-the-field.md`.
- `GET /` in `ServeMux` matches every GET path: use `GET /{$}` for the page so unknown paths get 404; `GET /healthz` also matches `HEAD`, and `net/http` sends no body on `HEAD`. The header middleware must wrap the whole mux so its 404 and 405 carry the AD-8 headers. Source: <https://pkg.go.dev/net/http>.
- `net.Listen`, then the start line, then `Serve(ln)`: `ListenAndServe` leaves no point to log between bind and serve. Source: <https://pkg.go.dev/net/http>.
- Calling `stop()` of `signal.NotifyContext` re-arms the default action, so a second signal would kill the process; keep the signals captured until exit and give `Shutdown` a fresh 10 s context. Source: <https://pkg.go.dev/os/signal#NotifyContext>.
- `http.Server.ErrorLog` writes to stderr by default. Source: <https://pkg.go.dev/net/http>.
- `gofmt -l` exits 0 even with unformatted files: test its output. Source: <https://github.com/golang/go/issues/46289>.
- `testdata` directories are ignored by the go tool, so fixture `go.mod` files with `require` do not join the module. Source: <https://pkg.go.dev/cmd/go>.
- `actions/setup-go` keys its cache on `go.sum`, which does not exist here: set `cache: false`. Source: <https://github.com/actions/setup-go/issues/427>.
- Each Dockerfile stage that uses `TARGETOS`/`TARGETARCH` must declare them with `ARG`, or they are silently empty. Source: <https://docs.docker.com/build/building/multi-platform/>.
- Pod Security Restricted is met by the contract's securityContext; `readOnlyRootFilesystem` is extra. Source: <https://kubernetes.io/docs/concepts/security/pod-security-standards/>.
- helm-unittest: `failedTemplate` needs a selected template; `notExists` asserts an absent field; set `release.name: scaleway-manager`; list `tests/` in `.helmignore`. Source: <https://github.com/helm-unittest/helm-unittest/blob/main/DOCUMENT.md>.
- Since Go 1.25, `GOMAXPROCS` follows the container CPU limit. Source: <https://go.dev/doc/go1.25>.
- An HTTPRoute in another namespace attaches only when the listener's `allowedRoutes` allows it; otherwise it is ignored silently. Source: <https://gateway-api.sigs.k8s.io/guides/multiple-ns/>.

### Frameworks

| Tool | Version | Note | Source |
| --- | --- | --- | --- |
| Go | 1.27.2 | 1.27.1 lacks the `net/http` security fixes of 1.27.2 (2026-10-08); 1.27.3 (2026-10-19) is one day old | <https://go.dev/doc/devel/release> |
| Helm | 4.3.0 | 4.3.1 is a non-security patch; local Helm is v4.3.0 | <https://github.com/helm/helm/releases> |
| helm-unittest | 1.2.1 | Full Helm 4 support since 1.2.0; 1.2.1 fixes install for Helm 4.3.0; git install on Helm 4 needs `--verify=false`, OCI install can be verified | <https://github.com/helm-unittest/helm-unittest/blob/main/CHANGELOG.md> |
| `actions/setup-go` | v7.0.0 (`b7ad1dad31e06c5925ef5d2fc7ad053ef454303e`) | `go-version-file: go.mod`, `cache: false` | <https://github.com/actions/setup-go/releases> |
| `azure/setup-helm` | v5.0.1 (`9bc31f4ebc9c6b171d7bfbaa5d006ae7abdb4310`) | Pass an explicit `version`, else it may fall back to v3.18.4 | <https://github.com/Azure/setup-helm> |
| `golang` image | 1.27.2 | Multi-arch index digest taken at build time | Docker Hub |
| `gcr.io/distroless/static-debian13:nonroot` | index digest at build time | uid 65532, no shell, no curl | <https://github.com/GoogleContainerTools/distroless> |

Not adopted, outside the spec: golangci-lint, staticcheck, hadolint, kubeconform, chart-testing, actionlint.

## Approaches

1. **Spine layout (chosen).** `internal/server` holds the handler: AD-8 header middleware around the whole mux, and the routes. `cmd/scaleway-manager` holds configuration, listening, signals and logs, through a `run` function whose shutdown delay is injected (10 s in production), so Go tests drive the lifecycle in-process. Later features add routes to `internal/server` without moving code.
2. **Everything in `cmd/`.** Fewer files now; the first feature moves the handler to `internal/server`. Set aside: a refactor planned from the start.
3. **Process-level lifecycle tests.** Spine layout, but tests run the real binary with real signals. Set aside: the shutdown-timeout test would last over 10 s.

Decisions taken in Plan with the author, also in the spec memlog: Go 1.27.2; `net/http` `ErrorLog` to slog JSON on stdout at level WARN; default resources requests `cpu: 10m`, `memory: 32Mi`, limits `memory: 64Mi`, no CPU limit; the process files are updated in #41; details E1 to E12 below.

| # | Detail |
| --- | --- |
| E1 | helm-unittest `1.2.1` in `HELM_UNITTEST_VERSION`; install method (verified OCI, or git with `--verify=false`) chosen during build on a real run and stated in the PR |
| E2 | CI Helm: `azure/setup-helm` v5.0.1 by SHA, version from `make -s print-helm-version` |
| E3 | CI Go: `actions/setup-go` v7.0.0 by SHA, `go-version-file: go.mod`, `cache: false` |
| E4 | Image digests taken at build with `docker buildx imagetools inspect` (multi-arch index digest) |
| E5 | `GET /{$}` for the page; `GET /healthz` covers `HEAD`; header middleware wraps the whole mux; page served from its bytes |
| E6 | Signals captured until exit; `Shutdown` gets a fresh 10 s context |
| E7 | `ARG TARGETOS` and `ARG TARGETARCH` in the build stage; `CGO_ENABLED=0` |
| E8 | `lint-go` fails on `gofmt -l` output, not its exit code |
| E9 | `check-go-mod` runs `go mod edit -json <file>` through `os/exec` and reads the JSON |
| E10 | `docs/migrations/next/41.md`: `Migration: no impact` |
| E11 | No extra linter |
| E12 | Spine Stack: Go 1.27.2 |

## Contracts at each boundary

- **Story 1 → story 2:** `go build ./cmd/scaleway-manager` from the module root produces the static server binary; it embeds `web/index.html`; it reads only `LISTEN_ADDR`; it answers `GET /healthz` with 200 `ok`. The Dockerfile builds that package and nothing else (not `cmd/check-go-mod`).
- **Story 1 → stories 2, 3:** the Makefile pattern story 1 sets: a tool guard target per tool, a `##` comment on each public target, every target in `.PHONY`, `check` extended with the new target; `pr-checks.yml` gets one job per target, named exactly as the target.
- **Story 2 → story 3:** the image listens on the port of `LISTEN_ADDR` and runs as uid 65532 with a read-only root and no capability; the chart's securityContext matches what `build-image` exercised.
- **Stories 1 to 3 → story 4:** the five target names `lint-go`, `test-go`, `check-go-mod`, `lint-chart`, `build-image`; `make check` runs the first four; `build-image` needs Docker and runs in CI; the required-check `gh api` command of the contract.

## Build order

- **Trunk, in order:** story 1 (Go server and AD-1 check), story 2 (container image), story 3 (Helm chart). Sequential because each edits `Makefile`, `pr-checks.yml` and `prek.toml`; story 2 also needs story 1's binary.
- **Leaves, in parallel after the trunk:** story 4 (process files).
- Story 1 has `spec_checkpoint` and `done_checkpoint`: the author reviews its story spec before the build and its result before story 2.
