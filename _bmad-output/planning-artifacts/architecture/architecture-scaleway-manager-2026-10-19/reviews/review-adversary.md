---
review: adversary
target: ARCHITECTURE-SPINE.md (status draft, 2026-10-19)
date: '2026-10-19'
method: construct pairs of feature units that each obey every AD to the letter yet build incompatibly; each pair is a hole, closed by a one-sentence rule
sources_checked:
  - ARCHITECTURE-SPINE.md, .memlog.md
  - brief.md, AGENTS.md
  - scaleway-operator src/resources.rs, src/reconcilers.rs, src/context.rs
---

# Adversarial review: architecture spine

## Verdict

Not ready to bind parallel feature work. The spine fixes the boundaries well (one outbound peer, user token, zero dependency, raw JSON, one stream, server-side policy), but it fixes almost none of the contracts *between* units: the API URL scheme, the update method and its concurrency, the SSE event shape, the error mapping, the export field list, the re-import apply semantics, and the "managed namespace" predicate. Each one lets two compliant units diverge. 17 holes found: 4 critical, 7 high, 6 medium. Each closes with one sentence; none requires reopening an adopted decision, except H5 (AD-6 contradicts itself) and C4 (AD-5 scope).

Severity scale: **critical** = silent data corruption or Scaleway duplicates; **high** = units that cannot be integrated without rewriting one; **medium** = drift or rework, detectable by tests.

## Code-grounded facts used

- Operator status fields are serialized snake_case (no `rename_all` in `resources.rs`): `status.sync_state`, `status.error_message`. But the operator's own `printcolumn` for `ScalewaySecret` uses `.status.syncState`, and Instance/LoadBalancer printcolumns use `.status.scalewayId`, `.status.publicIp`, `.status.vipAddress`. Anyone copying field paths from `kubectl get` output or the CRD's `additionalPrinterColumns` gets camelCase paths that never match. (This is also an operator bug, to report upstream.)
- Initial `sync_state` differs by kind in the real operator: Instance and ScalewaySecret default `Syncing`, LoadBalancer `Pending`. The fake (AD-9) always starts at `Pending`.
- `NamespaceRole` is a fourth, cluster-scoped CR; its name equals the namespace it serves, and the operator refuses to reconcile a namespace without one.
- The IAM credentials Secret `scaleway-ns-creds-{namespace}` lives in `scaleway-system`, not in the managed namespace. `ScalewaySecret.spec.source.kubernetes_secret` references a Secret in the CR's own namespace.
- The operator requires the `project-id` annotation to be a UUID.

## Holes

### C1 (critical): update path and Kubernetes conflicts

- **Pair:** "Instance edit form" vs "LoadBalancer edit form".
- **Both comply:** the Instance form `GET`s the object, edits `spec` in the browser, and sends the whole object back; the server checks `Synced` and `PUT`s it (resourceVersion included, 409 on conflict). The LoadBalancer form sends only edited fields; the server re-reads the object fresh, checks `Synced`, merges, and `PUT`s with the *fresh* resourceVersion. AD-4 (byte-equivalent `spec` except edited fields) and AD-6 (server rejects non-`Synced`) are satisfied by both.
- **Incompatibility:** the second path never returns 409 and silently overwrites changes the user never saw (operator or another tab). The first path round-trips `spec` through `JSON.parse`/`JSON.stringify`, which renders numbers and key order differently, so "byte-equivalent" holds for one and not the other. The `Synced` check is time-of-check/time-of-use in both: the status can leave `Synced` between the server's read and its write.
- **Rule:** every CR update is a JSON merge patch (`application/merge-patch+json`) holding only the edited `spec` fields plus the `metadata.resourceVersion` the user saw; the server runs the `Synced` check on a read at that same resourceVersion, answers 409 `conflict` on mismatch, and never retries or re-bases a write.

### C2 (critical): export field list is a denylist, not an allowlist

- **Pair:** "export" vs "re-import".
- **Both comply:** AD-5 keeps "the namespace name" and each CR's `apiVersion`, `kind`, `metadata.name`, `spec`, and lists what is dropped. Export unit A therefore keeps `metadata.namespace`, all labels and annotations; export unit B keeps only the listed fields and drops `metadata.namespace` (the namespace "name" is kept on the Namespace object). Re-import unit expects A.
- **Incompatibility:** with B, `kubectl apply -f` puts every CR in the current context namespace, which breaks the LoadBalancer and ScalewaySecret adoption tag (namespace + CR name), and Scaleway gets duplicates. With A, the export carries `kubectl.kubernetes.io/last-applied-configuration`, `ownerReferences`, `deletionTimestamp`, and the test-only `fake.scaleway-manager.test/outcome`, so a re-import in the test cluster forces `Error`.
- **Rule:** the export keeps exactly `apiVersion`, `kind`, `metadata.name`, `metadata.namespace`, `metadata.labels`, `metadata.annotations` minus the `kubectl.kubernetes.io/` and `fake.scaleway-manager.test/` prefixes, and `spec`; every other field is dropped, and the `internal/policy` test enumerates this allowlist.

### C3 (critical): "Secrets by reference" has two readings

- **Pair:** "export" vs "re-import" (and "ScalewaySecret form").
- **Both comply:** export unit A emits a `v1/Secret` stub with `metadata` only, so the reference resolves; export unit B emits no Secret object, the reference being `spec.source.kubernetes_secret` inside the CR. Neither contains `data` or `stringData`.
- **Incompatibility:** with A, `kubectl apply` creates an empty Secret in the new cluster; the operator then syncs a missing key and the ScalewaySecret goes to `Error`, or worse, a later "fill gaps" unit sees the Secret as present and does not ask for the value. Re-import cannot tell a stub from a real Secret.
- **Rule:** an export contains no object of kind `Secret`; a Secret is referenced only by the CR field that names it, and re-import rejects any `Secret` item.

### C4 (critical): NamespaceRole and the export scope

- **Pair:** "namespace creation / readiness fill-gaps" vs "export".
- **Both comply:** AD-5 names the Namespace and "each CR". Export unit A includes the cluster-scoped `NamespaceRole` named after the namespace (it is a CR); export unit B includes only namespaced CRs (the namespace's CRs). AD-6's reserved-namespace check covers "Scaleway CRs in" a namespace, which a cluster-scoped `NamespaceRole` with `spec.namespace: default` escapes.
- **Incompatibility:** with B, the disposable cycle (brief success criterion 4) restores a namespace the operator refuses to reconcile. With A, a re-import unit that only creates namespaced objects rejects the item. A `NamespaceRole` for `kube-system` passes AD-6.
- **Rule:** an export covers one namespace and contains, in this order, its Namespace, the `NamespaceRole` whose name is that namespace, and its namespaced CRs; the reserved-namespace check also applies to `NamespaceRole.metadata.name` and `spec.namespace`.

### H1 (high): re-import apply semantics

- **Pair:** "re-import" vs "namespace creation" (and vs "export" ordering).
- **Both comply:** "Re-import accepts that format only" says nothing about how items are written. Unit A uses `POST` (create) and fails on the first existing object; unit B uses server-side apply and overwrites `spec` of existing objects, including ones not `Synced` (bypassing AD-6's update rule through a different verb); unit C continues past failures.
- **Incompatibility:** re-import into a cluster where the readiness screen already created the namespace fails under A, and under B silently changes adoption keys of live CRs. The page cannot render one result shape.
- **Rule:** re-import creates items in file order with `POST` only, never updates an existing object, and returns one result per item (`created`, `already-exists`, or the error envelope), continuing after a failure.

### H2 (high): no API URL scheme

- **Pair:** "Instance list page" vs "LoadBalancer form".
- **Both comply:** the conventions say only "JSON over HTTP under `/api/`". Unit A adds `/api/instances?namespace=x`; unit B adds `/api/namespaces/x/loadbalancers/y`; a third uses kind names (`Instance`) where the others use plurals.
- **Incompatibility:** the shared SSE module (AD-7) and a page store cannot key objects the same way across units; each new kind invents routes; RBAC ClusterRole entries (AD-12) cannot be derived from routes.
- **Rule:** CR routes mirror the Kubernetes REST path under `/api/k8s/` (`/api/k8s/apis/scaleway.mathieubodin.io/v1/namespaces/{ns}/{plural}/{name}`, `/api/k8s/api/v1/namespaces/{name}`) for an allowlist held in `internal/server`, and every other route is a named action under `/api/actions/` (`export`, `import`, `readiness`, `login`, `logout`).

### H3 (high): SSE event contract is unspecified

- **Pair:** "Instance list page" vs "readiness screen" (both subscribers of AD-7's shared module).
- **Both comply:** one stream per session, one shared module, no polling. Unit A expects raw watch events `{type, object}` and does its own initial `GET` list; unit B expects the stream to start with the current state. Nothing says what the server watches, how a subscriber widens the scope on a single already-open `EventSource`, what happens on watch `410 Gone`, or how a mid-stream 401 (token expired) reaches the page.
- **Incompatibility:** A loses events between its `GET` and the first event; B shows an empty screen; after a 410 one unit relists and the other shows stale data forever; the "never poll" rule forbids the obvious workaround.
- **Rule:** the server watches a fixed set (Namespaces, the four operator CRDs' kinds, cluster-wide) with the user's token, each SSE message is `event: <plural>` with data `{type, object}` and `id` = resourceVersion, the stream opens and reopens after any 410 with synthetic `ADDED` events followed by `event: synced`, and a watch error ends the stream with `event: error` carrying the error envelope.

### H4 (high): two error shapes for Kubernetes errors

- **Pair:** "LoadBalancer form" vs "re-import".
- **Both comply:** the convention says a Kubernetes `Status` error "keeps its HTTP code, and its `message` is passed through", and AD-4 says Kubernetes object shapes pass through unchanged. Unit A wraps it in the envelope with `code` = `"kubernetes"`; unit B returns the raw `Status` object (it is a Kubernetes object). Policy rejections: one unit answers 403, another 409, another 422, with codes it invents.
- **Incompatibility:** the page cannot tell 409 `AlreadyExists` (create) from 409 `Conflict` (stale edit) from a policy "not `Synced`", and one shared error renderer cannot read both shapes.
- **Rule:** every non-2xx body is the envelope, with `code` = the kebab-case of the Kubernetes `Status.reason` (`conflict`, `already-exists`, `forbidden`, `not-found`, `unauthorized`) or a policy code declared once in `internal/policy` (`namespace-reserved` 422, `not-synced` 409, `immutable-field` 422).

### H5 (high): AD-6 contradicts itself; status field names have two sources

- **Pair:** "Instance list page" (JS) vs "Instance edit route" (Go).
- **Both comply:** AD-6 lets the page "hide or disable controls for the same rules" and also claims to prevent "two copies of a rule that drift apart"; the page's copy *is* the second copy. The JS unit reads the field path from the CRD printer columns or `kubectl get -o wide` (`.status.syncState` for ScalewaySecret); the Go unit uses `sync_state`. A missing `status` (just created) is "not `Synced`" in Go and "Pending" in one JS view, "Unknown" in another.
- **Incompatibility:** the ScalewaySecret edit button is never enabled (or always enabled) while the server rules the opposite; each view labels an absent status differently.
- **Rule:** the page's mirror of a policy rule lives in one module `web/policy.js`, and both it and `internal/policy` are tested against the same fixture table `internal/policy/testdata/` (input object, expected verdict and display state), whose field paths are checked against the pinned CRD schema.

### H6 (high): who owns the Namespace, and what makes it "managed"

- **Pair:** "readiness screen (fill gaps)" vs "namespace list / namespace creation".
- **Both comply:** the brief asks both to create what is missing. The readiness unit creates a namespace with the `project-id` annotation; the namespace unit lists namespaces that contain Scaleway CRs and creates namespaces with a label `app.kubernetes.io/managed-by`. Neither validates the annotation as a UUID (the operator does).
- **Incompatibility:** a namespace created from readiness is invisible in the namespace list until it has a CR; a namespace created from the list is reported "missing" by readiness; a non-UUID project-id is accepted and fails only in the operator.
- **Rule:** a namespace manages a Scaleway project if and only if it carries a UUID `scaleway.mathieubodin.io/project-id` annotation, decided by one `internal/policy` function, and only one server action creates or annotates namespaces.

### H7 (high): one stream "per browser session" vs several tabs

- **Pair:** "SSE relay" (server) vs "shared subscription module" (page).
- **Both comply:** the session is cookie-wide, so all tabs share it. The server unit enforces AD-7 literally and closes the older stream when a second opens; the page unit opens one `EventSource` per page load as AD-7 tells it to.
- **Incompatibility:** two open tabs knock each other's stream off in a loop; each reconnect re-lists and restarts API watches.
- **Rule:** the limit is one `EventSource` per page load, opened only by the shared module; the server accepts several streams per session and closes them all on logout or session end.

### M1 (medium): Kubernetes 401 vs session end

- **Pair:** "login/session" vs "any form".
- **Both comply:** AD-3 says a 401 "is shown to the user as is". Tokens from `kubectl create token` expire in one hour by default. One unit keeps the session alive and shows the 401 on each call; another drops the session and redirects to login.
- **Rule:** a Kubernetes 401 on any call ends the server session and returns `unauthorized`, and the page module returns to the login screen on that code from either `fetch` or the stream.

### M2 (medium): browser routing

- **Pair:** "namespace page" vs "Instance form".
- **Both comply:** AD-8 says nothing on client routing. One uses `#/ns/x`; the other uses `history.pushState('/ns/x/instances/new')`, which the server answers with 404 on reload because it serves assets at `/`.
- **Rule:** client routes are hash routes (`#/...`), and the server serves only real asset paths and `/api/`.

### M3 (medium): page state, optimistic updates, delete with finalizers

- **Pair:** "Instance list page" vs "delete action".
- **Both comply:** the delete unit removes the row when `DELETE` returns 200; the list unit removes it only on the `DELETED` event. With a finalizer (AD-9), `DELETE` returns 200 and the object stays `Terminating`, possibly forever on `TerminationBlocked`, which brief criterion 1 requires to be visible.
- **Rule:** the page holds one store keyed by `<plural>/<namespace>/<name>`, changed only by stream events, and no unit updates the UI from a write response.

### M4 (medium): fake operator vs Gherkin steps

- **Pair:** "fake operator" vs "Gherkin step library".
- **Both comply:** AD-9 defines the annotation but not who sets it, when it is read, the transition timing, or what happens on a spec edit. One step definition sets the annotation through the UI form (so the product form must expose arbitrary annotations); another sets it with `kubectl` before creation and waits a fixed delay. The fake reads the annotation only on first reconcile; an edit scenario changes it later and waits for `Error` forever. The fake starts at `Pending` while the real Instance starts at `Syncing`, so a scenario asserting the first state passes in tests and fails manually.
- **Rule:** the fake operator restarts `Pending` → `Syncing` → `Synced` (or the annotated outcome) on every `metadata.generation` or annotation change, with a step delay from one env variable, and scenarios set test-only annotations only through one `kubectl`-backed step file, never through the UI, and never assert a state before the first one the real operator writes.

### M5 (medium): each Go package fakes the Kubernetes API its own way

- **Pair:** "edit route tests" vs "re-import tests".
- **Both comply:** the convention says "Kubernetes API faked with `net/http/httptest`". One fake enforces resourceVersion and returns 409; the other accepts any write; neither models the status subresource or finalizers.
- **Rule:** all Go tests use one fake API server in `internal/kube/kubetest` that enforces resourceVersion conflicts, the status subresource, finalizer-delayed deletion and Kubernetes `Status` error bodies.

### M6 (medium): readiness identity and RBAC

- **Pair:** "readiness screen" vs "chart RBAC" (AD-12).
- **Both comply:** one unit runs readiness before login with the pod service account (AD-3 only forbids it "for user actions"); another runs after login with the user token and checks CRDs by listing `customresourcedefinitions` (cluster-wide, apiextensions). AD-12 says the pod account has nothing "identified yet"; the first unit adds rights to it, the second adds CRD list rights to the user role.
- **Rule:** readiness runs only after login with the user's token, detects CRDs through API discovery (`/apis/scaleway.mathieubodin.io/v1`), and reports a 403 on a check as "cannot verify" naming the missing verb and resource, so the pod service account stays without rights.

## Open questions for the author (not assumed)

- Should `scaleway-system` (holder of every `scaleway-ns-creds-*` Secret) join the reserved-namespace list of AD-6? Yes/no.
- Are the adoption-key fields (Instance `spec.zone`, `spec.name`) editable after creation? H4 proposes an `immutable-field` code; the list of immutable fields is the author's decision.
- Does the export include the `scaleway-ns-creds-{namespace}` reference at all (as a readiness hint), given it lives in another namespace? C3's rule says no Secret object; whether to mention it elsewhere is open.

## Not holes (checked)

- Outbound peers, token handling, zero dependency, image and chart packaging, CRD pinning: single owner, single path.
- Time format: Kubernetes already emits RFC 3339; no conflicting producer.
- Deletion without confirmation: stated on both sides in AD-6.
