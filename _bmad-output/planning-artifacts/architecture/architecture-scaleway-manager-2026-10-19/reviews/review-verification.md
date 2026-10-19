---
reviewed: ARCHITECTURE-SPINE.md (draft, 2026-10-19)
lens: verification (was each technical claim web-researched or reality-checked?)
date: 2026-10-19
verdict: changes needed
---

# Verification review: ARCHITECTURE-SPINE.md

## Verdict

Changes needed. Most claims hold. Two would break the build or the running product as written:

- The go-mutesting pin cannot be installed.
- SSE is cut by the Envoy Gateway default timeout.

Four medium findings and two low findings are in the table below.

## Method

- **Baseline:** the `(version)` and `Verified` lines in `.memlog.md` count as already verified. They are not re-checked here unless a newer fact contradicts them.
- **Web:** official docs (distroless, Kubernetes, Envoy Gateway, Helm, Playwright, StrykerJS, go-mutesting README), plus GitHub releases through `gh` and npm metadata.
- **Local:**
  - A throwaway kind cluster (server v1.36.1, kubectl v1.35.1), deleted afterwards.
  - Go 1.24.4 with `go doc` and `go install`.
  - A scratch Go module for go-mutesting.
- **Operator source:** `scaleway-operator` d6e721f (`src/resources.rs`, `src/reconcilers.rs`).

## Findings

| # | Severity | Spine location | Finding | Evidence |
| --- | --- | --- | --- | --- |
| F1 | High | Stack (go-mutesting v2.3.1), memlog test-tooling decision | `go install github.com/avito-tech/go-mutesting/cmd/go-mutesting@v2.3.1` fails: "module contains a go.mod file, so module path must match major version". The tag's `go.mod` declares a path without `/v2`. Pin the pseudo-version `v0.0.0-20251226130216-48d0401f00fb`, which is the v2.3.1 commit `48d0401f`; it installs. The binary exits 0 even when mutants survive (observed score 0.5, rc=0), and the README documents no threshold option. A wrapper script must parse "The mutation score is …" to fail the gate. The file-path claim holds: `go-mutesting p/p.go` works, and the README says "Every target can be either a Go source file, a directory or a package". | Local `go install`, `go list -m`, run in a scratch module; README |
| F2 | High | AD-7, AD-11 (HTTPRoute), Conventions `/api/events` | Envoy Proxy's default request timeout is 15 s, an absolute deadline, and Envoy Gateway keeps it. Envoy Gateway's docs say "The default request timeout is set to 15 seconds in Envoy Proxy". Every SSE stream at `/api/events` is therefore cut after 15 s (HTTP 504), and heartbeats do not help. The chart's HTTPRoute needs a rule for `/api/events` with `timeouts.request: 0s` (Gateway API standard field), or a per-route BackendTrafficPolicy. Envoy Gateway issue #9513 reports that a Gateway-targeted policy is not applied to routes. The rule also binds users' Gateway implementations, because the chart is implementation-agnostic. | gateway.envoyproxy.io/docs/tasks/traffic/http-timeouts; envoyproxy/gateway#9513 |
| F3 | Medium | Stack (Kubernetes 1.37, Gateway API v1.6.2, Envoy Gateway v1.9.2) | The official Envoy Gateway compatibility matrix lists v1.9 with Gateway API **v1.6.1** and Kubernetes **v1.33–v1.36**. It does not list 1.37. The Envoy Gateway Helm chart also installs its own Gateway API CRDs (v1.6.1) by default, which conflicts with pinning v1.6.2 separately. Either pin the kind node image at ≤1.36 for the project clusters (the Deferred item already leaves the version to #14/#16), or state that 1.37 is untested with Envoy Gateway. Also state which component owns the Gateway API CRDs in the kind clusters. | gateway.envoyproxy.io/news/releases/matrix; quickstart ("also installs the Gateway API CRDs") |
| F4 | Medium | AD-11 "Project kind clusters", Structural Seed diagram | kind has no LoadBalancer implementation, so the Envoy proxy Service of type LoadBalancer stays pending. Envoy Gateway's quickstart recommends MetalLB, or uses `kubectl port-forward` to the Envoy service. The spine names a path browser → Gateway (HTTPS) but not how the browser reaches the Gateway on kind: port-forward, NodePort with kind `extraPortMappings`, MetalLB, or cloud-provider-kind. That choice changes the make targets and the Playwright base URL. Playwright also needs `ignoreHTTPSErrors` (or the generated CA) for the self-signed certificate. The self-signed HTTPS listener itself is confirmed: openssl → `kubectl create secret tls` → listener `certificateRefs`. | Envoy Gateway quickstart and secure-gateways tasks |
| F5 | Medium | AD-4 "byte-equivalent" | The claim is not achievable as written. `encoding/json` re-encodes `map[string]any` with sorted keys and its own formatting, and only an untouched `json.RawMessage` keeps bytes. The API server also re-serializes stored objects: an integer above int64 in `spec`, applied through kubectl, was stored as `1.2345678901234567e+19`. "Semantically equal" is the reachable property. A safer rule is to send only the edited fields as a JSON merge patch (`application/merge-patch+json`, with `metadata.resourceVersion` for optimistic concurrency), so `spec` is never rewritten. Strategic merge patch is rejected on CRs (verified: "application/strategic-merge-patch+json is not supported"). | kind reality check |
| F6 | Medium | AD-5 export / re-import | (a) `v1 List` is client-side only. `kubectl apply -f` accepts it (verified: Namespace with annotation, then a namespaced CR, re-apply "unchanged"). The API server has no `lists` resource (raw POST gives 404), so a server-side re-import must unwrap `items` itself, Namespace first. (b) `kubectl apply` adds the `kubectl.kubernetes.io/last-applied-configuration` annotation to every object. The spine's kept/dropped list says nothing about `metadata.annotations`, `metadata.labels` or `metadata.namespace` on CR items, so a re-export after a kubectl re-import would carry that annotation. The test-pinned field list should state these three fields. | kind reality check |
| F7 | Low | Conventions "in-cluster API address and CA from the pod's service account mount", AD-12 | The address does not come from the mount. It comes from `KUBERNETES_SERVICE_HOST` and `KUBERNETES_SERVICE_PORT_HTTPS`; the mount holds `token`, `ca.crt` and `namespace`. If the chart sets `automountServiceAccountToken: false`, the natural choice for a pod account with no rights (AD-12), `ca.crt` disappears as well (verified: directory absent, env var present). The chart must then mount the `kube-root-ca.crt` ConfigMap explicitly. | kubernetes.io access-api-from-pod; kind pod check |
| F8 | Low | AD-7, AD-3 | The watch relay's lifecycle is unspecified. The API server ends watches on its own timeout (kube-apiserver `--min-request-timeout`, default 1800 s, randomized; from the flag reference, not re-checked live this session). The relay must resume from the last `resourceVersion` and relist on `410 Gone`. Reconnection uses the user's token, which `kubectl create token` issues for 1 h by default, minimum 10 min (verified: a 5 m request is rejected). The session's practical lifetime is the token's, and the spine should say so, since it affects AD-7. | kind check (`create token`, `?watch=1` stream format); memlog |

## Claims confirmed (not in memlog, now verified)

| Spine claim | Result | Evidence |
| --- | --- | --- |
| `gcr.io/distroless/static-debian13`, nonroot | Confirmed. Tags `latest`, `nonroot`, `debug`, `debug-nonroot`, for amd64 and arm64 among others. Use `CGO_ENABLED=0` for a truly static binary. | GoogleContainerTools/distroless README |
| Go std lib covers: in-cluster config, TLS with custom CA, watch, SSE, slog, embed, ServeMux | Confirmed. `crypto/x509` + `net/http` with bearer header work like the doc's curl example. A watch is a stream of JSON objects (`{"type":"ADDED","object":…}`) that `json.Decoder` reads. `http.ResponseController` has `Flush` and `SetWriteDeadline`, needed so `Server.WriteTimeout` does not kill SSE. `log/slog.NewJSONHandler` and `http.Cookie.SameSite` exist. ServeMux patterns and `go:embed` are in the memlog. | `go doc`; kind `?watch=1` curl |
| Status subresource PATCH/PUT for CRDs | Confirmed. A merge patch of `status` on the main resource is ignored ("no change"). The same patch with `--subresource=status` applies, and `generation` stays 1. The operator's four CRDs have the subresource (memlog). | kind reality check |
| Field names `status.sync_state`, `status.error_message`; finalizer names (AD-9) | Confirmed. No `rename_all` in `resources.rs`, so the JSON names are snake_case. Finalizer constants are in `reconcilers.rs:19-21`. | operator d6e721f |
| `kubectl create token` for a ServiceAccount; 401/403 as a `Status` | Confirmed. The token works against the API; the forbidden list returns `kind: Status`, `reason: Forbidden` (Conventions "Errors" holds). `SelfSubjectReview` v1 returns the username. | kind reality check |
| Playwright: open shadow roots reachable, closed not (AD-8) | Confirmed: "Closed-mode shadow roots are not supported"; XPath does not pierce. The ARIA snapshot page itself is silent on shadow DOM. | playwright.dev/docs/locators |
| StrykerJS tap-runner with `node:test` | Confirmed. The default `nodeArgs` since v9 are `--test-reporter=tap -r {{hookFile}} {{testFile}}`. Limits: granularity is the test file, and there is one process per file. `@stryker-mutator/core` 10.0.0 was published 2026-08-14 on npm, but the stryker-js git repo has no v10 tag (its last tag is v9.6.1). | stryker-mutator.io tap-runner; npm |
| Helm 4 `helm push` to OCI | Confirmed in the Helm 4.3.1 docs (`helm push [chart] [remote]`, `oci://` without basename or tag). The registries page warns it is "not yet updated for Helm 4". | helm.sh/docs/helm/helm_push |
| Envoy Gateway HTTPS listener with self-signed cert | Confirmed (see F4 for access). The chart is `oci://docker.io/envoyproxy/gateway-helm --version v1.9.2`. | Envoy Gateway secure-gateways task |

## Version drift since the memlog check (all respect the two-week rule)

- Gateway API v1.6.3 (2026-10-06, 13 days old): v1.6.2 stays correct under the rule. Re-check after 2026-10-20.
- Helm v4.3.1 (2026-10-15): v4.3.0 stays correct.
- Envoy Gateway v1.9.2 (2026-09-28) is the latest.

## Coherence notes (outside the verification lens, low)

- The memlog test-tooling decision lists Biome. The spine's Stack table does not.
- The author's machine has Go 1.24.4 (and Helm 3.17, already noted). The spine pins Go 1.27.1.
- AD-9: the real operator's default `sync_state` is `Syncing` for Instance and ScalewaySecret, and `Pending` for LoadBalancer (`resources.rs:100,177,260`). If the fake operator always starts at `Pending`, it differs from the real one for two kinds.

## Sources

- <https://github.com/GoogleContainerTools/distroless>
- <https://github.com/avito-tech/go-mutesting>
- <https://playwright.dev/docs/locators>, <https://playwright.dev/docs/aria-snapshots>
- <https://stryker-mutator.io/docs/stryker-js/tap-runner/>
- <https://helm.sh/docs/helm/helm_push/>, <https://helm.sh/docs/topics/registries/>
- <https://gateway.envoyproxy.io/news/releases/matrix/>
- <https://gateway.envoyproxy.io/docs/tasks/quickstart/>
- <https://gateway.envoyproxy.io/docs/tasks/security/secure-gateways/>
- <https://gateway.envoyproxy.io/docs/tasks/traffic/http-timeouts/>
- <https://github.com/envoyproxy/gateway/issues/9513>
- <https://kubernetes.io/docs/tasks/run-application/access-api-from-pod/>
