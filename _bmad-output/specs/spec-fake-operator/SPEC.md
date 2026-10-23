---
id: SPEC-fake-operator
companions:
  - fake-operator-contract.md
  - ../../planning-artifacts/architecture/architecture-scaleway-manager-2026-10-19/ARCHITECTURE-SPINE.md
sources:
  - ../../forge/fake-operator/forged-idea.md
  - ../../work/issue-42-fake-operator/define-notes.md
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Fake operator (#42)

## Why

This work exists for two reasons: it unblocks other work, and it meets a mandate.

- **Blocker:** automated tests of scaleway-manager need custom resources whose status moves without Scaleway. #53 (automated-test kind cluster) and #15 (behavior diff) depend on it.
- **Mandate:** AD-9 requires a test-only fake that mimics `scaleway-operator-v0.1.15` exactly, so that no test passes against behavior the real operator never has.
- **Prerequisite:** the work also creates `internal/kube`, the only Kubernetes client (AD-2), which the server will reuse.

## Capabilities

- **CAP-1**
  - **intent:** `internal/kube` sends Kubernetes API requests with a caller-supplied token, through one URL builder over the AD-2 closed list.
  - **success:** `make test-go` shows:
    - each AD-2 closed-list entry except the readiness endpoints builds the exact expected URL of a table test (for example `/apis/<group>/<version>/namespaces/<ns>/<plural>/<name>`, `/api/v1/namespaces/<name>`);
    - a namespace failing the DNS-1123 label rule, or a resource name failing the DNS-1123 subdomain rule, is refused before any request;
    - a name carrying `/` or `%2F` is refused before any request;
    - list, get, merge patch and status patch each send `Authorization: Bearer <token>`, and only the two patches send `Content-Type: application/merge-patch+json`;
    - an empty token returns an error and sends nothing;
    - the address comes from `KUBERNETES_SERVICE_HOST` and `KUBERNETES_SERVICE_PORT`;
    - requests trust only the CA of the `ca.crt` file passed in, and a server certificate from another CA is refused;
    - `kube.Token` is redacted by `String`, `GoString`, `LogValue` and `MarshalJSON`.
- **CAP-2**
  - **intent:** `internal/kube/kubetest` stands in for the API server on every behavior the fake relies on.
  - **success:** `make test-go` has one test per sub-behavior of the companion's "kubetest" section, each with a comment citing the Kubernetes documentation URL it reproduces:
    - list across namespaces;
    - get of a namespace;
    - get of a NamespaceRole;
    - list of deployments;
    - merge patch on the object;
    - merge patch on `/status`;
    - delete with finalizers;
    - `resourceVersion` bumped;
    - `resourceVersion` kept on a no-op patch;
    - `409` on a stale `resourceVersion`;
    - `401` without a token;
    - token recording;
    - `415`.

    Fault injection has one table test, with no URL required.
- **CAP-3**
  - **intent:** A new Instance, LoadBalancer or ScalewaySecret goes through the operator's steps, one per pass, with the operator's status values.
  - **success:** Go tests run passes one at a time (injected clock and random source, no sleep) against kubetest. After each pass, they compare the whole status with the companion's "Lifecycle values", for each kind:
    - the finalizer alone first;
    - then `Syncing`, then `Synced` (Instance and LoadBalancer);
    - a ScalewaySecret creation pass sends the `Syncing` patch then the `Synced` patch, seen in order in the kubetest request log, and the status after that pass is `Synced`;
    - over three passes after `Synced`, the generated fields of each kind stay unchanged: `scaleway_id`, `public_ip` and `created_at` for Instance, `scaleway_id` and `vip_address` for LoadBalancer, `scaleway_id` for ScalewaySecret;
    - a ScalewaySecret spec change raises `current_version` by exactly 1.
- **CAP-4**
  - **intent:** Deleting an object releases it the way the operator would, or holds it where the operator would.
  - **success:** Go tests show, on an object with `deletionTimestamp`, for each kind each case applies to:
    - **No `outcome` (all three kinds):** the finalizer is removed. The namespace has neither NamespaceRole nor `project-id`, which shows that no row of the companion's "Checks" runs except the ScalewaySecret region. The ScalewaySecret of this case has a valid region.
    - **`outcome: Error` (all three kinds):** the finalizer is kept and no status patch is sent.
    - **`outcome: Locked` on a LoadBalancer:** `sync_state: TerminationBlocked` is written with the operator's fixed message, and the finalizer is kept.
    - **A refused `outcome`:** the finalizer is removed and no status patch is sent. The test uses `Locked` on Instance, `NotFound` on ScalewaySecret and `Revoked` on LoadBalancer.
    - **A ScalewaySecret with an invalid region and no `outcome`:** it keeps its finalizer and gets the region `Error`.
- **CAP-5**
  - **intent:** The fake reproduces the operator's checks it can read (spec, namespace, NamespaceRole), in the operator's order and with its messages.
  - **success:** Go tests include one test per message of the companion's "Checks" table, for each kind the row applies to.
    - Secondary messages are included:
      - `Kubernetes API error`;
      - `Cannot access namespace`, for Instance, LoadBalancer and ScalewaySecret. The test fixes the error in kubetest and asserts the exact prefix `Configuration error: Cannot access namespace <ns>:` plus one space, followed by non-empty text.
    - Each test asserts the exact message, and whether the finalizer is present: a row marked "before" leaves it absent on a new object, and a row marked "after" leaves it present.
    - `Kubernetes API error` is tested on each of the three kinds.
    - The ScalewaySecret region is also tested on delete: finalizer kept, region message.
    - Each spec-list field is tested on the kinds that have it (zone on Instance and LoadBalancer, instance type on Instance, `lb_type` on LoadBalancer). The test asserts that no status is written and that the finalizer stays.
- **CAP-6**
  - **intent:** A test forces a Scaleway-side cause through an annotation and sees the effect the operator writes for that kind.
  - **success:** every cell of the companion's "Causes" table has its own Go test. A cell with two branches has one test per branch:
    - `Revoked` with and without `scaleway_id`;
    - ScalewaySecret `Error` at creation and once synced;
    - `NotFound` keeping the object cleared, then re-creating it once the annotation is removed.

    Tests also cover:
    - `Locked` on a LoadBalancer outside deletion, with no effect;
    - the unknown value `Foo` on each kind;
    - an empty `outcome`;
    - the `message` annotation applied with `Error`, `Locked` and `Revoked`, and ignored with no cause, with `NotFound` and with a refused cause, each case tested;
    - a valid `source-version` on a new secret and on a synced secret;
    - an invalid `source-version`.
- **CAP-7**
  - **intent:** The fake writes like the operator and never more than needed.
  - **success:** Go tests show:
    - finalizer and status merge patches carry the whole list or struct and no `resourceVersion`;
    - a pass whose computed object equals the stored one sends no write request (no PATCH, reads allowed), so the `resourceVersion` stays the same;
    - no patch reaches a NamespaceRole;
    - the fake changes no annotation or label of any object;
    - no Event is created;
    - the finalizer patch is the only metadata write.
- **CAP-8**
  - **intent:** The fake runs unattended in a cluster and refuses to run beside the real operator.
  - **success:** Go tests show:
    - exit 1 when `KUBERNETES_SERVICE_HOST` is unset or empty, and exit 1 when `KUBERNETES_SERVICE_PORT` is unset or empty, each case tested;
    - at start only, exit 1 when a Deployment labelled `app.kubernetes.io/name: scaleway-operator` exists in `scaleway-system`;
    - the token is reread at each pass from the token file path passed in: the test rewrites the file between two passes, and kubetest expects the new token;
    - `/healthz` answers:
      - 503 until the first pass ends;
      - then 200 when the last pass ended less than 3 × `FAKE_OPERATOR_STEP_DELAY` ago;
      - 503 at or beyond that;
    - a failed list skips that kind for that pass only, the other kinds run, and the pass counts as ended;
    - SIGTERM ends with exit 0.
- **CAP-9**
  - **intent:** The fake ships as a local test image with its manifests, and never reaches a release artifact.
  - **success:**
    - `scripts/check-dockerfile.sh cmd/fake-operator/Dockerfile` passes.
    - `make build-fake-operator-image` builds `scaleway-manager-fake-operator:local`.
    - A test in `tests/make`, run by `make test-go`, asserts that the rules under `tests/cluster/fake-operator/` equal the companion's "RBAC rules" table, as an order-insensitive set:
      - for the ClusterRole;
      - for the Role, whose namespace must be `scaleway-system`.

      The test also asserts that no rule names `secrets` or a `*` wildcard.
    - The same test asserts that the string `fake-operator` appears neither in `Dockerfile`, nor in any file under `charts/`, nor in `.github/workflows/deliver.yml`.

## Constraints

- **Contract:** `scaleway-operator-v0.1.15` with CRDs `0.1.14`; the operator code is read only at that tag. Another release means reviewing the fake.
- **Dependencies:** Go standard library only, no `require` in `go.mod` (AD-1).
- **Scaleway:** no call to the Scaleway API (AD-2).
- **Secrets:** the fake has no access to Secrets, so the operator checks that read Secrets are not mimicked (companion, "Not mimicked").
- **Tokens:** `internal/kube` reads no token itself, for both the server and the fake (AD-3, AD-9).
- **Isolation:** the fake never appears in the release image, `Dockerfile`, chart or release workflow.
- **Causes:** each cause applies on every pass while present, with no memory. It names a Scaleway-side cause, never a `sync_state`.

## Non-goals

- The automated-test kind cluster, CRD installation, loading the image and the real-cluster smoke test: #53.
- Watch, create and delete in `internal/kube`, which come with the server.
- The AD-2 discovery and review endpoints of the readiness screen: the readiness feature adds them to the same URL builder.
- `RDB` and any kind absent from `v0.1.15`.
- Mimicking Secret-dependent checks, `last_synced_resource_version`, `FinalizerRemovedWithoutScalewayDelete` or the circuit breaker. The Secrets feature will need `@manual` scenarios for these.
- Gherkin scenarios and steps.

## Success signal

`make check` is green with the Go tests of CAP-1 to CAP-9. Against kubetest, an Instance created in a namespace with a `NamespaceRole` `Editor` and a UUID `project-id` goes:

1. finalizer;
2. `Syncing`;
3. `Synced` with `state: running` and a `192.0.2.x` IP;
4. with `outcome: Error`, `Error`;
5. once the annotation is removed and the object deleted, gone.

`make build-fake-operator-image` produces the image.

## Assumptions

- `current_version` starts at 1 on a new secret. That is Scaleway's numbering, not the operator code's.
