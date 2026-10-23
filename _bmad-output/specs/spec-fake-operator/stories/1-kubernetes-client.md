---
title: 'Kubernetes client internal/kube'
type: 'feature'
created: '2026-10-23'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-fake-operator/SPEC.md'
  - '{project-root}/_bmad-output/work/issue-42-fake-operator/plan.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Nothing in the repository talks to the Kubernetes API. The fake operator (stories 3 to 6) and later the server need one client that builds every URL the same way (AD-2), takes the caller's token on each call (AD-3) and keeps objects as raw JSON (AD-4).

**Approach:** Create the package `internal/kube` (CAP-1): the AD-2 URL builder over the closed list, a redacted `Token`, and a client with `List`, `Get`, `MergePatch` and `PatchStatus`. Use only the standard library.

## Boundaries & Constraints

**Always:**

- Standard library only, with no `require` in `go.mod` (AD-1).
- **Closed list.** `type Resource int` with exactly these constants. A value outside the list is refused with `ErrUnknownResource`.

  | Constant | Path prefix | Scope | Operations allowed in this story |
  | --- | --- | --- | --- |
  | `Instances` | `/apis/scaleway.mathieubodin.io/v1`, `instances` | namespaced | list, get, merge patch, status patch |
  | `LoadBalancers` | same group, `loadbalancers` | namespaced | same |
  | `ScalewaySecrets` | same group, `scalewaysecrets` | namespaced | same |
  | `NamespaceRoles` | same group, `namespaceroles` | cluster | list, get |
  | `Namespaces` | `/api/v1`, `namespaces` | cluster | list, get |
  | `Secrets` | `/api/v1`, `secrets` | namespaced | none (URL only; the server adds its operations) |
  | `CRDs` | `/apis/apiextensions.k8s.io/v1`, `customresourcedefinitions` | cluster | get |
  | `Deployments` | `/apis/apps/v1`, `deployments` | namespaced | list |

- **Errors.**
  - An operation outside this table returns `ErrOperationNotAllowed` and sends nothing.
  - A non-empty namespace on a cluster kind, or an empty namespace on a namespaced get or patch, returns `ErrInvalidName`.
- **Path building.** One exported function builds every path: `Path(r Resource, namespace, name string, status bool) (string, error)`. Every request uses it.
  - A namespace must pass the DNS-1123 label rule: at most 63 characters, `[a-z0-9-]`, alphanumeric at both ends.
  - A resource name must pass the DNS-1123 subdomain rule: at most 253 characters, `[a-z0-9.-]`, alphanumeric at both ends.
  - Any failure, including `/`, `%`, `.` or `..`, returns `ErrInvalidName` before any request.
  - Since valid names never need escaping, `url.URL.Path` holds the plain path.
- **Address and trust.**
  - `NewInCluster(getenv func(string) string, caPEM []byte) (*Client, error)` reads `KUBERNETES_SERVICE_HOST` and `KUBERNETES_SERVICE_PORT`. An unset or empty value returns `ErrNoAPIServer`.
  - The base URL is `https://` plus `net.JoinHostPort(host, port)`.
  - The `tls.Config` uses `RootCAs` built only from `caPEM`. PEM with no certificate returns `ErrNoCA`.
- **Requests.**
  - Every request sends `Authorization: Bearer <token>` and `Accept: application/json`.
  - Only `MergePatch` and `PatchStatus` send a body, with `Content-Type: application/merge-patch+json`.
  - An empty token returns `ErrEmptyToken` and sends nothing.
- **Responses.**
  - Response bodies are returned as `json.RawMessage`. `List` returns the elements of `items`.
  - A non-2xx answer returns `*APIError{Code int, Body []byte}`. It satisfies `errors.As`, and its `Error()` names the method, the path and the code only.
- **`type Token string`.**
  - `String`, `GoString` and `LogValue` (`slog.LogValuer`) return `[REDACTED]`.
  - `MarshalJSON` returns `"[REDACTED]"`.
  - The client never logs.

**Never:** read a token file or any file; a watch, create, update or delete; a `fmt` or `slog` call that prints a request, a header or a body; any query parameter; `InsecureSkipVerify`; a dependency.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Paths | each constant, namespace `ns`, name `n`, `status` true for the three operator kinds | exact strings, e.g. `/apis/scaleway.mathieubodin.io/v1/namespaces/ns/instances/n/status`, `/api/v1/namespaces/n`, `/apis/apps/v1/namespaces/ns/deployments` | N/A |
| List across namespaces | `List(tok, Instances, "")` | `GET /apis/scaleway.mathieubodin.io/v1/instances`, items returned | N/A |
| Bad names | namespace `Ns`, `a.b`, 64 chars; name `a/b`, `a%2Fb`, `..`, 254 chars | `ErrInvalidName`, no request | error returned |
| Headers | list, get, two patches | Bearer on all four; merge-patch `Content-Type` on the patches only | N/A |
| Empty token | `Token("")` | `ErrEmptyToken`, no request | error returned |
| Not allowed | `Get(Secrets…)`, `MergePatch(Deployments…)` | `ErrOperationNotAllowed`, no request | error returned |
| API error | server answers 404, 409, 401 | `*APIError` with that code | error returned |
| Environment | host or port unset or `""` | `ErrNoAPIServer` | error returned |
| IPv6 | host `::1` | base `https://[::1]:<port>` | N/A |
| Trust | server cert from another CA | TLS error, no response read | error returned |
| Redaction | `fmt.Sprint`, `%#v`, `slog` JSON, `json.Marshal` of a `Token` | `[REDACTED]` only | N/A |

</frozen-after-approval>

## Code Map

- `internal/server/server.go`, `internal/server/server_test.go`: package layout and test style to follow (external test package, `httptest`, `t.Helper`).
- `cmd/scaleway-manager/main.go`: the `getenv func(string) string` injection that `NewInCluster` follows.
- `tests/make/wiring_test.go` (frozen, do not edit): it asserts `go.mod` has no `require`.
- New: `internal/kube/` only. Do not touch `cmd/`, `internal/server/` or any frozen file.

## Tasks & Acceptance

**Execution:**

- [ ] `internal/kube/token.go`: `Token` and its four redacting methods.
- [ ] `internal/kube/resource.go`: `Resource`, the table above, the name rules, `Path`.
- [ ] `internal/kube/client.go`: `NewInCluster`, `Client` with `List`, `Get`, `MergePatch` and `PatchStatus` (all taking `ctx context.Context, tok Token` first), plus `APIError` and the sentinel errors `ErrUnknownResource`, `ErrOperationNotAllowed`, `ErrInvalidName`, `ErrEmptyToken`, `ErrNoAPIServer` and `ErrNoCA`.
- [ ] `internal/kube/*_test.go` (package `kube_test`): every matrix row. The tests use `httptest.NewTLSServer` and write `Server.Certificate()` as PEM for `caPEM`. A second `httptest.NewTLSServer` provides the "other CA" case. The tests assert on `r.RequestURI`, `r.Method` and the headers recorded by the handler.

**Acceptance Criteria:**

- Given the story merged, when `make lint-go test-go check-go-mod` runs, then all three pass.
- Given `internal/kube`, when its imports are listed (`go list -deps ./internal/kube`), then only standard-library packages appear.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

`List` with an empty namespace on a namespaced kind lists across namespaces. On a cluster kind, the namespace must be empty. `Deployments` list requires a namespace (the fake lists `scaleway-system`).

`Path` is exported so the closed list is testable without a server. The client methods call it and never concatenate a path themselves.

## Verification

**Commands:**

- `go test ./internal/kube/...`: expected: pass.
- `make lint-go test-go check-go-mod`: expected: exit 0.
