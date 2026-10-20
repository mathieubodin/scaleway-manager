# Skeleton contract (#41)

What each part must be, field by field. `SPEC.md` holds the capabilities and their verification; the architecture spine holds the rules cited here.

## Files

| Path | Holds |
| --- | --- |
| `go.mod` | `module`, `go 1.27.2`, nothing else |
| `internal/server/` | Handler: AD-8 header middleware around the whole mux, routes |
| `cmd/scaleway-manager/` | `main`: configuration, listening, signals, logs, through a testable `run` function whose shutdown delay is injected (10 s in production) |
| `cmd/check-go-mod/` | AD-1 check program, standard library, with `testdata/` fixtures; not in the image |
| Module root, one Go file | `go:embed` of `web/` (so `web/` holds no Go file) |
| `web/index.html` | Placeholder page |
| `Dockerfile`, `.dockerignore` | Image |
| `charts/scaleway-manager/` | Chart, `ci/test-values.yaml`, `tests/` (helm-unittest), `.helmignore`, `README.md` |
| `Makefile`, `prek.toml`, `.github/workflows/pr-checks.yml` | Checks |
| `docs/development-process.md`, `scripts/evidence.sh`, `skills/sdlc-work/references/deliver.md` (and its installed copy) | Process files that list the checks |
| `docs/migrations/next/41.md` | `Migration: no impact` |

## Server

- **Configuration:** one environment variable, `LISTEN_ADDR`; unset or empty gives `:8080`.
- **Routes:** `GET /healthz` → `200`, body `ok` (no trailing newline); `HEAD /healthz` → `200`, empty body. `GET /` → `web/index.html`, embedded. Anything else → `404`.
- **Headers on every response,** `/healthz` and `404` included: `Content-Security-Policy: default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'` and `X-Content-Type-Options: nosniff`.
- **Timeouts:** `ReadHeaderTimeout` 10 s; no `WriteTimeout` (the future SSE stream).
- **Stop:**
  - on SIGTERM or SIGINT, `http.Server.Shutdown` with a 10 s timeout; requests in flight complete;
  - a second signal during shutdown is ignored;
  - a shutdown over 10 s writes one ERROR line instead of the stop line and exits 1;
  - a bind failure writes one ERROR line, no start line, and exits 1;
  - a clean stop exits 0; the 10 s run from the first signal to the end of `Shutdown`.
- **Logs:** every line is one `log/slog` JSON object on stdout: start (INFO, `msg` `"listening"`, field `addr`, written once after listening succeeds), stop (INFO, `msg` `"stopped"`), and the two ERROR lines. A second signal and a request write no line. `http.Server.ErrorLog` writes the internal errors of `net/http` as slog JSON on stdout at level WARN.

## Page

`web/index.html`: title `scaleway-manager`, the sentence "scaleway-manager is running; no feature is available yet.", a viewport meta, no script, no inline style.

## Image

- **Stages:** exactly two, build then runtime.
- **Build stage:** `FROM --platform=$BUILDPLATFORM golang:1.27.2@sha256:<digest> AS build`, with `ARG TARGETOS` and `ARG TARGETARCH`. The static binary (`CGO_ENABLED=0`) is built by a `RUN` line containing `GOOS=$TARGETOS GOARCH=$TARGETARCH go build`.
- **Runtime stage:** `FROM gcr.io/distroless/static-debian13:nonroot@sha256:<digest>`.
- **Local build:** host architecture only, unsigned.
- **`.dockerignore`:** excludes everything the build does not need, for example `.git`, `node_modules`, `_bmad*`, `.claude`, `coverage`, `docs`, `tests/workflows`.

## Chart

- **`Chart.yaml`:** `version` and `appVersion` `0.0.0`. `values.yaml` notes that no image is published before #18, so an install needs an `image.tag` override until then.
- **Names:** namespaced objects use the standard Helm fullname rule; for the release name `scaleway-manager` they are named `scaleway-manager`. The ClusterRoles have fixed names (AD-12): one install per cluster, stated in `README.md`.

### Values

```yaml
listenAddr: ":8080"        # port >= 1024; a lower port is unsupported (no capability is added)
image:
  repository: ghcr.io/mathieubodin/scaleway-manager
  tag: ""                  # empty: Chart appVersion
  pullPolicy: IfNotPresent
httpRoute:
  gateway:
    name: ""               # required
    namespace: ""          # required
    sectionName: ""        # optional: the HTTPS listener
  hostnames: []            # optional
resources:
  requests: {cpu: 10m, memory: 32Mi}
  limits: {memory: 64Mi}   # no CPU limit: GOMAXPROCS follows it
```

Rendering fails with a message naming the key in either of these cases:

- `httpRoute.gateway.name` or `httpRoute.gateway.namespace` is absent, `null` or `""`;
- a valid port in `listenAddr`: the text after its last `:` must be decimal digits, no sign, no leading zero, from 1 to 65535, so `[::]:8080` gives `8080` and `08080`, `+80`, a leading space before `8080` fail; an address without `:` fails.

The messages hold the key path without `.Values` (`httpRoute.gateway.name`, `listenAddr`).

### Objects

| Object | Fields |
| --- | --- |
| ServiceAccount | Dedicated; `automountServiceAccountToken: false`; no binding |
| Deployment | `replicas: 1`; `strategy.type: Recreate`; `serviceAccountName` the one above; `automountServiceAccountToken: false`; env `LISTEN_ADDR` from `listenAddr`; container port named `http`, `containerPort` the port of `listenAddr`; readiness and liveness `httpGet /healthz` on port `http`, no timing field (Kubernetes defaults); `resources` from values |
| Pod `securityContext` | `runAsNonRoot: true`, `runAsUser: 65532`, `seccompProfile.type: RuntimeDefault` |
| Container `securityContext` | `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, nothing added |
| Service | Port 80 → `targetPort: http` |
| HTTPRoute (`gateway.networking.k8s.io/v1`) | `parentRefs`: the Gateway's `name` and `namespace`, plus `sectionName` when set; `hostnames` when set; one rule, `PathPrefix /`, to the Service on port 80 |
| ClusterRole `scaleway-manager-namespace` | No rules, unbound |
| ClusterRole `scaleway-manager-cluster` | No rules, unbound |

The chart contains no Role in `scaleway-system`, no RoleBinding and no ClusterRoleBinding.

## Checks

### Make targets

Each target fails with an install hint naming its own tool when it is missing, like `check-node`: `go` for `lint-go`, `test-go`, `check-go-mod`; `helm` at `HELM_VERSION` and the helm-unittest plugin for `lint-chart`; `docker` for `build-image`.

| Target | Does | In `make check` |
| --- | --- | --- |
| `lint-go` | `gofmt` (no diff) and `go vet ./...` | yes |
| `test-go` | `go test ./...`, without `-race` | yes |
| `check-go-mod` | Runs `cmd/check-go-mod`: parses `go.mod` (as `go mod edit -json` does); fails on any `require`, line or block, or on a non-empty `go.sum`, naming the file | yes |
| `lint-chart` | Checks `helm version --template '{{.Version}}'` is exactly `v` + `HELM_VERSION` (`v4.3.0`), else a hint naming `helm` 4.3.0; checks the helm-unittest plugin, else a hint naming it and its pinned version; `helm lint` with `ci/test-values.yaml`; helm-unittest tests | yes |
| `build-image` | First checks the Dockerfile: every `FROM` carries `tag@sha256:`, the build stage carries `--platform=$BUILDPLATFORM` and uses `TARGETOS`/`TARGETARCH`. Then builds the image, runs it `--read-only --cap-drop ALL --user 65532` with a published port, waits up to 30 s for `200 ok` on `/healthz` from the host (distroless has no curl), stops it, fails without that answer. Digests are 64 lowercase hex characters | no |

- **Go version:** comes from the `go 1.27.2` line. Locally, `GOTOOLCHAIN=auto` fetches it, which needs network on the first run; the install hint names Go 1.27.2.
- **Helm version:** comes from `HELM_VERSION` only; CI reads it through `make -s print-helm-version`.
- **helm-unittest:** pinned at `HELM_UNITTEST_VERSION` (1.2.1).

### prek

- `check-yaml` excludes `charts/*/templates/`.
- Local hook `make lint-go` on `.go` files and `go.mod`.
- Local hook `make lint-chart` on `charts/`.
- No `test-go` hook.

### CI (`pr-checks.yml`)

- One job per target above.
- Go: `actions/setup-go`, pinned by SHA, with `go-version-file: go.mod` and `cache: false` (no `go.sum`).
- Helm: exactly `HELM_VERSION`, plus the pinned helm-unittest.
- Each action pinned by SHA, `persist-credentials: false`, as the existing jobs.

### Required checks, after the merge

`sdlc-work` reminds the human at the merge. The human, with admin rights, runs:

```bash
gh api -X POST repos/mathieubodin/scaleway-manager/branches/main/protection/required_status_checks/contexts \
  --input - <<'JSON'
{"contexts":["lint-go","test-go","check-go-mod","lint-chart","build-image"]}
JSON
```

This adds to the six checks already required; it removes none.
