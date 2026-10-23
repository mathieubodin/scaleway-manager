# Review: AD-9 reality check

- Target: `ARCHITECTURE-SPINE.md`, AD-9, as changed in the working tree (`git diff HEAD`, 2026-10-23).
- Lens: every committed claim in AD-9 checked against the Kubernetes documentation, the operator code (`/home/mathieu/dev/github.com/scaleway-operator/src`), and the repo files.
- Verdict: the Kubernetes write mechanics and most operator facts hold. The fake's preconditions are incomplete in ways that let tests pass where the real operator writes `Error`. AD-9 also drops the token re-read and the normal deletion path, which the forged idea had stated.

## Findings

### F1 (high): ScalewaySecret preconditions are missing, and the fake cannot see them

AD-9 lists preconditions for `Instance` and `LoadBalancer` only. It gives the fake no access to Secrets. The real operator writes `Error` on a `ScalewaySecret` in these cases:

- The source Secret is missing, has no opt-in label `scaleway.mathieubodin.io/allow-operator-read: "true"`, has no annotation `scaleway.mathieubodin.io/allowed-cr: "<ns>/<cr>"`, or lacks the key. The fixed message is `Source Secret unavailable (see operator logs)`. Sources: `reconcilers.rs:1322-1324`, `1411-1449`, `1755-1803`, `1844-1880`.
- The namespace has no `scaleway.mathieubodin.io/project-id` annotation. This check runs in `CreateAndSyncSecret` after the finalizer is added. Source: `reconcilers.rs:1883-1893`.
- `scaleway-ns-creds-{ns}` is missing from `scaleway-system`. This applies to all three kinds, after the finalizer. Sources: `reconcilers.rs:225-269`, `459`, `1895`.

As written, every `ScalewaySecret` the UI creates reaches `Synced` under the fake. Example: a UI flow that creates the source Secret without the opt-in label passes the tests, but the real operator always writes `Error` for it. This is exactly what AD-9's "Prevents" clause forbids. AD-9 does not say this divergence is accepted.

Options:

- Give the fake metadata-only read access on Secrets (`PartialObjectMetadata`, as AD-14 does for the server) to check the label, annotation and existence.
- Or record the gap explicitly in AD-9.

The `project-id` check needs no Secret access, and the fake should do it.

### F2 (medium): Instance and LoadBalancer preconditions are incomplete, and they differ on the finalizer

The real operator also checks these:

- **`project-id` must be a UUID.** Otherwise the message is `Annotation 'scaleway.mathieubodin.io/project-id' must be a valid UUID, got: '<v>'`, written before the finalizer is added. Sources: `reconcilers.rs:359-371`, `931-941`.
- **The role must allow writes.** It must be `Editor`, `Admin` or `OrganizationOwner` (`role_allows_write`, `reconcilers.rs:149-151`). A read-only role gives `BlockReadOnlyRole`. That check runs after `AddFinalizer` (`decide_next_action`, `reconcilers.rs:121-135`; LB `208-216`), so the finalizer **is** present when that `Error` is written.

The forged idea (`_bmad-output/forge/fake-operator/forged-idea.md:15`) had both checks. The spine dropped them. The forged idea also says "no finalizer" for a role failure, which contradicts the code: the finalizer is added first.

Confirmed:

- The NamespaceRole is looked up first and the annotation second. Sources: `reconcilers.rs:339-389`, `915-951`.
- `NamespaceRole` is cluster-scoped and named after the namespace. Sources: `context.rs:112-150`; `resources.rs`, which has no `#[kube(namespaced)]` on `NamespaceRole`.
- On these failures no finalizer is added.

The messages carry the `Configuration error:` prefix, because `for_status` falls back to `Display` (`error.rs:29`, `324-342`).

### F3 (medium): the token re-read is not stated

AD-9 says the fake "reads its own projected ServiceAccount token", but not when. Kubernetes documentation (Configure Service Accounts for Pods, <https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/>) says:

- The kubelet rotates the token when it is older than 80 % of its TTL or older than 24 h.
- "The application is responsible for reloading the token when it rotates."
- The default lifetime is 1 h (<https://kubernetes.io/docs/concepts/storage/projected-volumes/>).

The forged idea said "at each polling cycle" (`forged-idea.md:42`). The spine dropped it. A fake that reads the token once at startup could get a `401` once the token expires.

Not confirmed: whether kind's default `--service-account-extend-token-expiration` masks this, since it extends default tokens. Do not rely on it.

AD-9 also does not name the mount path (`/var/run/secrets/kubernetes.io/serviceaccount/token`), or say that the fake's Deployment must keep the token mounted. That Deployment cannot use `automountServiceAccountToken: false`, unlike the server (AD-3).

### F4 (medium): deletion behavior is under-specified

The previous AD-9 said "it removes the finalizer on delete". The new text keeps only the `Locked` case. The normal path is only implied, through "Contract: … when the finalizer is added or kept".

Facts checked:

- **Deletion is handled first.** Deletion is decided before every other check, including the circuit breaker and preconditions, and preconditions are skipped while deleting. Sources: `reconcilers.rs:107-113`, `338`, `914`, `1668-1674`. This order also matters on the Kubernetes side. Once `deletionTimestamp` is set, "you can remove existing finalizers … but you cannot add a new finalizer" (<https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/>). A fake that tries step 1 on a terminating object gets a rejected patch. AD-9's Lifecycle does not say that deletion comes first.
- **The `Error` cause on delete does not match the code.** The table maps `Error` to `Error` for every kind, in any phase. When the Scaleway delete fails with a generic error, the real operator returns `Err` without writing status, and the finalizer stays. Sources: Instance `reconcilers.rs:~755-765`, LB `1215-1219`, Secret `~2137-2145`. Only the LB `409`/`423` path writes a status (`TerminationBlocked`).
- **The LB without IAM credentials case is not modeled.** If the IAM Secret is missing at delete time, the LB writes `FinalizerRemovedWithoutScalewayDelete` and removes the finalizer (`reconcilers.rs:1221-1245`). The fake has no Secret access, so it cannot reproduce this. This is acceptable, but unstated.
- **Recovery is unstated.** The forged idea's line "removing [the cause] lets the object recover (`Synced`, or the deletion goes through)" is absent from AD-9.

### F5 (low): `Revoked` is unconditional in the fake, conditional in the operator

The operator writes `Revoked` only when opt-in is withdrawn **and** `scaleway_id` is set (`reconcilers.rs:1781-1795`). Otherwise it writes `Error` with the generic message.

`build_revoked_status` behaves as follows (`reconcilers.rs:1490-1498`):

- It keeps `scaleway_id` and `current_version`.
- It clears `last_synced_resource_version`.
- It sets the fixed message `Source Secret opt-in revoked — Scaleway version disabled. Re-add label/annotation to re-sync.`

AD-9 does not say whether `Revoked` on a never-synced object gives `Revoked` or `Error`.

### F6 (low): ScalewaySecret `Syncing` is near-instant in the real operator

`CreateAndSyncSecret` writes a preliminary `Syncing`, then `Synced`, in the same reconcile (`reconcilers.rs:1966-2027`). Instance and LB hold `Syncing` across a requeue: 10 s, then a 30 s sync. The fake's three-step order is right, but for secrets it holds `Syncing` for a full step delay, which the real operator does not. This is harmless unless a test asserts on timing.

### F7 (low): attribution and cross-AD gaps

- **The AD-3 attribution.** "`internal/kube` reads no token (AD-3)": AD-3 does not contain that sentence. It says every call carries the user's token and that the server pod has `automountServiceAccountToken: false`. The rule is new in AD-9 and attributed to AD-3.
- **Cluster-wide lists.** The fake lists the three kinds across all namespaces. AD-2 says "Namespace and name must pass DNS-1123 validation" for the single URL builder. It does not say how a namespace-less, cluster-wide list URL is built. `NamespaceRole` is cluster-scoped and has the same issue.
- **The RBAC description.** AD-9 says "read-only on `namespaces` and `namespaceroles`". The forged idea names `get`, `list` and a ClusterRoleBinding. AD-9 lists ClusterRole but not the binding.

### F8 (low): what "whole struct" status writes must contain

- **No key renaming, and `None` serializes as `null`.** The status types have no `rename_all` and no `skip_serializing_if` (`resources.rs`). Keys are snake_case (`sync_state`, `error_message`, `scaleway_id`), and `None` serializes as `null`. So AD-9's "`null` removes a key" matches the operator's actual wire behavior.
- **Defaults on a fresh object.** A precondition `Error` on a fresh object is built from `unwrap_or_default()`. Instance: `state: "unknown"`. LB: `state: ""`. The fake must emit these defaults to match byte for byte. AD-9 does not mention them.
- **Operator printer columns (outside AD-9).** The operator's printer columns use camelCase paths (`.status.scalewayId`), which do not match the snake_case keys. This is an operator bug; it does not affect the fake.

## Confirmed claims

| Claim | Evidence |
| --- | --- |
| Finalizer names `…/instance-finalizer`, `…/loadbalancer-finalizer`, `…/scalewaysecret-finalizer` | `reconcilers.rs:19-21` |
| Finalizer is written by merge patch with the whole list | `reconcilers.rs:818-835`, `1274-1286`, `2207-2217` (`Patch::Merge`, `{"metadata":{"finalizers":[…]}}`) |
| JSON merge patch replaces arrays wholesale, and `null` removes a key | RFC 7386 §1, §2, Appendix A (<https://www.rfc-editor.org/rfc/rfc7386>) |
| Status is written by merge patch on `/status`, with the whole struct | `reconcilers.rs:841-857`, `1289-1301`, `2220-2232` (`patch_status`, `Patch::Merge`, `{"status": <struct>}`) |
| Requests to the main resource ignore `status`; `PUT` to `/status` ignores everything else | CRD docs, "Status subresource" (<https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/>) |
| Order: finalizer only (no status), then `Syncing`, then `Synced` | `AddFinalizer` writes no status (`reconcilers.rs:430-433`, `979-982`, `1826-1829`); create writes `Syncing` (`reconcilers.rs:691`, `1159`); sync writes `Synced` (`reconcilers.rs:581`, `1058`, `2026`) |
| Precondition failure writes `Error` and adds no finalizer (Instance, LB) | Precondition checks return before `decide_next_action` (`reconcilers.rs:339-389`, `915-951`) |
| `NotFound`: Instance gets `Syncing` with ids, IP, project and `created_at` cleared; LB gets `Pending` with ids, VIP and project cleared | `reconcilers.rs:586-620`, `1063-1088` (LB `created_at` is never set on create, so nothing to clear) |
| `Locked`, LB only, on delete: `TerminationBlocked`, finalizer kept | `reconcilers.rs:1196-1216` (409 or 423, message `Load balancer is locked — deletion blocked by Scaleway`, early return before finalizer removal) |
| `TerminationBlocked` and `Pending` exist for LB only; `Revoked` for ScalewaySecret only | grep over `reconcilers.rs`, `resources.rs` |
| The operator writes nothing on `NamespaceRole` | `main.rs:81-105`: controllers for Instance, LB and ScalewaySecret only |
| `current_version` is an integer revision | `resources.rs` (`Option<u32>`); `reconcilers.rs:2024`, `2090` |
| The release `Dockerfile` builds only `./cmd/scaleway-manager`; the runtime stage copies only that binary | `Dockerfile` lines 8, 12 |
| `scripts/check-dockerfile.sh` requires exactly two `FROM` lines, so an added fake-operator stage would fail it; a separate Dockerfile is the only way to reuse it | `scripts/check-dockerfile.sh:51-56` |

## Not confirmed

- **Merge patch on `/status` of a custom resource.** The fetched Kubernetes page does not state which patch types `/status` accepts. Support is evidenced only by the operator's production use of `Patch::Merge` with `patch_status`.
- **Repo tooling for the fake.** No Makefile target, script or CI step builds, checks or loads `cmd/fake-operator/Dockerfile` today. `scripts/build-image.sh` checks only the root `Dockerfile`. `tests/cluster/` does not exist yet. AD-9 does not say whether the fake's Dockerfile must pass `scripts/check-dockerfile.sh`; the forged idea does.
- **`internal/kube`.** It does not exist yet (`internal/` holds only `server/`). The claims about its token-passing are design, not reality.
