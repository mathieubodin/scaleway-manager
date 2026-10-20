# Forged idea: application skeleton (#41)

Outcome: hardened. Rules: architecture spine AD-1, AD-3, AD-8, AD-11, AD-12.

## Scope

- `go.mod` (Go 1.27.1, no `require`), `cmd/scaleway-manager`, `web/index.html`, Dockerfile, chart `charts/scaleway-manager`.
- Verification shipped by #41: make targets for `go vet` + `go test`, `helm lint` + `helm template`, `docker build`; a CI job running them; the AD-1 check that fails when `go.mod` gains a `require`.
- Left to others: coverage, mutation and thresholds (#14); kind cluster (#16); multi-arch, cosign, attestation, ghcr.io publication, release versions (#18); `/api/events` HTTPRoute rule (stream feature); `scaleway-manager-credentials` Role (first feature that writes a Secret).

## Server

- One environment variable, `LISTEN_ADDR`, default `:8080`. A port below 1024 is documented as unsupported in `values.yaml`, not rejected by the chart.
- `GET` and `HEAD` `/healthz`: `200`, body `ok`; used by readiness and liveness probes.
- `/`: static `web/index.html`, embedded with `go:embed`, titled `scaleway-manager`, one sentence saying no feature exists yet; no script, no inline style.
- AD-8 headers (`Content-Security-Policy`, `X-Content-Type-Options: nosniff`) on every response, `/healthz` and 404 included.
- SIGTERM: `http.Server.Shutdown` with a 10 s timeout.
- `log/slog` JSON on stdout: one line at start (address), one at stop; no per-request log.

## Image

Static binary on `gcr.io/distroless/static-debian13:nonroot`, built locally for the host architecture, unsigned.

## Chart

- `version` and `appVersion` `0.0.0`. `image.repository` defaults to `ghcr.io/mathieubodin/scaleway-manager`, tag defaults to `appVersion`, `image.pullPolicy` configurable.
- Deployment: one replica, probes on `/healthz`, dedicated ServiceAccount with `automountServiceAccountToken: false` and no binding; securityContext `runAsNonRoot`, uid 65532, `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]` with nothing added, `seccompProfile: RuntimeDefault`; modest default requests and limits, overridable.
- Service: port 80 to 8080.
- HTTPRoute (`gateway.networking.k8s.io/v1`): one rule, path prefix `/`, to the Service. Gateway name and namespace required, rendering fails with a clear message when missing; listener `sectionName` and `hostnames` optional.
- Two ClusterRoles without rules, unbound: `scaleway-manager-namespace`, `scaleway-manager-cluster`.

## Rejected, and why

- Shipping the `scaleway-manager-credentials` Role now: `scaleway-system` exists only after the real operator's install, which the automated-test cluster never runs; the install would fail there.
- A default Gateway in `values.yaml`: a route attached to no listener fails silently.
- The `/api/events` rule now: the stream does not exist; the rule grows with it, like the roles.
- Any capability added: a non-root process binds 8080 without `CAP_NET_BIND_SERVICE`; the Service maps 80 to 8080 outside the pod.
- Local make targets without CI: Go code would merge checked by nothing.

## Hidden constraints

- The prek builtin `check-yaml` rejects Helm templates: `charts/*/templates/` must be excluded from it.
- `pr-checks.yml` has no Go job today.
