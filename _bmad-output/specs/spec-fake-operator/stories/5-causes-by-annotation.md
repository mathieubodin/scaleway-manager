---
title: 'Causes by annotation'
type: 'feature'
created: '2026-10-23'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-fake-operator/go-interfaces.md'
  - '{project-root}/_bmad-output/specs/spec-fake-operator/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-fake-operator/fake-operator-contract.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Tests cannot yet force what Scaleway would answer: an object lost, an API error, a locked load balancer, a revoked secret opt-in, or a changed source secret.

**Approach:** Fill `causes.go` of `cmd/fake-operator` (CAP-6). Read `fake.scaleway-manager.test/outcome`, `fake.scaleway-manager.test/message` and `fake.scaleway-manager.test/source-version`, and write the effect the operator writes for that kind, from `fake-operator-contract.md`, section "Causes".

## Boundaries & Constraints

**Always:**

- **Signature.** Keep story 3's signature `applyCause(ctx, k, obj) (handled bool, err error)`. The engine calls it after the checks and spec lists, before the lifecycle step. It returns `handled` when a cause or `source-version` produced the pass's write, or decided that there is none.
- **State model, with no memory.** A cause applies on every pass while the annotation is present. Removing it lets the lifecycle resume.
- **Effects outside deletion:**
  - **`NotFound`:**
    - Instance: `Syncing`, `state: "unknown"`, with `scaleway_id`, `public_ip`, `project_id`, `created_at` and `error_message` null.
    - LoadBalancer: `Pending`, `state: ""`, with `scaleway_id`, `vip_address`, `project_id` and `error_message` null.
    - The object stays like this while the annotation is present. Once it is removed, the next passes re-create it (`Syncing`, then `Synced`) with new generated values.
  - **`Error`:**
    - Instance and LoadBalancer: `Error`, with `error_message: Scaleway API error: 500 Internal Server Error`; the other fields are kept.
    - ScalewaySecret: the same, but only at creation, while `scaleway_id` is null. On a synced secret it has no effect: `handled`, no write.
  - **`Locked`:** on a LoadBalancer outside deletion, no effect (`handled` false, the normal lifecycle runs).
  - **`Revoked`** (ScalewaySecret only):
    - with `scaleway_id`: `Revoked`, `error_message: Source Secret opt-in revoked — Scaleway version disabled. Re-add label/annotation to re-sync.`, `scaleway_id` and `current_version` kept, `last_synced_resource_version` null;
    - without `scaleway_id`: `Error`, `Source Secret unavailable (see operator logs)`, and no finalizer. This is evaluated before the finalizer step, as the operator does, so story 5 also exposes `func revokedBeforeFinalizer(k kind, obj map[string]any) bool`, which the engine calls before adding the finalizer.
- **Refused causes.** A cause a kind does not have gives `Error`, `fake-operator: cause '<v>' is not supported for <Kind>`. That covers:
  - `NotFound` on ScalewaySecret;
  - `Locked` on Instance or ScalewaySecret;
  - `Revoked` on Instance or LoadBalancer;
  - any unknown value such as `Foo`.

  An empty `outcome` counts as absent.
- **`message` annotation.** When non-empty, it replaces the message written by `Error`, `Locked` and `Revoked`. It is ignored with no cause, with `NotFound`, and with a refused cause.
- **`source-version: "<n>"`.** It applies only on a synced ScalewaySecret with no cause, and when `n` is greater than `current_version`: it writes `current_version: n` and `Synced`; an equal or lower `n` lets the lifecycle run. A new secret gets `current_version: 1` from the engine first, and `n` on the next pass. A value that is not a positive integer up to 4294967295 (the operator's `current_version` is a `u32`) gives `Error`, `fake-operator: source-version '<v>' is not a positive integer`.
- **Writes.** A whole-struct status patch, only when the result differs from the object read.

**Never:**

- the deletion rules (story 4);
- reading a Secret;
- editing files other than `causes.go`, its test, and the single call to `revokedBeforeFinalizer` in `operator.go`;
- writing annotations: the fake never removes the `outcome` annotation itself.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| NotFound kept | Synced Instance, `outcome: NotFound`, two passes | cleared state on pass 1, no PATCH on pass 2 | N/A |
| NotFound removed | annotation removed | `Syncing` with a new `scaleway_id`, then `Synced` | N/A |
| Error, synced secret | Synced ScalewaySecret, `outcome: Error` | no PATCH | N/A |
| Locked outside delete | Synced LoadBalancer, `outcome: Locked` | no PATCH | N/A |
| Revoked, no id | new ScalewaySecret, `outcome: Revoked` | `Error`, `Source Secret unavailable…`, no finalizer | N/A |
| Refused | `Foo` on each kind; each refused cell | `Error`, refusal message | N/A |
| Message | `Error` + `message: boom` on each kind; `Locked` on delete is story 4; `Revoked` with id + `message` | `error_message: boom` | N/A |
| Message ignored | `message` with no cause, with `NotFound`, with `Foo` | message not written | N/A |
| source-version | synced secret `"3"`; new secret `"3"`; `"0"`, `"x"` | `current_version: 3`; `1` then `3`; the invalid message | N/A |

</frozen-after-approval>

## Code Map

- `cmd/fake-operator/operator.go` and `lifecycle.go` (story 3): the call site of `applyCause`, and the status-patch helper to reuse. Add only the `revokedBeforeFinalizer` call before the finalizer step.
- `cmd/fake-operator/deletion.go` and `checks.go` (story 4, built in parallel): do not edit.

## Tasks & Acceptance

**Execution:**

- [ ] `cmd/fake-operator/causes.go`: `applyCause`, `revokedBeforeFinalizer`, the cause table per kind, the messages as constants.
- [ ] `cmd/fake-operator/operator.go`: one call to `revokedBeforeFinalizer` before the finalizer step.
- [ ] `cmd/fake-operator/causes_test.go`: one test per cell and branch of the companion's "Causes" table, plus every matrix row. Test names start with `TestCauses`.

**Acceptance Criteria:**

- Given the story merged, when `make lint-go test-go check-go-mod` runs, then all pass.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Verification

**Commands:**

- `go test ./cmd/fake-operator/...`: expected: pass.
- `make lint-go test-go check-go-mod`: expected: exit 0.
