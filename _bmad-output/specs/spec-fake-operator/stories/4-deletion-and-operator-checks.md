---
title: 'Deletion and operator checks'
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

**Problem:** The engine (story 3) removes a finalizer on any deletion and creates objects whatever their namespace or spec. The real operator checks the NamespaceRole, the role, the `project-id` annotation, the spec lists, the region and the source, and blocks some deletions.

**Approach:** Fill `deletion.go` and `checks.go` of `cmd/fake-operator` (CAP-4, CAP-5) with the operator's checks, order and messages, from `fake-operator-contract.md`, sections "Pass order" and "Checks". The `outcome` annotation belongs to story 5, except the deletion rules for `Error`, `Locked` and refused causes, which live here (see Design Notes).

## Boundaries & Constraints

**Always:**

- Keep the signatures story 3 fixed:
  - `deleting(ctx, k, obj) (handled bool, err error)`;
  - `checkBefore(ctx, k, obj) (failed bool, err error)`;
  - `checkAfter(ctx, k, obj) (failed bool, err error)`;
  - `specListsValid(k, obj) bool`.
- Every check failure writes `sync_state: Error` with the companion's exact message, through a whole-struct status patch. The other fields keep their current values, or the kind's Rust defaults when there is no status yet (Instance `state: "unknown"`, LoadBalancer `state: ""`).
- **Region (ScalewaySecret).** `spec.region` must be one of `fr-par`, `nl-ams`, `pl-waw`. This check runs first, deletion included: `deleting` checks it before anything else. A failure keeps the finalizer and writes `Configuration error: Invalid region '<r>'. Use one of: fr-par, nl-ams, pl-waw.`
- **Deletion** (`metadata.deletionTimestamp` set). The fake skips every other check and does one of three things:
  - under `outcome: Error`, it keeps the finalizer and sends no status patch;
  - under `outcome: Locked` on a LoadBalancer, it writes `sync_state: TerminationBlocked` with `error_message: Load balancer is locked — deletion blocked by Scaleway` and keeps the finalizer;
  - otherwise it removes the finalizer, including under a refused cause, and sends no status patch.

  The finalizer is removed with a whole-list merge patch.
- **`checkBefore`.** In the companion's order:
  - NamespaceRole named after the namespace (all three kinds). A 404 gives the `No NamespaceRole found` message, any other error gives `Kubernetes API error`;
  - for ScalewaySecret, the read-only role (`cannot sync secrets`);
  - for Instance and LoadBalancer, the `project-id` annotation. A missing annotation gives the `must have annotation` message, a namespace read error gives `Configuration error: Cannot access namespace <ns>: <error text>`, and a non-UUID value gives `must be a valid UUID, got: '<pid>'`.

  A failure returns `failed` before the finalizer step, so a new object gets no finalizer.
- **`checkAfter`.**
  - Instance and LoadBalancer: the read-only role, only while `status.scaleway_id` is null (`cannot create instances` or `cannot create load balancers`).
  - ScalewaySecret:
    - no `spec.source.kubernetes_secret` gives `Secret source not configured: spec.source must have kubernetes_secret configured`;
    - at creation, a missing `project-id` annotation gives the same message as Instance, with no UUID check, and a namespace read error gives `Cannot access namespace`.
- **`specListsValid`.**
  - `spec.zone` must be one of `fr-par-1`, `fr-par-2`, `nl-ams-1`, `pl-waw-1`, `sg-sin-1`, `it-mil-1` (Instance and LoadBalancer).
  - `spec.instance_type` must be one of `DEV1-S`, `DEV1-M`, `DEV1-L`, `DEV1-XL`, `GP1-XS`, `GP1-S`, `GP1-M`, `GP1-L`, `GP1-XL`, `CPU1-XS`, `CPU1-S`, `CPU1-M`, `CPU1-L`, `GPU-3090`, `GPU-4090` (Instance).
  - `spec.lb_type` must be one of `LB-S`, `LB-GP` (LoadBalancer).
  - These lists come from `src/scaleway.rs` at the tag.
  - A false result makes the engine end the object's pass with no status write; the finalizer stays.
- Roles allowed: exactly `Editor`, `Admin`, `OrganizationOwner`, read from `spec.scaleway_role` of the NamespaceRole.

**Never:**

- implementing the `outcome` causes outside deletion (story 5);
- reading a Secret;
- editing files outside `cmd/fake-operator/deletion.go`, `checks.go` and their tests;
- changing story 3's signatures.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Delete, no cause | each kind, namespace with neither NamespaceRole nor annotation, valid region for the secret | finalizer removed, no status patch | N/A |
| Delete, Error | each kind, `outcome: Error` | finalizer kept, no status patch | N/A |
| Delete, Locked | LoadBalancer, `outcome: Locked` | `TerminationBlocked` and its message, finalizer kept | N/A |
| Delete, refused | Instance `Locked`, ScalewaySecret `NotFound`, LoadBalancer `Revoked` | finalizer removed, no status patch | N/A |
| Delete, bad region | ScalewaySecret, region `xx-yyy` | finalizer kept, region `Error` | N/A |
| No NamespaceRole | each kind, new object | `Error`, `No NamespaceRole found…`, no finalizer | N/A |
| NamespaceRole read error | each kind, kubetest `Fail` 500 on the get | `Error`, `Kubernetes API error`, no finalizer | N/A |
| Read-only role | Instance and LoadBalancer with finalizer and no `scaleway_id`; ScalewaySecret new | `Error` with the kind's message; finalizer present (Instance, LoadBalancer), absent (ScalewaySecret) | N/A |
| Annotation | Instance and LoadBalancer: missing, not a UUID, namespace read error; ScalewaySecret at creation: missing, namespace read error | the row's message; no finalizer (Instance, LoadBalancer), finalizer present (ScalewaySecret) | N/A |
| No source | ScalewaySecret without `spec.source.kubernetes_secret` | `Error`, `Secret source not configured…`, finalizer present | N/A |
| Spec lists | zone (Instance, LoadBalancer), `instance_type` (Instance), `lb_type` (LoadBalancer) out of list | no status written, finalizer present | N/A |

</frozen-after-approval>

## Code Map

- `cmd/fake-operator/operator.go` and `lifecycle.go` (story 3): the engine calls these functions in the pass order. Reuse its status-patch helper.
- `cmd/fake-operator/causes.go` (story 5, built in parallel): do not edit.
- `internal/kube/kubetest` (story 2): `Put` namespaces and NamespaceRoles, `Delete`, `Fail`.

## Tasks & Acceptance

**Execution:**

- [ ] `cmd/fake-operator/deletion.go`: the region check first for ScalewaySecret, then the deletion rules.
- [ ] `cmd/fake-operator/checks.go`: `checkBefore`, `checkAfter`, `specListsValid`, the messages as constants.
- [ ] `cmd/fake-operator/deletion_test.go`, `checks_test.go`: one test per matrix row and kind. Test names start with `TestDeletion` and `TestChecks`. For `Cannot access namespace`, the test asserts the exact prefix `Configuration error: Cannot access namespace <ns>:` plus one space, followed by non-empty text.

**Acceptance Criteria:**

- Given the story merged, when `make lint-go test-go check-go-mod` runs, then all pass.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

Deletion reads `metadata.annotations["fake.scaleway-manager.test/outcome"]` itself, only to tell `Error`, `Locked` and the other values apart. Which values are refused for which kind follows the companion's "Causes" table: `Locked` is valid only for LoadBalancer and `Error` for all kinds. Story 5 owns every other use of the annotation.

## Verification

**Commands:**

- `go test ./cmd/fake-operator/...`: expected: pass.
- `make lint-go test-go check-go-mod`: expected: exit 0.
