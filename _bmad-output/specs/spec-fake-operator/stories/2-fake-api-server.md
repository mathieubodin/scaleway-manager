---
title: 'Fake API server internal/kube/kubetest'
type: 'feature'
created: '2026-10-23'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-fake-operator/go-interfaces.md'
  - '{project-root}/_bmad-output/specs/spec-fake-operator/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-fake-operator/fake-operator-contract.md'
  - '{project-root}/_bmad-output/work/issue-42-fake-operator/plan.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The fake operator's Go tests (stories 3 to 6) need a Kubernetes API server that behaves like the real one on the points they rely on (AD-15). Without one, they would test a fake against an imagined server.

**Approach:** Create the package `internal/kube/kubetest` (CAP-2): a TLS `httptest` server holding objects in memory, which answers the requests `internal/kube` (story 1) sends. Each behavior has a test citing the Kubernetes documentation it reproduces.

## Boundaries & Constraints

**Always:**

- Standard library plus `internal/kube`. Routing uses `kube.Path`, so the server answers exactly the paths the client builds.
- **API, as in the plan's S2 contract:**
  - `New(t testing.TB) *Server`, which starts a `httptest.NewTLSServer` and closes it at cleanup;
  - fields `Host`, `Port` and `CAPEM []byte` (PEM of `Server.Certificate()`);
  - `Put(r kube.Resource, obj json.RawMessage)`, which seeds or replaces an object by `metadata.namespace` and `metadata.name`, sets `resourceVersion`, and sets `generation` to 1 when it is absent;
  - `Object(r, namespace, name) (json.RawMessage, bool)`;
  - `Delete(r, namespace, name)`;
  - `ExpectToken(tok string)`, `Tokens() []string`;
  - `Requests() []Request{Method, RequestURI, ContentType, Body}`, in order;
  - `Fail(method, pathPrefix string, code int)`, `ClearFaults()`.
- **Behaviors, one test each, each test's comment citing the URL:**
  1. **List across namespaces and in one namespace:** `{"kind":"…List","items":[…]}`. <https://kubernetes.io/docs/reference/using-api/api-concepts/>
  2. **Get of a namespace and of a cluster-scoped `NamespaceRole`:** 404 when absent. Same URL.
  3. **List of deployments in a namespace.** Same URL.
  4. **Merge patch on the object:** RFC 7386 (`null` removes a key, arrays replaced). `status` in the patch is ignored, and `metadata.generation` is incremented when `spec` changes. <https://www.rfc-editor.org/rfc/rfc7386>, <https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource>
  5. **Merge patch on `/status`:** only `status` is applied. Same URLs.
  6. **Delete with finalizers:** `Delete` sets `metadata.deletionTimestamp` (RFC 3339) and keeps the object. A patch that empties `finalizers` then removes it. Without finalizers, `Delete` removes the object at once. <https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/>
  7. **`resourceVersion`:** a decimal string, bumped on each change.
  8. **No-op patch:** `resourceVersion` kept on a patch that changes nothing. <https://kubernetes.io/docs/reference/using-api/api-concepts/>
  9. **Stale `resourceVersion`:** a patch carrying a stale `metadata.resourceVersion` gets 409. Same URL.
  10. **No bearer token:** 401. <https://kubernetes.io/docs/reference/access-authn-authz/authentication/>
  11. **Token recording:** every received token is recorded, and a token other than the expected one gets 401 once `ExpectToken` is set. Same URL.
  12. **Content type:** 415 on a `PATCH` whose `Content-Type` is not `application/merge-patch+json`. <https://kubernetes.io/docs/reference/using-api/api-concepts/#patch-and-apply>
  13. **Errors:** 404 on an unknown object. Same URL as 1.
- **Fault injection:** `Fail` answers matching requests with the code until `ClearFaults`. It has one table test, with no URL required.
- **Storage:** objects are stored as `map[string]any` decoded with `UseNumber`. Every response is re-encoded from that map, with no struct.
- **Concurrency:** safe for concurrent requests (one mutex).

**Never:** a watch, selectors, pagination, server-side apply, JSON patch or strategic merge patch; a dependency; any file outside `internal/kube/kubetest/`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Status isolation | object patch `{"spec":{"x":1},"status":{"a":1}}` | `spec.x` set, `status` unchanged, `generation` +1 | N/A |
| Status only | `/status` patch `{"metadata":{"labels":{"l":"v"}},"status":{"a":1}}` | `status.a` set, labels unchanged, `generation` unchanged | N/A |
| Null removal | `/status` patch `{"status":{"a":null}}` | `status.a` absent | N/A |
| Finalizers emptied while deleting | object with `deletionTimestamp`, patch `{"metadata":{"finalizers":[]}}` | object gone; `Object` returns false | N/A |
| No-op | patch equal to current content | 200, same `resourceVersion` | N/A |
| Stale | patch with an old `resourceVersion` | 409 | 409 |

</frozen-after-approval>

## Code Map

- `internal/kube/` (story 1): use `kube.Path`, `kube.Resource` and `kube.NewInCluster`. Tests drive the server through a real `kube.Client` built with `Host`, `Port` and `CAPEM`, and use raw `http` requests only for cases the client refuses to send (no token, wrong content type).
- `internal/server/server_test.go`: test style.
- New: `internal/kube/kubetest/` only.

## Tasks & Acceptance

**Execution:**

- [ ] `internal/kube/kubetest/server.go`: store, routing through `kube.Path`, the behaviors above, the request and token log, faults.
- [ ] `internal/kube/kubetest/mergepatch.go`: RFC 7386 merge.
- [ ] `internal/kube/kubetest/*_test.go` (package `kubetest_test`): one test per behavior 1 to 13, each with a `// Reproduces: <URL>` comment; the matrix rows; one table test for `Fail`.

**Acceptance Criteria:**

- Given the story merged, when `make lint-go test-go check-go-mod` runs, then all pass.
- Given `grep -c 'Reproduces: https://' internal/kube/kubetest/*_test.go`, then the total is at least 13.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Verification

**Commands:**

- `go test ./internal/kube/...`: expected: pass.
- `make lint-go test-go check-go-mod`: expected: exit 0.
