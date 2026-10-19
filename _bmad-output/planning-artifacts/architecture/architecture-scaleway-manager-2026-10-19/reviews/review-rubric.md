# Review: architecture spine, good-spine rubric and security lens

- Reviewed: `ARCHITECTURE-SPINE.md` (status `draft`, 2026-10-19) and `.memlog.md` in the same folder.
- Also read: `AGENTS.md`, `Makefile`, `package.json`, `.nvmrc`, `biome.json`, `.c8rc.json`, `prek.toml`, `.github/workflows/*.yml`, `docs/development-process.md`, the brief and its addendum, the forged idea, and `scaleway-operator` at `d6e721f` (`src/resources.rs`, `src/reconcilers.rs`, `charts/scaleway-operator-crds`).
- Spine not modified.

## Verdict

The spine is a sound base (stack named, zero-dependency rule enforceable, export and fake-operator rules precise and checked against the operator), but it is not ready to bind features: it leaves the write path (how a CR is updated, how re-import applies), Secret handling, input-to-URL building, CSRF and HTML rendering undecided, and those are exactly where the first features (readiness and gap filling, namespace CRUD, export and re-import) will diverge or open a hole.

## 1. Rubric

### 1.1 Real divergence points for the features built next

Next features, from the brief: readiness check and gap filling (creates Namespace with annotation, `NamespaceRole`, Secrets `scaleway-credentials` and `scaleway-ns-creds-{ns}`), namespace and CR listing with live status, create / edit-when-`Synced` / delete per kind, JSON export and re-import.

Covered well: shipped dependencies (AD-1), outbound peer (AD-2), whose identity calls Kubernetes (AD-3), raw JSON (AD-4), export content (AD-5), server-side policy (AD-6), one SSE stream (AD-7), browser code form (AD-8), fake operator (AD-9), CRD source (AD-10), packaging (AD-11).

Missed divergence points:

| # | Divergence point | Why two units diverge | Severity |
| --- | --- | --- | --- |
| D1 | Write method for an existing CR: `PUT` of the whole object, JSON merge patch, strategic merge, or server-side apply (and its `fieldManager`) | AD-4 says `spec` is written back "byte-equivalent except edited fields" but not how. A `PUT` built from a stale read silently reverts concurrent changes; a merge patch without `resourceVersion` skips the `Synced` check race (see S6); SSA without a fixed `fieldManager` creates field-ownership conflicts between features. Each form feature will pick its own | High |
| D2 | Re-import semantics: create-only, skip existing, overwrite, or apply; ordering (Namespace, then `NamespaceRole`, then CRs); partial failure | AD-5 fixes the format only. Re-import onto a cluster where some objects exist is the disposable-cycle success criterion; two answers produce either duplicates-by-overwrite or a half-imported namespace | Medium |
| D3 | What a Secret "by reference" is in a `v1 List` | A `List` holds only objects. Options: omit Secrets entirely (the `ScalewaySecret` CR already references them by `spec.source.kubernetes_secret.name/key`), or include a Secret with empty `data` (which `kubectl apply` creates as an empty Secret and the operator then reads as empty credentials). Export and re-import features can pick different answers | Medium |
| D4 | Secret handling outside export: whether the server ever returns Secret `data` to the browser, reads Secrets with data, or relays Secret watch events over SSE | Gap filling writes the IAM key; a later "show/edit credentials" feature or a readiness feature watching Secrets would push key material to the browser through AD-7. No rule forbids it (see S4) | High |
| D5 | Browser-server API resource shape: route scheme for CRs (`/api/namespaces/{ns}/{kind}/{name}` with a closed kind list, or per-feature handlers), SSE event names and payload (raw watch event `{type, object}` or a wrapper), initial list vs. snapshot-on-connect | "JSON under `/api/`" and "shapes pass through unchanged" leave each feature to invent its own routes and event format; the shared page module of AD-7 then has to support several | Medium |
| D6 | `NamespaceRole` under AD-6 | `NamespaceRole` has `status.validation_state` (`Validated`, `Invalid`), not `sync_state` (`src/resources.rs` line ~297). AD-6 as written rejects every update of a `NamespaceRole`; AD-9 does not cover `NamespaceRole` at all, so its readiness and gap-filling scenarios have no fake behavior | Medium |
| D7 | Policy coverage of non-CR writes and cluster-scoped objects | AD-6's forbidden-namespace list applies to "Scaleway CRs". Gap filling also writes a `NamespaceRole` (cluster-scoped, named like the namespace, `spec.namespace`), a `scaleway-ns-creds-{ns}` Secret and the `project-id` annotation; none is covered, so a `NamespaceRole` named `default` passes | Medium |
| D8 | HTML rendering of cluster data | `status.error_message` comes from Scaleway, names and tags from users; one component using `innerHTML` makes an XSS that owns a session able to delete infrastructure without confirmation (see S5) | High (as security) |

### 1.2 Is every Rule enforceable and does it prevent its divergence

| AD | Enforceable | Prevents its divergence | Notes |
| --- | --- | --- | --- |
| AD-1 | Yes (check on `go.mod`) | Mostly | Also forbid the `tool` directive (Go 1.24+ adds `require` lines for tools; pin tools with `go run module@vX.Y.Z`). The "web/ loads nothing not served by the binary" half needs a mechanism: a `Content-Security-Policy: default-src 'self'` header enforces it at run time and in Playwright tests |
| AD-2 | Partly | Yes | Enforceable by a check that only `internal/kube` imports `net/http` client code, or that `http.Client` is constructed only there. Interacts with the deferred readiness question: the operator health endpoint must be reached, if at all, through the API server service proxy, not a direct `Service` call |
| AD-3 | Partly | Partly | "Never logged" is not checkable as stated: give the token its own Go type whose `String`, `GoString`, `LogValue` and `MarshalJSON` redact (S3). Cookie attributes and session lifetime incomplete (S2). "Shown to the user as is" plus the error envelope is consistent |
| AD-4 | Partly | No (D1) | The decoding rule is checkable in review; the write rule is not, because the write method is unspecified |
| AD-5 | Yes (pinned tests in one function) | Partly (D2, D3) | Labels and non-`project-id` annotations: kept or dropped is unstated (`kubectl.kubernetes.io/last-applied-configuration` would be exported) |
| AD-6 | Yes | Partly (D6, D7, S6) | Check-then-write needs `resourceVersion` to be race-free |
| AD-7 | Partly | Yes for connection count | Watch scope unstated: a user with namespace-only RBAC cannot open cluster-wide watches; `410 Gone` / bookmark resync and token expiry mid-watch are a single-unit concern but deserve one line. Envoy route timeouts close long SSE streams unless a heartbeat is sent; say "heartbeat comment every N s" |
| AD-8 | Yes (tsc, Playwright) | Yes | `tsc` version open ("to pin in #14") is acceptable |
| AD-9 | Yes | Partly (D6) | Manifest location for the fake operator and its RBAC is unstated (`tests/e2e/cluster/`?) |
| AD-10 | Yes | Yes | |
| AD-11 | Yes | Partly | See release gaps (O3) |
| AD-12 | Yes | Partly | See S7 and the memlog contradiction (C2) |

### 1.3 Deferred items that could let two units diverge

- **Session persistence, more than one replica:** safe only if the chart prevents `replicas > 1`. Without a fixed `replicas: 1` (no value exposed, no HPA), a user sets 2 and sessions break at random. Turn into a rule.
- **Readiness verification:** acceptable as deferred, but state that it stays inside AD-2 (through the API server only), otherwise the first readiness feature can add a second outbound peer.
- **Identity beyond pasted tokens:** acceptable; but the chart is published for "any cluster" in version one, so the session and CSRF rules below are needed now, not at that revisit.
- Coverage thresholds, kind node image, Node 26, screen design, stage 3: cannot cause divergence. Fine.

### 1.4 Ratification of the existing repository

| File | Status | Finding |
| --- | --- | --- |
| `.nvmrc` (`24`) | Ratified | Stack says Node 24, bump to 26 deferred. Consistent |
| `package.json` | Ratified in spirit | Stack lists c8, Playwright, StrykerJS, TypeScript but not Biome 2.5.15, oxlint 1.86.0, oxfmt 0.71.0, markdownlint-cli2 0.23.3, already pinned. List them so the Stack is the single list |
| `biome.json` | Gap | `files.includes` covers only `tests/workflows/**/*.js` and JSON. `web/**/*.js` and `tests/e2e/**` are not linted or formatted by anything until it is extended. The spine names `tsc` as the only JS check; the memlog names Biome. Decide: Biome for `web/` and `tests/e2e/` (extend `includes`) |
| `.c8rc.json` | Conflict risk | c8 reads `.c8rc.json` by default, which pins `src` to workflow scripts and 100 % thresholds. JS coverage of `web/` must use a separate config (`c8 --config`), or every run inherits the workflow thresholds. State it |
| `Makefile` | Partly ratified | AD-10 adds a variable; fine. Target names for Go tests, Go lint, JS tests, e2e, test cluster, manual cluster, and whether `make check` includes them are not fixed. `docs/development-process.md` says `make check` is what the `sdlc-review` fixer runs, so it must include `go test` and `go vet`/`gofmt`, or the fixer never sees Go breakage. State that |
| `docs/development-process.md` | Ratified | Fake operator, manual cluster target, behavior diff local-only, migration files, "Deliver" all match. The process says the evidence record adds code tests, coverage, mutation and behavior diff "once the stack is chosen"; the spine should name the targets `evidence.sh` will call |
| `.github/workflows/pr-checks.yml` | Gap | No CI decision for Go jobs, JS web tests or the kind e2e job, yet the environment table says automated tests run in "CI". Which jobs become required status checks of `main` is open (O2) |
| `.github/workflows/deliver.yml` | Gap | Today it only tags and releases, with `permissions: contents: write, issues: read`. AD-11 needs `packages: write` and `id-token: write` (keyless cosign, attestation) and an ordering decision (O3) |
| `AGENTS.md` | Contradiction (pending) | Still says "no code, no stack chosen" and "A YAML export". Memlog decision (line 38, 40) says both must change; the spine and AGENTS.md currently disagree on the export format. Must change in the same PR as the spine |
| `prek.toml` | Neutral | Will need `gofmt`/`go vet` hooks once Go exists; not a spine matter |

### 1.5 Dimensions at application altitude

| Dimension | State | Gap |
| --- | --- | --- |
| Language, runtime, dependencies | Decided | |
| Module boundaries | Decided | |
| Data shape and persistence | Decided (cluster is the store; sessions in memory) | |
| API contract browser-server | Partly | D5 |
| Write semantics and concurrency | Open, not listed | D1, S6 |
| Import semantics | Open, not listed | D2, D3 |
| Error model | Decided | |
| Logging | Decided | Enforcement (S3) |
| Metrics, tracing | Not mentioned | Defer explicitly ("none in v1") |
| Config | Decided (env vars) | The variables are not named; at least listen address, log level |
| Local development loop | Not mentioned | In-cluster config only means no `go run` against a cluster without a kubeconfig parser; state "runs only in a cluster" or "dev flag reads API URL, CA, token from env" |
| Testing | Decided | |
| Deployment and environments | Decided (three environments) | Pod-level: O1 |
| CI | Not decided | O2 |
| Release | Partly | O3 |
| Operations | Absent | O1 |
| Security: TLS | Decided at the Gateway | Gateway-to-pod hop is plaintext and carries the token at login; state it as accepted |
| Security: session | Partly | S2 |
| Security: CSRF | Absent | S1 |
| Security: token handling | Partly | S3, S4 |
| Security: headers, XSS | Absent | S5 |
| Security: RBAC | Partly | S7 |
| Security: input validation | Absent | S8 |
| Supply chain | Decided (AD-1, signed image) | Vulnerability scan (`govulncheck`, a tool) not mentioned |

### 1.6 Operational envelope

- **O1 Operations (absent):** liveness and readiness endpoints and probes; `replicas: 1` fixed; resource requests and limits; pod `securityContext` (`runAsNonRoot`, `readOnlyRootFilesystem`, drop all capabilities, `seccompProfile: RuntimeDefault`, `allowPrivilegeEscalation: false`); graceful shutdown that closes SSE streams; upgrade = all sessions dropped (accepted, state it in the migration notes convention).
- **O2 CI (absent):** which jobs (Go test with coverage, Go vet/gofmt, tsc, Biome on `web/`, JS unit tests, kind e2e with the fake operator, go-mutesting on changed files) and which become required status checks. Whether the kind e2e runs on every PR or only before release.
- **O3 Release (partly):** image tag scheme (`v1.2.3` or `1.2.3`), chart `version` and `appVersion` coupling, order in "Deliver" (build, push, sign the image and chart before creating the tag and GitHub release, so a failed push leaves no published half-release, given "no published version is ever removed"), and the reinstall-previous-version command the process says arrives with publishing.

## 2. Security lens

Context: the server holds every logged-in user's bearer token in memory and relays Kubernetes calls with it; the UI deletes Scaleway infrastructure without confirmation, by policy. Any request forgery, XSS or token leak therefore converts directly into destroyed cloud resources or a leaked IAM key.

| # | Finding | Severity | Proposed fix (spine text) |
| --- | --- | --- | --- |
| S1 | **No CSRF rule.** `SameSite=Strict` does not cover same-site origins (another app on a sibling subdomain behind the same Gateway is "same-site"), older browsers, or login CSRF (an attacker logs the victim in with the attacker's token). No rule on content type of state-changing requests | High | New rule in AD-3: every non-safe method passes `http.CrossOriginProtection` (Go std lib since 1.25, checks `Sec-Fetch-Site`/`Origin`), the login route included; state-changing routes accept only `application/json`; no state change on `GET`. State that this is request authenticity, not a deletion safeguard, so it does not conflict with the "no safeguard on deletion" policy |
| S2 | **Session incomplete.** Unstated: session ID entropy, cookie name prefix and scope, session lifetime relative to the token's expiry (`kubectl create token` defaults to 1 h), logout, session creation before token verification, bound on sessions in memory | Medium | Cookie `__Host-smgr-session` (`Secure`, `Path=/`, no `Domain`, `HttpOnly`, `SameSite=Strict`); ID 32 bytes from `crypto/rand`, new ID at each login; a session is created only after the token passes a `SelfSubjectReview`; session expires at the token's `exp` (decoded, not trusted for auth) or after an idle limit, whichever first; `POST /api/logout` deletes it server side and closes its SSE stream; cap on live sessions |
| S3 | **Token leakage paths not closed by construction.** "Never logged" relies on discipline. Open paths: token in a URL (query string of login or of `EventSource`, which cannot set headers) lands in Gateway access logs; `fmt`/`slog` of a struct holding the token; an error that wraps the request; echoing the login input in an error message | High | Token only in a `POST` JSON body at login, never in a URL; a `kube.Token` type whose `String`, `GoString`, `LogValue` and `MarshalJSON` return `[redacted]`; no logging of request bodies or `Authorization`/`Cookie` headers; login errors never echo input; a test greps logs from the e2e run for the token |
| S4 | **Secret values can reach the browser.** AD-5 forbids Secret values in export only. Nothing forbids a route returning Secret `data`, or AD-7 relaying a Secret watch event (which carries `data`) | High | New rule: the server never sends Secret `data` or `stringData` to the browser, on any route or SSE event; it reads Secrets as metadata only (`Accept: application/json;as=PartialObjectMetadata;g=meta.k8s.io;v=v1`); Secret values are write-only (create or replace from a form, never displayed). Pin with a server test on every route that can return a Secret |
| S5 | **No XSS or framing rule.** Cluster data (`status.error_message` from Scaleway, names, tags, descriptions) is rendered by hand-written Web Components | High | AD-8 addition: cluster data enters the DOM only through `textContent`, attributes set with `setAttribute`, or `<template>` cloning; never `innerHTML`, `insertAdjacentHTML` or `outerHTML` with data (lint rule). Server sends `Content-Security-Policy: default-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'none'`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`. The CSP also enforces AD-1's "nothing not served by the binary" |
| S6 | **Edit-when-`Synced` has a race.** Read status `Synced`, then write: the operator can move the object to `Syncing` in between | Medium | AD-6: the server reads the object, checks `status.sync_state == Synced`, then writes with that object's `metadata.resourceVersion` (PUT, or a patch carrying `resourceVersion`); a `409 Conflict` is returned to the user as is. Ties to D1 |
| S7 | **RBAC breadth.** A ClusterRole holding `secrets` `get`/`create`/`update` bound by `ClusterRoleBinding` grants Secret access in every namespace, `kube-system` included. The pod service account token is mounted although AD-12 says the pod needs no rights. Memlog line 35 says "user rights are the cluster admin's business", AD-12 ships a growing user ClusterRole: contradiction | Medium | Split the user role in two ClusterRoles: `scaleway-manager-user-namespaced` (CRs, Secrets, intended for per-namespace `RoleBinding`) and `scaleway-manager-user-cluster` (namespaces, `NamespaceRole`, CRD read for readiness, `selfsubjectreviews create`); document the binding pattern in the chart `NOTES.txt`. Pod: `automountServiceAccountToken: false`, CA mounted from the `kube-root-ca.crt` ConfigMap, API address from `KUBERNETES_SERVICE_HOST`/`PORT`. Resolve the memlog contradiction at the gate |
| S8 | **Input reaches Kubernetes URL paths.** `ServeMux` `{name}` wildcards are matched on escaped segments and `PathValue` returns the unescaped value: `a%2F..%2F..%2Fsecrets%2Fx` becomes `a/../../secrets/x`. Concatenated into `/apis/scaleway.mathieubodin.io/v1/namespaces/{ns}/instances/{name}`, that redirects the call to another resource with the user's token. Same risk for kind, query parameters (`labelSelector`, `fieldSelector`, `dryRun`) passed through, and names taken from an imported file | High | Rule in `internal/kube`: URLs are built by one function from a closed allow-list of (group, version, resource) and from namespace and name validated as DNS-1123 label (namespace) and subdomain (name), then `url.PathEscape`d; any failure is a 400 before any call; no query parameter from the browser is forwarded; the API base URL comes only from the in-cluster environment. Re-import validates every object's `apiVersion`/`kind` against the same allow-list. `http.MaxBytesReader` on every body, import included |

Also noted (low): bodies of Kubernetes `Status` errors are passed through as is, which is fine (they carry no token); Go's client strips `Authorization` on cross-host redirects, but the client should refuse redirects outright (`CheckRedirect` returning an error); a login-attempt rate limit is cheap and avoids using the pod as a token oracle against the API server.

## 3. Coherence (cross-file)

| # | Finding | Severity |
| --- | --- | --- |
| C1 | `AGENTS.md` still says "A YAML export" and "no code, no stack chosen"; spine AD-5 says JSON. Memlog lines 38 and 40 record the update as required | Medium |
| C2 | Memlog line 35 (user rights are the cluster admin's business; chart RBAC covers only the pod) vs. AD-12 and assumption line 36 (chart ships a growing user ClusterRole) | Medium |
| C3 | AD-3 is marked `[ADOPTED]` and contains `SameSite=Strict`, which memlog line 41 lists among unvalidated architect proposals. AD-4, AD-7, AD-12 correctly carry no `[ADOPTED]` marker; the cookie attributes and the Consistency Conventions should be marked the same way until the gate | Low |
| C4 | Memlog line 26 says the server uses "the pod service account"; superseded by AD-3 and line 35 but not marked superseded | Low |
| C5 | String constants checked against the operator at `d6e721f`: finalizers (`…/instance-finalizer`, `…/loadbalancer-finalizer`, `…/scalewaysecret-finalizer`), `status.sync_state`, `status.error_message`, group `scaleway.mathieubodin.io/v1`, annotation `scaleway.mathieubodin.io/project-id`: all match. `NamespaceRole` uses `validation_state` (D6). The operator's `ScalewaySecret` print column reads `.status.syncState` while the field is `sync_state` (operator-side bug, no effect here) | Info |
| C6 | Stack omits tools already pinned in `package.json` (Biome, oxlint, oxfmt, markdownlint-cli2) | Low |

## 4. Prioritized fixes

1. Add the security rules S1, S3, S4, S5, S8 (one new AD "Request and data hygiene", or additions to AD-3, AD-4, AD-8). High.
2. Fix the write path: D1 (one write method, `resourceVersion` carried) and S6. High.
3. Decide re-import semantics and Secret-by-reference form: D2, D3. Medium.
4. Fix the API contract: D5 (route scheme with closed kind list, SSE event format). Medium.
5. Extend AD-6 and AD-9 to `NamespaceRole` and non-CR writes: D6, D7. Medium.
6. Add an operations and CI section: O1, O2, O3, and turn "one replica" into a rule. Medium.
7. Ratify the tooling files: Biome includes for `web/` and `tests/e2e/`, separate c8 config, `make check` scope, Stack list. Low to medium.
8. Resolve C1 and C2 at the Setup gate. Medium.
