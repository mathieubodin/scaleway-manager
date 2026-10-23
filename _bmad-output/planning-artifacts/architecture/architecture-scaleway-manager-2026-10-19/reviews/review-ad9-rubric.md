# Rubric review: AD-9 (fake operator) and its seams

- Spine: `ARCHITECTURE-SPINE.md`, working tree vs `HEAD` (AD-9 rewritten, Structural Seed `tests/cluster/fake-operator/` added).
- Contract checked: scaleway-operator `d6e721f`, `src/reconcilers.rs` (lines 1-2271, non-test), `src/resources.rs`, `src/context.rs` (`get_scaleway_role_for_namespace`), `src/error.rs` (`for_status`, `#[error]` formats).
- Decided input: `_bmad-output/forge/fake-operator/forged-idea.md`.
- Existing code: `Dockerfile`, `scripts/check-dockerfile.sh`, `Makefile`.

## Verdict

AD-9 now gets the per-kind cause table right (`Pending` and `TerminationBlocked` are LoadBalancer only, `Revoked` is ScalewaySecret only, Instance NotFound gives `Syncing`). It still leaves out operator preconditions that change the observable outcome, says nothing about the Secret-dependent behavior the fake cannot see, and leaves the timing of causes open to more than one reading. Fix these before the spec is written.

## What was verified as correct

| AD-9 claim | Operator evidence |
| --- | --- |
| Order: finalizer only, then `Syncing`, then `Synced` | `decide_next_action` step 3 `AddFinalizer` (requeue 5 s), `execute_create_instance` writes `Syncing` + `state: creating` (l. 688-706), `SyncInstance` writes `Synced` (l. 578-584); same for LB (l. 979-982, 1157-1171, 1055-1061); ScalewaySecret writes `Syncing` then `Synced` within one reconcile (l. 1964-2028) |
| Precondition failure: `Error`, no finalizer (Instance, LB; missing NamespaceRole or annotation) | Role and project-id are read before `decide_next_action`, and the function returns before `AddFinalizer` (l. 339-400, 916-957) |
| Instance `NotFound`: `Syncing`, ids cleared | l. 606-612 (`scaleway_id`, `public_ip`, `project_id`, `created_at` cleared, `state: unknown`) |
| LB `NotFound`: `Pending`, ids cleared | l. 1075-1080 |
| LB `Locked`, on delete: `TerminationBlocked`, finalizer kept | l. 1198-1214 (409/423 on delete, returns before finalizer removal) |
| `Revoked` is ScalewaySecret only | `build_revoked_status`, l. 1490-1498 |
| Finalizer: merge patch with the whole list; status: merge patch on `/status` with the whole struct, `null` removes | `add_finalizer` l. 821-839; `update_status` l. 841-858; status structs serialize `None` as `null` (no `skip_serializing_if`) |
| Snake_case status field names | `resources.rs`: no `rename_all`, so `sync_state`, `error_message`, `current_version` |
| Nothing written on NamespaceRole | No reconciler for NamespaceRole in `reconcilers.rs` |
| Separate Dockerfile is compatible with `scripts/check-dockerfile.sh` | The script takes a file argument; its build-stage regex and cross-compile line do not name a package, so `./cmd/fake-operator` passes. The release `Dockerfile` builds only `./cmd/scaleway-manager`, so the fake binary never enters the release image |
| Export strips the fake's annotations | AD-5 drops `fake.scaleway-manager.test/` annotations |

## Findings

### F1 [high] The preconditions are incomplete, so tests pass on outcomes the operator never produces

Location: AD-9 Rule, **Preconditions**.

The operator has more preconditions than AD-9 lists, and some apply after the finalizer is added:

1. **Role value.** For Instance and LB, `role_allows_write` accepts only `Editor`, `Admin` or `OrganizationOwner`. It runs at decision step 5, after `AddFinalizer` (l. 121-137, 202-218). The operator writes `Error` with `Configuration error: Role '<r>' is read-only and cannot create instances. Use 'Editor' or 'Admin'.` (or `load balancers`), and the finalizer stays. AD-9 checks only that the `NamespaceRole` exists. A `Viewer` role therefore gives `Synced` in the fake and `Error` with the operator. The forged idea did require the role check, but it put the failure before the finalizer ("no finalizer"), which is also wrong against l. 121-137. The AD should state the check and where it sits in the order.
2. **UUID.** The operator requires the annotation to parse as a UUID and writes its own message otherwise (l. 362-372). AD-9 says only that the annotation is checked. The forged idea says "(UUID)". This is partly covered because AD-6 already defines a managed namespace by a UUID annotation.
3. **ScalewaySecret project-id.** For ScalewaySecret, the operator reads the project-id annotation in `CreateAndSyncSecret`, after the finalizer, and writes `Error` with the message from `get_project_id_from_namespace_resource` (l. 1884-1893). It checks no NamespaceRole for this kind. AD-9 limits preconditions to Instance and LB, so a ScalewaySecret in a namespace with no annotation reaches `Synced` in the fake.
4. **Order and messages.** The NamespaceRole is checked before the annotation (l. 341 then 359), and each failure has its own message (`context.rs` l. 136-139, `reconcilers.rs` l. 292-295, all prefixed `Configuration error:` by `for_status`). "The operator's message" does not say which message wins when both preconditions fail. The AD should state that the NamespaceRole is checked first.
5. **Deletion skips preconditions.** On deletion the operator reads neither the role nor the annotation (l. 339, 916). AD-9 does not say whether the fake checks preconditions while an object is being deleted.

### F2 [high] What the fake cannot see because it has no Secret access is not stated

Location: AD-9 Rule, **Identity** ("It has no access to Secrets") vs **Prevents** ("tests passing against behavior the real operator never has").

Without Secret access, the fake cannot reproduce operator outcomes that depend on Secrets:

- **ScalewaySecret source Secret.** The operator writes `Error` (`Source Secret unavailable (see operator logs)`) in these cases:
  - the source Secret is missing (l. 1844-1860);
  - its key is missing (l. 1862-1879);
  - it lacks the label `scaleway.mathieubodin.io/allow-operator-read: "true"` or the annotation `scaleway.mathieubodin.io/allowed-cr: "<ns>/<name>"` (l. 1418-1429, 1796).

  The source Secret's `resourceVersion` drives the new-version logic (l. 1701). In the fake, a ScalewaySecret reaches `Synced` with no source Secret at all.
- **Namespace credentials.** For Instance, the operator writes `Error` when `scaleway-ns-creds-<ns>` is missing from `scaleway-system` or has no `secret_key` (l. 459-470, 501-512, 539-550). For LB, the same missing Secret on delete gives `FinalizerRemovedWithoutScalewayDelete` (l. 1222-1244).

The UI writes these Secrets: AD-12 `scaleway-manager-credentials`, AD-5 `required-secrets`, and AD-14 create and replace. A Gherkin scenario can therefore pass even when the UI creates a source Secret without the opt-in label and annotation, or never creates the credentials Secret. The forged idea rejected Secret access on purpose, and that is the human's decision. AD-9 should still state this gap explicitly, for example as "not mimicked: Secret-dependent outcomes; covered by `@manual` scenarios" or by `Error` causes. Otherwise its Prevents claim is false for these features.

One more fact for that decision: RBAC cannot restrict a role to metadata-only reads of Secrets. A `PartialObjectMetadata` read, which AD-14 uses, is the client's choice, not a permission boundary.

### F3 [medium] When causes apply is left open, so two builders would write different fakes

Location: AD-9 Rule, **Causes** ("on every pass while the annotation is present, with no memory").

- **Phase.** The rule does not say whether a cause applies before the finalizer step, before the preconditions, or during deletion. With the operator:
  - a Scaleway `Error` only happens after the finalizer is added (`execute_create_*`, `Sync*`);
  - an `Error` on delete writes no status, and only the finalizer stays (l. 757-765, 1215-1219);
  - a `Locked` cause outside deletion has no effect.

  Read literally, "every pass" lets the fake write `Error` on an object that has no finalizer yet. For Instance and LB, the operator only does that when a precondition fails.
- **NotFound with no memory.** In the operator, NotFound happens only while it syncs an object that has a `scaleway_id`. Once the ids are cleared, the next pass creates the object again (adopt or create, then `Syncing` with new ids), and the pass after that hits NotFound again. While the annotation stays, the observable state therefore cycles: `Syncing` (ids cleared), `Syncing` (new ids) for Instance; `Pending`, `Syncing` for LB. "Then re-created" combined with "no memory" fits either that cycle or a stable cleared state. The AD must pick one.
- **Revoked before a first sync.** The operator writes `Revoked` only when a `scaleway_id` exists (l. 1781-1795). Without one, it writes `Error`.
- **Messages.** The operator has fixed messages:
  - `TerminationBlocked`: `Load balancer is locked — deletion blocked by Scaleway` (l. 1204-1207);
  - `Revoked`: `STATUS_REVOKED_GENERIC` (l. 1329-1330).

  AD-9 also copies `fake.scaleway-manager.test/message` into `error_message`, and gives a refused cause its own message. It does not say which message wins. The Contract names "messages", but the cause table carries none.

### F4 [medium] The packaging rules from the forged idea did not land, so "never in the release" has no check

Location: AD-9 Rule, **Where it runs**; AD-11 ("Deliver" workflow).

These forged decisions are missing from AD-9:

- `cmd/fake-operator/Dockerfile` uses the same pinned bases and is checked by `scripts/check-dockerfile.sh`;
- `make build-fake-operator-image` builds `scaleway-manager-fake-operator:local`;
- a guard test fails if `Dockerfile`, `charts/` or the release workflow mentions `fake-operator`.

As written, "never in the release image, `Dockerfile` or chart" has no named check. It also leaves out the "Deliver" workflow of AD-11. Without the same-bases rule, the golang and distroless digests in the two Dockerfiles can drift apart.

The `Makefile` has no target for the fake today (it has `build-image` only), so the spine has to fix that name.

The manifest list also stops at "namespace, ServiceAccount and ClusterRole". The forged idea also lists the ClusterRoleBinding and the Deployment, and AD-9 itself refers to "its Deployment".

### F5 [low] The seams with AD-2, AD-3, AD-4 and the diagrams are not updated

Locations:

- **AD-3 and the paradigm flowchart.** AD-3 opens with "Every Kubernetes call carries the logged-in user's token", and the edge from `kube` to the API is labelled "HTTPS, user token". The fake sends a ServiceAccount token through `internal/kube`, and neither place mentions that exception. AD-9 also cites AD-3 for "`internal/kube` reads no token", but AD-3 does not say this. The forged idea does, and AD-3 should carry it.
- **AD-4.** "Every update is a JSON merge patch … plus `metadata.resourceVersion`". The operator's finalizer and status patches carry no `resourceVersion` (l. 825-835, 846-854), and AD-9 does not say whether the fake's patches carry one. If `internal/kube` adds the version to every merge patch, the fake gets `409` responses that the operator never gets.
- **AD-2.** The CA comes from the mounted `kube-root-ca.crt` ConfigMap. AD-9 does not say how the fake's Deployment gets the CA, or whether it mounts the token automatically or through an explicit projected volume. The server pod has `automountServiceAccountToken: false`.
- **Deployment diagram.** The edge reads `op -->|"watch, status"|`, but the fake polls and never watches. This is the forged decision "A watch in #42" rejected.
- **RBAC wording.** "Lists and patches the three other kinds and their `/status`": the status subresource needs `patch` only. The forged idea gives exact verbs (`get`, `list` on namespaces and namespaceroles), which are clearer than "read-only".

## Forged-idea items that did not land

| Forged item | In AD-9? | Note |
| --- | --- | --- |
| NamespaceRole role must be `Editor`, `Admin` or `OrganizationOwner` | No | F1; the forged "no finalizer" is wrong for this case |
| Project-id annotation must be a UUID | No | F1 |
| Status values: `scaleway_id` random UUID, `project_id` copied, `state` values, RFC 5737 IPs, `created_at` for Instance only, `current_version` starting at 1, `last_synced_resource_version` never written | No | Mostly material for the spec; acceptable to defer, but the AD should name where they are decided |
| Removing the annotation lets the object recover | Implied by "no memory" | Acceptable |
| `source-version`: writes `current_version: n` **and `Synced`** when it differs | Partly; `Synced` dropped | The operator's `PushNewVersion` writes `Synced` directly (l. 2089-2094) |
| ClusterRole exact verbs | Paraphrased | F5 |
| Same pinned bases, `check-dockerfile.sh`, make target, image name, guard test | No | F4 |
| Manifests include ClusterRoleBinding and Deployment | No | F4 |
| `kubetest` capabilities (finalizer deletion, `409`, `401`, recorded tokens) | Not in AD-9 | AD-15 owns `kubetest`; acceptable |
| Out of scope: deploying to the kind cluster is a separate issue | Not stated | The Environments table names `make` targets #14 and #15; consistent |

## Out of scope but noticed

- The Structural Seed omits existing directories: `cmd/check-go-mod/`, `tests/image/`, `tests/make/`, `tests/workflows/`. Only the `tests/cluster/` line was in scope.
