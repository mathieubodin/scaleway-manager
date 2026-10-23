# Forged idea: fake operator (#42)

Test-only Go controller `cmd/fake-operator` that mimics the observable Kubernetes behavior of scaleway-operator, so automated tests can drive CR status without Scaleway. Contract: the scaleway-operator release `v0.1.15` (CRDs `0.1.14`). AD-9 of the architecture spine, corrected after this forge and re-verified at that release, prevails where it differs from this file.

## Scope

- In: `cmd/fake-operator`; `internal/kube`, limited to what it uses; `internal/kube/kubetest`; a test image; raw manifests; Go tests against `kubetest`.
- Out: the automated-test kind cluster, the pinned operator CRDs (AD-10), and deploying there: a separate issue. Watch: comes to `internal/kube` with the server.
- All outcomes from the start, per kind, as the real operator produces them. Reason: a nominal-only fake would be flagged against AD-9 by the coherence review and force a second round; outcomes are a table on top of the costly part.

## Behavior

- **Steps:** the same order as the real operator, one per pass: finalizer only, no status; `Syncing`; `Synced`. Delay between steps from a Deployment environment variable, default 2 s.
- **Polling:** every step delay, list the three kinds in all namespaces and move each object one step. No watch.
- **Preconditions on the real cluster state:** for Instance and LoadBalancer, the namespace annotation `scaleway.mathieubodin.io/project-id` (UUID) and a `NamespaceRole` named after the namespace with role `Editor`, `Admin` or `OrganizationOwner`. On failure: `sync_state: Error` with the real operator's messages; no finalizer for a missing role or annotation, finalizer kept for a read-only role (checked after it is added). The full order is in AD-9.
- **Writes:** the finalizer by merge patch on the object, with the whole list; status by merge patch on `/status`, with the whole struct and null fields deleted. Nothing on `NamespaceRole`, no annotations, labels or events.
- **Status values:**
  - `scaleway_id`: a random UUID.
  - `project_id`: copied from the namespace annotation.
  - `state`, Instance: `creating`, then `running`.
  - `state`, LoadBalancer: `pending`, then `ready`.
  - `public_ip`: `192.0.2.x`; `vip_address`: `198.51.100.x` (RFC 5737).
  - `created_at`: Instance only.
  - `current_version`: starts at 1.
  - `last_synced_resource_version`: never written.
- **Causes:** `fake.scaleway-manager.test/outcome` carries the Scaleway-side cause, never the `sync_state`. The fake writes the effect the real operator writes for that kind:

  | Cause | Instance | LoadBalancer | ScalewaySecret |
  | --- | --- | --- | --- |
  | `NotFound` | `Syncing`, ids, IP, project, `created_at` cleared; re-created once the annotation is removed | `Pending`, ids, VIP, project cleared; re-created once the annotation is removed | refused |
  | `Error` | `Error` | `Error` | `Error` |
  | `Locked` | refused | on delete: `TerminationBlocked`, finalizer kept | refused |
  | `Revoked` | refused | refused | `Revoked` |

- **State model, no memory:** a cause applies on every pass while the annotation is present. Removing it lets the object recover (`Synced`, or the deletion goes through).
- **Refused cause:** `sync_state: Error` with an `error_message` naming the unsupported cause.
- **Message:** `fake.scaleway-manager.test/message` is copied to `status.error_message`.
- **New secret version:** `fake.scaleway-manager.test/source-version: "<n>"`. When it differs from `current_version`, the fake writes `current_version: n` and `Synced`. This replaces a source Secret change, which the fake cannot see.

## Identity and RBAC

- `internal/kube` reads no token: every call takes a `kube.Token` (AD-3). `cmd/fake-operator` reads its projected ServiceAccount token at each polling cycle and passes it in. Its Deployment mounts the token.
- ClusterRole:
  - `get`, `list` on `namespaces` and `namespaceroles`;
  - `list`, `patch` on `instances`, `loadbalancers` and `scalewaysecrets`;
  - `patch` on their `/status`.
  - No access to Secrets.

## Packaging

- A separate `cmd/fake-operator/Dockerfile` on the same pinned bases, checked by `scripts/check-dockerfile.sh`. The release `Dockerfile` would fail its "third-from" rule with an added target.
- `make build-fake-operator-image` builds `scaleway-manager-fake-operator:local`, never pushed.
- Raw manifests in `tests/cluster/fake-operator/`: namespace `fake-operator`, ServiceAccount, ClusterRole, ClusterRoleBinding, Deployment. They stay outside `charts/`.
- Guard test: `Dockerfile`, `charts/` and the release workflow never mention `fake-operator`.

## kubetest

It mimics the real API server on:

- list of a kind across namespaces;
- get of a namespace and of a cluster-scoped `NamespaceRole`;
- RFC 7386 merge patch on the object and on `/status`, each touching only its own part;
- delete with finalizers: `deletionTimestamp` is set, and the object is removed once its finalizers are empty;
- `resourceVersion` bumped on each write, `409` on a stale one;
- `401` without a bearer token, with every received token recorded.

No watch, selectors or pagination.

## Before the spec

AD-9 is wrong as written: `Pending` and `TerminationBlocked` exist for LoadBalancer only, and `Revoked` and the preconditions are missing. It is corrected right after this forge with `bmad-architecture`, from this file, on the branch of #42.

## Rejected

- Nominal cycle only, with outcomes later.
- AD-9 implemented literally.
- Effect values (`sync_state`) in the annotation.
- Releasing `Syncing` by annotation instead of a delay.
- A watch in #42.
- `internal/kube` reading a ServiceAccount token.
- Any access to Secrets.
- A third `FROM` in the release Dockerfile.
- An event model for causes.
- A `NewVersion` cause.
