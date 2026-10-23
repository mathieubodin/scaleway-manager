---
title: 'Fake operator engine'
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

**Problem:** With a client (story 1) and a fake API server (story 2), nothing yet moves an Instance, LoadBalancer or ScalewaySecret through the operator's lifecycle.

**Approach:** Create the engine of `cmd/fake-operator` (CAP-3, CAP-7). It is one pass over the three kinds that moves each object at most one step, with the operator's status values and write discipline. It also creates the files the leaves (stories 4 to 6) will fill, each with a stub that has no effect.

## Boundaries & Constraints

**Always:**

- **Contract.** The contract is `fake-operator-contract.md`, sections "Kinds", "Status fields", "Pass order", "Lifecycle values" and "Writes". Any fact about the real operator comes from tag `scaleway-operator-v0.1.15` (see the dispatch note).
- **Engine API**, as in the plan's S3 contract:
  - `type operator struct{ kube *kube.Client; clock Clock; rand *rand.Rand; tokenPath string; log *slog.Logger }`;
  - `type Clock interface{ Now() time.Time }`;
  - `func (o *operator) pass(ctx context.Context) error`.
- **Pass order.** `pass` reads the token file once, then lists `Instances`, `LoadBalancers` and `ScalewaySecrets` across namespaces with `kube.Client.List`. For each object it applies the companion's "Pass order":
  1. `deleting`;
  2. `checkBefore`;
  3. the finalizer step;
  4. `checkAfter`;
  5. `specListsValid`;
  6. `applyCause`;
  7. the lifecycle step.

  A step that reports `handled` or `failed` ends the object's pass.
- **Stubs, signatures fixed for the leaves:**
  - `deletion.go`: `func (o *operator) deleting(ctx context.Context, k kind, obj map[string]any) (handled bool, err error)`. If `metadata.deletionTimestamp` is set, it removes the finalizer and returns `handled`.
  - `checks.go`:
    - `checkBefore(ctx, k, obj) (failed bool, err error)` and `checkAfter(ctx, k, obj) (failed bool, err error)` return `false, nil`;
    - `func specListsValid(k kind, obj map[string]any) bool` returns `true`.
  - `causes.go`: `func (o *operator) applyCause(ctx context.Context, k kind, obj map[string]any) (handled bool, err error)` returns `false, nil`.
  - `run.go`: `func run(getenv func(string) string, stdout io.Writer, sigs <-chan os.Signal, tokenPath, caPath string, clock Clock, rnd *rand.Rand) int` returns 0.
  - `main.go` calls `os.Exit(run(...))` with `os.Getenv`, `os.Stdout`, `signal.Notify` on SIGTERM, the two paths under `/var/run/secrets/kubernetes.io/serviceaccount/`, the system clock and a seeded `math/rand/v2` source.
- **`kind` type.** `type kind struct{ resource kube.Resource; name string; finalizer string }`, with three values:
  - Instance, `scaleway.mathieubodin.io/instance-finalizer`;
  - LoadBalancer, `scaleway.mathieubodin.io/loadbalancer-finalizer`;
  - ScalewaySecret, `scaleway.mathieubodin.io/secret-finalizer`.
- **Lifecycle.** The values come from the companion's "Lifecycle values":
  - finalizer only, with no status;
  - Instance and LoadBalancer: `Syncing`, then `Synced` on the next pass;
  - ScalewaySecret: two status patches in one pass, `Syncing` then `Synced`.
- **Generated values:**
  - `scaleway_id`: a UUID v4 string from the injected `rand`;
  - `public_ip`: `192.0.2.<1..254>`;
  - `vip_address`: `198.51.100.<1..254>`.

  They are written once and kept.
- **Timestamps.** `created_at` (Instance) is the injected clock's `Now()` in RFC 3339 UTC, with the fraction of a second as chrono writes it (none, 3, 6 or 9 digits).
- **ScalewaySecret versions.** When `metadata.generation` differs from a non-null `status.observed_generation`, or when the secret leaves `Revoked`, the engine pushes a version: `current_version` + 1, `observed_generation` set, `Synced`. A `Synced` secret with a null `observed_generation` records the generation without a push.
- **Writes.** As in the companion's "Writes":
  - the finalizer is written as `{"metadata":{"finalizers":[<whole list>]}}` with `MergePatch`;
  - the status is written as `{"status":{<whole struct, absent fields as null>}}` with `PatchStatus`;
  - no `resourceVersion` is sent;
  - a patch is sent only when the computed object differs from the one read.
- **Errors and logs.**
  - An error on one object is logged (`slog` JSON, never a token or body) and does not stop the pass for the others.
  - A failed `List` skips that kind.
- **AD-4.** Objects stay `map[string]any` decoded with `UseNumber`.

**Never:**

- any write on `NamespaceRole`;
- any annotation, label or event written;
- an engine behavior that belongs to a leaf (deletion beyond the stub, checks, causes, the loop or `/healthz`);
- a dependency;
- editing `internal/kube` or `internal/kube/kubetest`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Pass 1 | new Instance | finalizer patch only, no status | N/A |
| Pass 2 | Instance with finalizer | status: `Syncing`, `state: creating`, `scaleway_id`, `project_id` copied from the namespace annotation if readable, else null, `created_at`, `error_message: null` | N/A |
| Pass 3 | Instance `Syncing` | `Synced`, `state: running`, `public_ip: 192.0.2.x` | N/A |
| LoadBalancer | passes 2 and 3 | `Syncing`, `state: pending`; then `Synced`, `state: ready`, `vip_address: 198.51.100.x` | N/A |
| ScalewaySecret | pass 2 | two `/status` PATCHes in order: `Syncing` (`scaleway_id`, `current_version` null), then `Synced` (`current_version: 1`, `observed_generation`) | N/A |
| Steady | Synced object, three more passes | no PATCH; `resourceVersion` and generated values unchanged | N/A |
| Spec change | ScalewaySecret `generation` 2, `observed_generation` 1 | `current_version` +1, `observed_generation: 2`, `Synced` | N/A |
| Failed list | kubetest `Fail("GET", "/apis/scaleway.mathieubodin.io/v1/instances", 500)` | Instances skipped, the two other kinds processed | logged |

</frozen-after-approval>

## Code Map

- `internal/kube`: the client API (`List`, `MergePatch`, `PatchStatus`, `Token`), from story 1.
- `internal/kube/kubetest`: the test server (`Put`, `Object`, `Requests`, `Fail`), from story 2.
- `cmd/scaleway-manager/main.go`: the `main` → `run` shape that `main.go` copies.
- New: `cmd/fake-operator/{main.go,operator.go,lifecycle.go,deletion.go,checks.go,causes.go,run.go}` and their tests. Touch nothing else.

## Tasks & Acceptance

**Execution:**

- [ ] `cmd/fake-operator/operator.go`: `operator`, `Clock`, `kind`, `pass`.
- [ ] `cmd/fake-operator/lifecycle.go`: the finalizer step, the lifecycle steps, the version push, generated values, the diff-then-patch rule.
- [ ] `cmd/fake-operator/{deletion,checks,causes,run,main}.go`: the stubs and `main` above.
- [ ] `cmd/fake-operator/lifecycle_test.go` (package `main`): every matrix row. Each test runs `pass` with a fixed clock and a seeded `rand` against `kubetest`, with no sleep, and compares the whole status after each pass. A namespace carries a UUID `scaleway.mathieubodin.io/project-id` annotation.
- [ ] `cmd/fake-operator/writes_test.go`: CAP-7, covering:
  - the patch bodies (whole list or struct, no `resourceVersion`);
  - no PATCH on a pass with no change;
  - no request to `namespaceroles` other than GET;
  - no PATCH whose body changes `metadata.annotations` or `metadata.labels`.

**Acceptance Criteria:**

- Given the story merged, when `make lint-go test-go check-go-mod` runs, then all pass.
- Given `go build ./cmd/fake-operator`, then it builds.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

`project_id` comes from the namespace's `scaleway.mathieubodin.io/project-id` annotation, read with `kube.Client.Get(Namespaces…)` at the creation step. Checking that annotation is story 4's job. Story 3 copies the value when the namespace has it, and writes null otherwise.

Test names start with `TestLifecycle` or `TestWrites`. The leaves will use their own prefixes (`TestDeletion`, `TestChecks`, `TestCauses`, `TestRun`), so their frozen test files do not clash.

## Verification

**Commands:**

- `go test ./cmd/fake-operator/...`: expected: pass.
- `make lint-go test-go check-go-mod`: expected: exit 0.
