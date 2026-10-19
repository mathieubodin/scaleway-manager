# Reconciliation review: ARCHITECTURE-SPINE.md against its inputs

- Reviewed: `ARCHITECTURE-SPINE.md` (222 lines) and `.memlog.md` (41 lines), 2026-10-19.
- Inputs: `AGENTS.md`, brief `brief.md` and `addendum.md`, `forged-idea.md`, `docs/development-process.md` (Setup guard, Testing without Scaleway, Behavior diff, Release, Migration files), `scaleway-operator/src/resources.rs` (plus `src/scaleway.rs` and `src/reconcilers.rs` for adoption and finalizer constants).
- Out of scope, as intended by the author: JSON instead of YAML, zero runtime dependency, Go, token auth, Gateway API exposure. Reported only where a consequence goes unrecorded.

## Verdict

Most inputs landed. Policy lines (Scaleway API ban, no delete safeguard, forbidden namespaces, CR-only display, Secret-free export), the fake-operator rules and the operator facts (`sync_state`, `error_message`, finalizer names, `project-id` annotation) all match the sources. Before the Setup gate, fix two high findings: the export loses `metadata.namespace`, and `NamespaceRole` is missing from the spine and blocked by AD-6. Six medium findings record consequences and markers the spine leaves out.

## Findings

### F1 (high): export does not pin `metadata.namespace` of each CR

- Spine: AD-5, lines 77-81. The kept-field list is "the namespace name and its annotation" plus "each CR's `apiVersion`, `kind`, `metadata.name` and `spec`". Line 81 says tests "pin the kept and dropped fields", so a field missing from the list is not pinned.
- Source: `AGENTS.md` line 20 ("LoadBalancer and ScalewaySecret: tags with namespace + CR `metadata.name`"). In the operator, `scaleway.rs:326` `lb_operator_tags(namespace, cr_name)` and `scaleway.rs:495` `secret_operator_tags(namespace, cr_name)` derive the adoption tags from the CR's namespace.
- Consequence: a `v1` `List` whose items have no `metadata.namespace` is applied by `kubectl apply -f` in the current context namespace, often `default`. The tags then fail to match, so Scaleway gets duplicates, and the CRs land in a forbidden namespace (AD-6). The spine names the Namespace object's name but never ties each CR to it.

### F2 (high): `NamespaceRole` is missing from the spine, and AD-6 blocks it

- `NamespaceRole` appears nowhere in the spine. AD-9 (line 109) covers only `Instance`, `LoadBalancer` and `ScalewaySecret`.
- Operator: `resources.rs:291-303`. `NamespaceRoleStatus` has `validation_state` (`Validated`, `Invalid`) and `error_message`, and no `sync_state`. It is cluster-scoped (`resources.rs:268-274`, no `namespaced`).
- AD-6 line 89 rejects "an update to a CR whose `status.sync_state` is not `Synced`". Read literally, every `NamespaceRole` update is rejected. This contradicts the brief (`brief.md:28`, "lets the user fill the gaps (create, edit, update)") and the forge (`forged-idea.md:18`). Changing a `Viewer` role to `Editor`, a prerequisite named at `forged-idea.md:40` and `addendum.md:18`, becomes impossible.
- AD-6 line 88 checks only the CR's namespace. A cluster-scoped `NamespaceRole` with `spec.namespace: default` passes the server check, yet it enables Scaleway in a forbidden namespace (`AGENTS.md:17`, `forged-idea.md:20`).
- The fake operator does not write `validation_state`, so the readiness screen cannot be tested against an `Invalid` role.

### F3 (medium): the pass-through convention sends Secret values to the browser

- Spine line 143: "Kubernetes object shapes pass through unchanged (AD-4)". AD-7 (line 97) relays watch events as they are.
- The readiness feature has to read and create `scaleway-credentials` in `scaleway-system` and `scaleway-ns-creds-{namespace}` (`addendum.md:18`, `forged-idea.md:40`). Under the pass-through rule, a GET or watch of those Secrets returns `data` (the IAM key) to the page.
- `AGENTS.md:19` bans Secret values only from the export, but the logging convention at line 148 shows the spine means to keep Secret values contained. No rule says whether the browser API strips Secret `data`.

### F4 (medium): the re-import cycle lacks the prerequisites it needs

- Success criterion 4 (`brief.md:61`) and the vision (`brief.md:87`) promise that re-importing in a new kind cluster gives "the same Scaleway project back under control".
- The operator needs, besides the CRs, the `NamespaceRole` named like the namespace and the Secrets `scaleway-ns-creds-{namespace}` and `scaleway-credentials` (`addendum.md:18`).
- AD-5 (lines 77-81) does not say whether the export contains the cluster-scoped `NamespaceRole`. Line 81 says Secrets appear "by reference only" but does not say what that reference is: a Secret object without `data`, or only the name inside `ScalewaySecret.spec.source.kubernetes_secret` (`resources.rs:187-198`). It also does not say what re-import does when the referenced Secret is missing.

### F5 (medium): no version on the export format, while re-import "accepts that format only"

- AD-5, line 81: "Re-import accepts that format only." Exports live outside the cluster for an unbounded time (`forged-idea.md:19`), and the format has no version field.
- `development-process.md:542-560` requires each PR to describe its upgrade and rollback. The spine does not say that a change to the export shape is a migration-impacting change, or that older exports must stay importable.

### F6 (medium): `[ADOPTED]` and convention lines include proposals the author has not validated

- `.memlog.md:41` lists these as "not yet validated by the author": export as a `v1` `List`, the fake-operator annotations `fake.scaleway-manager.test/outcome` and `/message`, the cookie flags, the error envelope, `/api` and `/api/events`, the `smgr-` prefix, slog, env config and the `tests/e2e/features` path. `.memlog.md:36` marks the `scaleway-manager-user` ClusterRole as `[ASSUMPTION]`.
- In the spine, the `List` sits in AD-5 `[ADOPTED]` (line 77), the annotations in AD-9 `[ADOPTED]` (line 114) and the cookie flags in AD-3 `[ADOPTED]` (line 65). The conventions table (lines 141-150) and AD-12 carry no marker. `.memlog.md:38` left the format open as "List or multi-object JSON".
- This breaks the `AGENTS.md:7` rule: an assumption must stay marked.

### F7 (medium): the spine records none of the input documents its decisions make inaccurate

- `.memlog.md:38` and `.memlog.md:40` say that YAML must become JSON in the brief and in `AGENTS.md`, and that `AGENTS.md` drops "no code, no stack chosen". The spine has no consequence or follow-up section that carries this. The lines that become stale:
  - `AGENTS.md:3` ("no code, no stack chosen") and `AGENTS.md:19` ("A YAML export").
  - `brief.md:31`, `:39`, `:61`, `:72` and `:87` (YAML export and re-import). AD-5 contradicts these lines silently. The spine lists `brief.md` as a source (line 13) and never names the deviation.
  - `development-process.md:528` and `:538`, which say the fake operator, Gherkin runner and behavior diff come "once the stack is chosen". The stack is now chosen.
  - `development-process.md:533`: the manual-cluster target is described without the CRD chart and Envoy Gateway, which AD-10 and AD-11 (lines 120 and 129) and the spine line 210 now require.
  - `development-process.md:566` and `:597`: image and chart publishing and evidence checks "once the stack is chosen".
  - `development-process.md:589` (CI): the spine line 209 runs automated tests in CI on kind with Envoy Gateway. The CI section has no such job. Behavior diff stays local-only (`:538`), and the spine does not separate it from the e2e runs.

### F8 (medium): snake_case CR fields are not stated as a convention

- Operator `resources.rs` has no `serde(rename_all)`, so the JSON fields are snake_case: `instance_type`, `boot_volume_size`, `lb_type`, `scaleway_role`, `kubernetes_secret`, `sync_state` and `error_message`.
- The operator's printcolumns use camelCase (`.status.scalewayId`, `.status.syncState` at `resources.rs:12-14`, `115-117`, `208-210`, `275-276`). Those paths do not exist in the objects (an operator defect), so copying them is wrong.
- The spine uses `status.sync_state` correctly (lines 89 and 111) but has no convention row warning that hand-written forms must use snake_case `spec` keys. A Kubernetes-habit camelCase key is an unknown field and is not the adoption key.

### F9 (low): AD-4 promises byte equivalence that its own mechanism cannot give

- AD-4, line 71: "`json.RawMessage` or `map[string]any` ... `spec` is written back byte-equivalent". Re-encoding a `map[string]any` with `encoding/json` sorts the keys and drops the original whitespace. Byte equivalence holds only for the `RawMessage` path. The adoption keys need semantic equality, so the stronger wording will make tests ambiguous.

### F10 (low): AD-2 constrains the open readiness check, and the spine does not say how

- `forged-idea.md:32` and `addendum.md:19`: operator health is an HTTP endpoint on the operator. AD-2 (line 59) allows only calls to the API server. Under AD-2, the readiness screen can reach that endpoint only through the API server's service proxy (an extra `services/proxy` verb in AD-12) or through pod readiness. The deferral at line 220 does not name this limit.

### F11 (low): fake lifecycle differs from the real initial states

- AD-9, line 111: `Pending`, then `Syncing`, then `Synced` for all three kinds. The real defaults (`resources.rs:100`, `:177`, `:260`) are `Syncing` for `Instance` and `ScalewaySecret` and `Pending` for `LoadBalancer`. This is within the accepted drift (`development-process.md:534`). Scenarios must not assert `Pending` for an `Instance` as real behavior.

### F12 (low): tooling and process facts from the memlog are missing from the Stack table

- `.memlog.md:39` names gofmt, go vet and Biome as checks. `.memlog.md:18` names the `MUTATION=0` switch. None of these is in the spine (lines 152-170) or the conventions table.
- AD-1 (line 53) forbids `require` in `go.mod`, which also rules out the Go 1.24+ `tool` directive. The spine does not say how `go-mutesting` is installed (for example `go run pkg@version`).

### F13 (low): release consequences are not recorded

- `development-process.md:598`: "The command that reinstalls the previous version is added with the image and chart publishing". AD-11 (lines 126-129) does not name it, does not say whether chart version and app version follow the tag, and does not say how rollback works when the operator CRDs are pinned separately (AD-10).

### F14 (low): open questions from the brief disappear without a trace

- `brief.md:63` (no success criterion for a stranger installing it) and `addendum.md:53` (why not a Headlamp plugin) are not carried into the Deferred list. Choosing a standalone Go app implicitly answers the Headlamp question, and the spine does not say so.
- `forged-idea.md:20` ("at least one non-default namespace before talking to Scaleway") is a readiness rule that the spine does not mention.

## Checked and landed

- `AGENTS.md:15`, no Scaleway API: AD-2, line 59.
- `AGENTS.md:16`, no delete safeguard: line 91.
- `AGENTS.md:17`, fixed forbidden namespaces: line 88.
- `AGENTS.md:18`, CR-only display: line 91.
- `AGENTS.md:39`, no automated test touches Scaleway: lines 59, 114 and 150.
- Operator: `sync_state` values, `error_message`, finalizer names (`reconcilers.rs:19-21`), the `project-id` annotation (`context.rs:7`) and the CRD group all match the spine.
- Setup guard (`development-process.md:232`): the spine exists under `architecture/` and names the stack (lines 152-170).
- Testing without Scaleway (`:530-534`): AD-9, AD-10 and the spine line 209.
- Behavior diff (`:538`): ARIA snapshots and playwright-bdd (AD-8, line 166).
- Release (`:597`): `oci://ghcr.io/<owner>/charts` (AD-11, line 128).
- Brief edit-when-`Synced`: AD-6, line 89.
- Addendum identity question (`addendum.md:57`): AD-3, with OIDC deferred at line 218.
