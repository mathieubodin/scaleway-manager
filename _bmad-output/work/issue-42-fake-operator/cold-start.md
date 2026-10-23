# Cold-start: #42

## Round 1 (`wf_724308f2-deb`)

Red: six stories had blocking items, all about the Go API between stories (in `plan.md`, which a builder does not get) and fine cross-story rules. Fix: the companion `_bmad-output/specs/spec-fake-operator/go-interfaces.md`, added to every story context, settles each blocking item and the useful items below.

### 1-kubernetes-client.md

- Which error wins when several apply (for example an empty token together with an invalid name or a disallowed operation, an unknown Resource together with an empty token, or unset env together with invalid CA PEM in NewInCluster)
- Exact format of `APIError.Error()` (for example `kube: GET /api/v1/namespaces/x: 404`)
- Behavior of `List` when a 2xx body is not valid JSON or has no `items` field: an error (which one?) or an empty slice
- Whether the client sets an HTTP timeout or a response-body size limit, or relies only on ctx
- Whether MergePatch/PatchStatus check that the patch is valid JSON or contains `metadata.resourceVersion`, or send it unchanged

### 2-fake-api-server.md

- The body of error responses (401, 404, 409, 415, injected faults): a Kubernetes `Status` object (`kind: Status`, `reason`, `code`) or an empty or plain body.
- Whether the HTTP DELETE verb is served, or only the Go method `Server.Delete`.
- `resourceVersion` scope and seed: one global counter (like etcd) or one per object; its initial value; whether `Put` and `Delete` (setting deletionTimestamp) bump it; whether `Put` replacing an object keeps or resets `generation`.
- Rules for `Fail`, `Requests()` and `Tokens()`: is `pathPrefix` matched against the URL path or the RequestURI? Does an empty method match any method? Is a fault applied before or after the token check? Are rejected requests (401, 415, faults) recorded in `Requests()`? Is an absent token recorded in `Tokens()`?
- Behavior of a merge patch on `/status` or on the object when the target is absent, or when the patch changes `metadata.name`, `metadata.namespace` or `metadata.resourceVersion` itself. Also: is the stored `resourceVersion` taken from the patch or from the server?
- Whether `Put` of a namespaced resource without `metadata.namespace` (or of a cluster-scoped one with it) panics, calls `t.Fatal`, or is accepted.

### 3-fake-operator-engine.md

- Rule that picks the lifecycle step from the status read (for example: no scaleway_id means creation, sync_state Syncing with scaleway_id means the next step, Synced means steady or a version push), and the behavior for other sync_state values (Error, Pending, Revoked) in story 3
- created_at format precision: time.RFC3339 or time.RFC3339Nano
- Whether the version push (generation != observed_generation) applies only when sync_state is Synced or in any state, and the JSON type to write for observed_generation and current_version (number taken from the json.Number)
- Behavior of the deletion stub when deletionTimestamp is set and the kind's finalizer is already absent (return handled with no patch?), and whether it keeps other finalizers in the list
- math/rand/v2 source and seed for main.go (PCG or ChaCha8; seed from time or crypto/rand)
- slog message texts and attribute keys for the logged errors (per-object failure, failed List)

### 4-deletion-and-operator-checks.md

- The Go types of `k` and `obj` in story 3's signatures, how the kind of `obj` is identified (a field, a kind string, the plural), and what the error return means. For example, is a failed status or finalizer patch returned as err, and does the engine then stop the pass?
- The name, signature and behavior of story 3's status-patch helper (whole struct, nulls for absent values, Rust defaults when there is no status, no patch when unchanged), plus any finalizer-patch helper and the finalizer name constants.
- The kubetest API from story 2: the signatures of `Put` (namespace, NamespaceRole, CR objects), `Delete` and `Fail` (how a fault targets a method and path, so a namespace GET can fail without failing the NamespaceRole GET), and how to read the request log to assert that no status PATCH was sent.
- The UUID formats accepted for the project-id annotation: only the canonical 8-4-4-4-12 hyphenated form, or also the forms Rust's Uuid::parse_str accepts (simple, braced, urn:uuid:), and upper case.
- What happens when `spec.scaleway_role` is missing or empty in the NamespaceRole: treat it as read-only with ``<r>`` empty, or something else.

### 5-causes-by-annotation.md

- What counts as a 'synced ScalewaySecret' for `source-version` and for the `Error` no-op: `sync_state == "Synced"`, or `scaleway_id` non-null (the spec uses 'while scaleway_id is null' for Error)? It also does not say what happens to a secret in `Error` or `Revoked` state that has a `scaleway_id`.
- Where invalid `source-version` sits in the precedence: is it reported on a new secret before the engine's `current_version: 1` write, when a cause is also present, and on Instance/LoadBalancer (ignored or refused)? Also, which comes first when both `source-version` and a `metadata.generation` change are pending?
- Whether story 3's lifecycle starts a creation whenever `status.scaleway_id` is null, whatever the `sync_state` and `state` are, so that removing `NotFound` re-creates the object (`Syncing` with a new id, then `Synced`).
- Whether the `message` annotation also replaces `Source Secret unavailable (see operator logs)` in the Revoked-without-`scaleway_id` branch, or only the Revoked-with-`scaleway_id` message.
- Names of the annotation-key and message constants that story 3 or story 4 may already define (for example the `fake.scaleway-manager.test/` prefix, or the `Error` message reused by deletion in story 4).

### 6-runtime.md

- Whether a pass that fails before listing (e.g. token file unreadable) counts as an 'ended pass' for `/healthz`.
- Whether the first pass starts immediately after start-up or after one delay.
- Timeout for the health server `Shutdown` and the `ReadHeaderTimeout` value.
- `/healthz` details beyond status: body of the 503 response, behavior for non-GET methods and other paths.
- Log message texts and attribute keys for the start-up ERROR lines (no host/port, bad delay, CA unreadable, guard hit, list error, bind failure) and for a logged pass error; in particular the key that 'names' the real-operator Deployment.
- Whether `run` must also handle a second signal (drain and ignore, as in cmd/scaleway-manager) and SIGINT, and what main.go (story 3) already registers on `sigs`.

### 7-packaging-and-isolation.md

- The metadata names of the ServiceAccount, ClusterRole, ClusterRoleBinding, Role, RoleBinding and Deployment, and the Deployment's labels and selector.
- Where the new CI step goes in the `build-image` job: right after `make build-image`, or after the existing "Run the image tests" step.
- A note that the new JSON manifests must pass `biome ci .`, which runs inside `make check` (via `lint-workflows`, 2-space indent, `**/*.json` included).
