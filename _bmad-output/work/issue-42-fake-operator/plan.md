# Plan: #42 fake operator

Spec: `_bmad-output/specs/spec-fake-operator/SPEC.md`, companion `fake-operator-contract.md`. Rules: AD-1, AD-2, AD-3, AD-4, AD-9 and AD-15 of the architecture spine.

## Explorations

Run `wf_a268a900-166` (code, practices, frameworks; none failed). Findings used here:

- **Everything is new.** `internal/kube`, `internal/kube/kubetest`, `cmd/fake-operator`, `tests/cluster/fake-operator/` and the CAP-9 test file in `tests/make` do not exist yet (spine "Structural Seed").
- **`main` pattern.** `cmd/scaleway-manager/main.go` wires `os.Getenv`, stdout and a signal channel into `run(...) int`, so exit codes and SIGTERM are testable. `cmd/fake-operator` follows the same shape.
- **Test conventions.** Tests are external test packages using standard `testing` and `httptest`. Linux-only tests live in `*_linux_test.go`. `tests/make` (package `make_test`) has `repoRoot` and `read` helpers. Sources: `internal/server/server_test.go`, `tests/make/make_test.go`.
- **Frozen files.** `tests/make/make_test.go`, `tests/make/wiring_test.go` and `tests/workflows/process-files-new-checks.test.js` are frozen, so CAP-9 goes to a new file in `tests/make`.
  - The frozen evidence test fixes the ten checks of `scripts/evidence.sh`. `build-fake-operator-image` therefore stays out of the evidence record and out of `make check`, like `build-image`.
- **Dockerfile.** `scripts/check-dockerfile.sh` requires exactly two `FROM` lines (the pinned golang and distroless digests) and a cross-compiling `go build` `RUN` line.
  - The root `.dockerignore` is an allowlist that already contains `cmd/` and `internal/`.
  - `scripts/build-image.sh` is the model for a `scripts/build-fake-operator-image.sh`.
- **No YAML parser in the standard library** (AD-1). Decided: the manifests are JSON.
- **Go 1.27 standard library:**
  - `uuid` (`New`, `Parse`) for `scaleway_id` and the UUID check;
  - `math/rand/v2` for an injectable random source;
  - `testing/synctest` is available;
  - `signal.NotifyContext` handles SIGTERM;
  - `httptest.NewTLSServer` and `Server.Certificate()` cover the CA tests.
- **Pitfalls.**
  - `url.URL.Path` stores decoded paths, and setting an escaped `Path` double-escapes. Set `Path` decoded with `RawPath` escaped, or validate names so that no escaping is needed. Assert on `r.RequestURI`.
  - `fmt` prints an unexported struct field's raw value without calling `String`, so `kube.Token` lives in its own type and is never printed inside a struct.
  - The API server refuses to add a finalizer once `deletionTimestamp` is set. The pass order (deletion first) avoids that, and kubetest refuses it too.
  - The liveness probe settings must exceed 3 × `FAKE_OPERATOR_STEP_DELAY`.
- **Kubernetes documentation URLs for CAP-2 citations:**
  - <https://kubernetes.io/docs/reference/using-api/api-concepts/>: list, patch, 409;
  - <https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/>: status subresource, finalizers;
  - <https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/>;
  - <https://kubernetes.io/docs/reference/access-authn-authz/authentication/>: 401;
  - <https://www.rfc-editor.org/rfc/rfc7386>.
- **Out of scope, noted.** Go 1.27.3 (2026-10-19) fixes `crypto/x509`, and the project pins 1.27.2. That is a separate tooling PR.

MemPalace was not searched: the server was not connected in this session.

## Approaches

- **A. Layered trunk, then leaves (chosen by the author).** The trunk builds the client, the fake API server and the engine. The leaves then add deletion and checks, causes, runtime and packaging in parallel. Each story covers one or two capabilities, and the contracts sit between layers.
- **B. Vertical slices per kind (set aside).** Every story would touch `internal/kube`, `kubetest` and `cmd/fake-operator`. That gives the most shared packages between stories (pre-mortem R10) and blurred contracts.
- **C. Three large stories (set aside).** The middle story would hold all fake behavior and risk exhausting the builder's two-failure rule.

Other choices:

- Manifests are JSON (author). AD-9 and the companion were updated.
- Time is injected through a small clock interface, as CAP-3 states ("injected clock"); `testing/synctest` is not required.
- Randomness comes from an injected `math/rand/v2` source. `scaleway_id` is formatted as a UUID from that source.

## Contracts

### S1 → S3: `internal/kube`

- `type Token string`: redacted by `String`, `GoString`, `LogValue` and `MarshalJSON`.
- `type Resource`: closed list of values, one per AD-2 entry:
  - `Instances`, `LoadBalancers`, `ScalewaySecrets`, `NamespaceRoles`;
  - `Namespaces`, `Secrets`, `CRDs`, `Deployments`.
- `func NewClient(host, port string, caPEM []byte) (*Client, error)`.
- `func (c *Client) List(ctx, tok Token, r Resource, namespace string) ([]json.RawMessage, error)`. An empty namespace means across namespaces.
- `func (c *Client) Get(ctx, tok Token, r Resource, namespace, name string) (json.RawMessage, error)`.
- `func (c *Client) MergePatch(ctx, tok Token, r Resource, namespace, name string, patch []byte) (json.RawMessage, error)`.
- `func (c *Client) PatchStatus(ctx, tok Token, r Resource, namespace, name string, patch []byte) (json.RawMessage, error)`.
- Errors:
  - `*APIError{Code int}` for a non-2xx answer, so callers can test 404, 401 and 409;
  - `ErrEmptyToken`;
  - `ErrInvalidName`.

### S2 → S3: `internal/kube/kubetest`

- `func New(t testing.TB) *Server`: a TLS server, stopped at test cleanup.
- Fields and accessors: `Host`, `Port`, `CAPEM []byte`.
- Seeding and reading:
  - `Put(r kube.Resource, obj json.RawMessage)`: seeds or replaces an object;
  - `Object(r, namespace, name) (json.RawMessage, bool)`;
  - `Delete(r, namespace, name)`: sets `deletionTimestamp`, or removes the object if it has no finalizers.
- Tokens and requests:
  - `ExpectToken(tok string)`;
  - `Tokens() []string`;
  - `Requests() []Request` with `Method`, `RequestURI`, `ContentType` and `Body`, in order.
- Faults: `Fail(method, pathPrefix string, code int)`, cleared by `ClearFaults()`.
- It depends on `kube.Resource` for routing. S2 therefore starts after S1, or S2 owns a copy of the path table. Decided: S2 after S1, and kubetest imports `internal/kube`.

### S3 → S4, S5, S6: `cmd/fake-operator`

S3 owns the engine and leaves one file per leaf, with a stub that has no effect, so that the parallel leaves edit separate files.

- `operator.go` (S3):
  - `type operator struct{ kube *kube.Client; clock Clock; rand *rand.Rand; tokenPath string; log *slog.Logger }`;
  - `func (o *operator) pass(ctx context.Context) error`, which runs one pass over the three kinds in the companion's pass order;
  - `type Clock interface{ Now() time.Time }`.
- `lifecycle.go` (S3): finalizer, `Syncing`, `Synced`, generated values, `observed_generation`, the write discipline (patch only on change, whole list or struct, no `resourceVersion`).
- `deletion.go` and `checks.go` (S4):
  - `func (o *operator) deleting(ctx, kind, obj) (handled bool, err error)`;
  - `func (o *operator) checkBefore(ctx, kind, obj) (failed bool, err error)`;
  - `func (o *operator) checkAfter(ctx, kind, obj) (failed bool, err error)`;
  - `func specListsValid(kind, obj) bool`.
  - S3 stubs: deletion removes the finalizer, and the checks pass.
- `causes.go` (S5): `func (o *operator) applyCause(ctx, kind, obj) (handled bool, err error)`. S3 stub: no cause.
- `main.go` (S3) calls `run`. `run.go` (S6): `func run(getenv func(string) string, stdout io.Writer, sigs <-chan os.Signal, tokenPath, caPath string, clock Clock, rnd *rand.Rand) int`, with the polling loop, the startup guards, `/healthz` and SIGTERM. S3 stub: returns 0.
- Status objects stay `map[string]any` decoded with `UseNumber` (AD-4).

### S3 → S7

S7 needs `./cmd/fake-operator` to compile; it does once S3 is merged. S7 owns:

- `cmd/fake-operator/Dockerfile`;
- `scripts/build-fake-operator-image.sh`;
- the Makefile target;
- `tests/cluster/fake-operator/*.json`;
- `tests/make/fake_operator_test.go`.

## Trunk (in order)

1. **S1 `internal/kube`**: CAP-1. URL builder, `Token`, client, errors. Tests use plain `httptest.NewTLSServer`.
2. **S2 `internal/kube/kubetest`**: CAP-2. Fake API server, each behavior's test citing its documentation URL.
3. **S3 engine**: CAP-3 and CAP-7. The pass over three kinds, lifecycle values, write discipline, and stubs for the leaves.

## Leaves (parallel, after S3)

- **S4 deletion and checks**: CAP-4 and CAP-5, in `deletion.go` and `checks.go`.
- **S5 causes**: CAP-6, in `causes.go`.
- **S6 runtime**: CAP-8, in `run.go`.
- **S7 packaging and isolation**: CAP-9: Dockerfile, script, Makefile target, JSON manifests, the `tests/make` test.

S4, S5 and S6 share the package `cmd/fake-operator` with S3 (pre-mortem R10). Their frozen tests may not compile before their story is built, so the runner map and the set-aside rule of #48 and #50 apply. Leaf tests use names unique to their story to avoid symbol clashes between test files.

## Risks

- R10, shared package `cmd/fake-operator`: frozen tests of S4, S5 and S6 break its compilation until each is merged. They are handled by the runner map and set-aside rules.
- Merge conflicts between leaves: each leaf owns its own files. The stubs of S3 fix the signatures.
- Liveness settings versus the step delay (S6, S7): the probe's `periodSeconds × failureThreshold` must exceed 3 × the delay.
