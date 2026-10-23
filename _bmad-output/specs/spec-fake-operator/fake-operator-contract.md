# Fake operator contract

The behavior `cmd/fake-operator` must reproduce, per kind. Source: `scaleway-operator-v0.1.15` (`src/reconcilers.rs`, `src/resources.rs`, `src/error.rs`, `src/context.rs`, `src/scaleway.rs`), CRDs `scaleway-operator-crds` `0.1.14`. Read the operator only at that tag. AD-9 of the architecture spine is the rule; this file is its detail.

## Kinds

| Kind | Group/version | Plural | Scope | Finalizer |
| --- | --- | --- | --- | --- |
| `Instance` | `scaleway.mathieubodin.io/v1` | `instances` | Namespaced | `scaleway.mathieubodin.io/instance-finalizer` |
| `LoadBalancer` | `scaleway.mathieubodin.io/v1` | `loadbalancers` | Namespaced | `scaleway.mathieubodin.io/loadbalancer-finalizer` |
| `ScalewaySecret` | `scaleway.mathieubodin.io/v1` | `scalewaysecrets` | Namespaced | `scaleway.mathieubodin.io/secret-finalizer` |
| `NamespaceRole` | `scaleway.mathieubodin.io/v1` | `namespaceroles` | Cluster | none; never written |

## Status fields

JSON keys are snake_case. Every write sends the whole struct; an absent value is `null`, which the merge patch deletes.

| Kind | Fields | Rust default (written with a first `Error`) |
| --- | --- | --- |
| `Instance` | `scaleway_id`, `public_ip`, `state`, `project_id`, `created_at`, `error_message`, `sync_state` | `state: "unknown"`, `sync_state: "Syncing"` |
| `LoadBalancer` | `scaleway_id`, `vip_address`, `state`, `project_id`, `created_at` (never set), `error_message`, `sync_state` | `state: ""`, `sync_state: "Pending"` |
| `ScalewaySecret` | `scaleway_id`, `current_version`, `last_synced_resource_version` (never written by the fake), `observed_generation`, `sync_state`, `error_message` | `sync_state: "Syncing"` |

## Pass order

One pass moves an object at most one step. The fake evaluates, in this order:

1. **ScalewaySecret region check:** runs first, deletion included (see Checks).
2. **Deletion** (`deletionTimestamp` set). Every other check is skipped.
   - Under `Error`, with a `scaleway_id`: the finalizer is kept and no status is written.
   - Under `Locked` (LoadBalancer only), with a `scaleway_id`: `TerminationBlocked` is written and the finalizer is kept.
   - Otherwise: the finalizer is removed. Without a `scaleway_id` the operator calls no Scaleway API on delete, so no cause blocks the deletion. A refused cause never blocks deletion.
3. **Checks before the finalizer** (see Checks). A failure writes `Error` and adds no finalizer.
4. **`Revoked` before the finalizer:** a ScalewaySecret under `Revoked` with no `scaleway_id` gets the `Error` of the Causes table and no finalizer.
5. **Finalizer:** if missing, it is added and nothing else happens this pass.
6. **Checks after the finalizer.**
7. **Spec lists.** A failure writes no status.
8. **Cause or lifecycle step** (see Causes).

## Lifecycle values

| Step | `Instance` | `LoadBalancer` | `ScalewaySecret` |
| --- | --- | --- | --- |
| Creation pass | `Syncing`, `state: "creating"`, `scaleway_id` (random UUID), `project_id` (namespace annotation), `created_at` (now), `error_message: null` | `Syncing`, `state: "pending"`, `scaleway_id`, `project_id`, `error_message: null` | `Syncing` then `Synced` in the same pass, as two patches. The first carries `scaleway_id`, with `current_version` and `last_synced_resource_version` null. The second carries `current_version: 1` and `observed_generation` |
| Next pass | `Synced`, `state: "running"`, `public_ip: 192.0.2.<x>`, `error_message: null` | `Synced`, `state: "ready"`, `vip_address: 198.51.100.<x>`, `error_message: null` | steady: no write |

- Generated values (`scaleway_id`, IPs) are kept once written.
- A ScalewaySecret with a `scaleway_id` whose `metadata.generation` differs from `status.observed_generation` pushes a version: `current_version` + 1, `observed_generation` updated, `Synced`. This holds whatever its `sync_state`, except `Revoked`.

## Checks

Each check uses the operator's message, with `<ns>`, `<r>` and `<pid>` filled in.

| Kind | Order | Check | Before or after finalizer | Message |
| --- | --- | --- | --- | --- |
| `ScalewaySecret` | first, deletion included | `spec.region` in `fr-par`, `nl-ams`, `pl-waw` | before; a failure on delete keeps the finalizer | `Configuration error: Invalid region '<r>'. Use one of: fr-par, nl-ams, pl-waw.` |
| all three | 1 | `NamespaceRole` named `<ns>` exists | before | `Configuration error: No NamespaceRole found for namespace '<ns>'. Create a NamespaceRole resource with name '<ns>' to assign a Scaleway role.`; any other read error: `Kubernetes API error` |
| `ScalewaySecret` | 2 | role is `Editor`, `Admin` or `OrganizationOwner` | before | `Configuration error: Role '<r>' is read-only and cannot sync secrets. Use 'Editor' or 'Admin'.` |
| `Instance`, `LoadBalancer` | 2 | namespace annotation `scaleway.mathieubodin.io/project-id` present | before | `Configuration error: Namespace '<ns>' must have annotation 'scaleway.mathieubodin.io/project-id'`; namespace read error: `Configuration error: Cannot access namespace <ns>: <error>` |
| `Instance`, `LoadBalancer` | 3 | annotation is a UUID | before | `Configuration error: Annotation 'scaleway.mathieubodin.io/project-id' must be a valid UUID, got: '<pid>'` |
| `Instance`, `LoadBalancer` | 4 | role is `Editor`, `Admin` or `OrganizationOwner`, while `scaleway_id` is null | after | `Configuration error: Role '<r>' is read-only and cannot create instances. Use 'Editor' or 'Admin'.` (`load balancers` for LoadBalancer) |
| `ScalewaySecret` | 3 | `spec.source.kubernetes_secret` set | after | `Secret source not configured: spec.source must have kubernetes_secret configured` |
| `ScalewaySecret` | 4 | `project-id` annotation present (no UUID check), at creation | after | same message as `Instance` |
| `Instance` | spec list | `spec.zone` in `fr-par-1`, `fr-par-2`, `nl-ams-1`, `pl-waw-1`, `sg-sin-1`, `it-mil-1`; `spec` instance type in the operator's list | after | no status write |
| `LoadBalancer` | spec list | `spec.zone` as above; `lb_type` in `LB-S`, `LB-GP` | after | no status write |

The exact field names of the spec lists and the instance-type list are read from `src/scaleway.rs` at the tag.

**Not mimicked** (they need Secret reads): the source Secret, its opt-in label and annotation, its key, `scaleway-ns-creds-<ns>`, `FinalizerRemovedWithoutScalewayDelete`, and the circuit breaker.

## Causes

The annotation `fake.scaleway-manager.test/outcome` applies on every pass while it is present. A cause acts at the step where the operator would call Scaleway, after the checks.

| Cause | `Instance` | `LoadBalancer` | `ScalewaySecret` |
| --- | --- | --- | --- |
| `NotFound` | `Syncing`, `state: "unknown"`; `scaleway_id`, `public_ip`, `project_id`, `created_at`, `error_message` null. Re-created once removed | `Pending`, `state: ""`; `scaleway_id`, `vip_address`, `project_id`, `error_message` null. Re-created once removed | refused |
| `Error` | `Error`, `Scaleway API error: 500 Internal Server Error`, other fields kept; on delete: finalizer kept with a `scaleway_id`, removed without | same | at creation only, same message; a synced secret is unchanged; on delete as `Instance` |
| `Locked` | refused | on delete with a `scaleway_id`: `TerminationBlocked`, `Load balancer is locked — deletion blocked by Scaleway`, finalizer kept; on delete without `scaleway_id`: finalizer removed; outside deletion: no effect | refused |
| `Revoked` | refused | refused | with `scaleway_id`: `Revoked`, `Source Secret opt-in revoked — Scaleway version disabled. Re-add label/annotation to re-sync.`, `scaleway_id` and `current_version` kept, `last_synced_resource_version` null; without `scaleway_id`: `Error`, `Source Secret unavailable (see operator logs)`, no finalizer (checked before it) |

Other annotations and values:

- **Refused cause:** `Error` with `fake-operator: cause '<v>' is not supported for <Kind>`.
- **Empty `outcome`:** treated as absent.
- **`fake.scaleway-manager.test/message`:** replaces the message of a cause that writes one (`Error`, `Locked`, `Revoked`). It is ignored otherwise: no cause, `NotFound`, or a refused cause.
- **`fake.scaleway-manager.test/source-version: "<n>"`**, without a cause, on a synced ScalewaySecret: sets `current_version` to `n` and writes `Synced`.
  - On a new secret, the fake writes `current_version: 1` first, then `n` on the next pass.
  - A value that is not a positive integer gives `Error` with `fake-operator: source-version '<v>' is not a positive integer`.

## Runtime

- **Polling:** the delay is `FAKE_OPERATOR_STEP_DELAY` (Go duration, default `2s`; an invalid or non-positive value exits 1 at start). The next pass starts one delay after the previous one ends. A failed list skips only that kind.
- **Token:** read at each pass from `/var/run/secrets/kubernetes.io/serviceaccount/token`. The CA comes from `ca.crt` in the same directory, an explicit projected volume of the token and `kube-root-ca.crt`. `main` hardcodes these two paths and passes them to `run`, so tests inject their own.
- **Exit 1 at start:**
  - when `KUBERNETES_SERVICE_HOST` or `KUBERNETES_SERVICE_PORT` is missing;
  - when a Deployment labelled `app.kubernetes.io/name: scaleway-operator` exists in `scaleway-system`.
- **`GET /healthz`** on `HEALTH_ADDR` (default `:8081`): 503 until the first pass ends; then 200 while the last pass ended less than 3 × the delay ago, 503 otherwise. A failed list still counts as an ended pass. It is used by a liveness probe.
- **SIGTERM or SIGINT:** the pass stops between two objects: the in-flight object finishes, the next one is not started. Then `run` returns 0. A request is never canceled by the signal; each pass stays bounded by 30 s.

## Writes

- **Finalizer:** JSON merge patch on the object, `{"metadata":{"finalizers":[<whole list>]}}`.
- **Status:** JSON merge patch on `/status`, `{"status":{<whole struct>}}`.
- **Patch rules:**
  - No `resourceVersion` in either patch.
  - A patch is sent only when the result differs from the object read.
- **Never written:** `NamespaceRole`, annotations, labels, events.

## Packaging and RBAC

- **Image:** `cmd/fake-operator/Dockerfile`, on the release bases, passes `scripts/check-dockerfile.sh`. `make build-fake-operator-image` builds `scaleway-manager-fake-operator:local`, which is never pushed.
- **Manifests:** JSON files in `tests/cluster/fake-operator/`, `imagePullPolicy: Never`.
  - Namespace `fake-operator`.
  - ServiceAccount.
  - ClusterRole and ClusterRoleBinding: `get` and `list` on `namespaces` and `namespaceroles`; `list` and `patch` on `instances`, `loadbalancers` and `scalewaysecrets`; `patch` only on their `/status`.
  - Role and RoleBinding in `scaleway-system`: `list` on `deployments`.
  - Deployment with `automountServiceAccountToken: false`, the projected volume and a liveness probe on `/healthz`.
- **No access to Secrets.**
- **Guard test** in `tests/make`: the release `Dockerfile`, `charts/` and the release workflow never mention `fake-operator`.

### RBAC rules

| Object | `apiGroups` | `resources` | `verbs` |
| --- | --- | --- | --- |
| ClusterRole | `""` | `namespaces` | `get`, `list` |
| ClusterRole | `scaleway.mathieubodin.io` | `namespaceroles` | `get`, `list` |
| ClusterRole | `scaleway.mathieubodin.io` | `instances`, `loadbalancers`, `scalewaysecrets` | `list`, `patch` |
| ClusterRole | `scaleway.mathieubodin.io` | `instances/status`, `loadbalancers/status`, `scalewaysecrets/status` | `patch` |
| Role in `scaleway-system` | `apps` | `deployments` | `list` |

No other rule, no `secrets`, no `*`.

## kubetest

It behaves like the API server on:

- list of a kind across namespaces;
- get of a namespace and of a cluster-scoped `NamespaceRole`;
- list of deployments in a namespace;
- RFC 7386 merge patch on the object and on `/status`, each touching only its own part;
- delete with finalizers, which sets `deletionTimestamp` and removes the object once its finalizers are empty;
- `resourceVersion` bumped on each change, kept on a patch that changes nothing, `409` on a stale one;
- `401` without a bearer token, every received token recorded;
- `415` on a content type other than `application/merge-patch+json` for a patch.

It also injects faults per method and path. Each behavior's test cites the Kubernetes documentation URL it reproduces. Out of scope: watch, selectors, pagination.
