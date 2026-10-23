# Adversarial review: AD-9 (fake operator), 2026-10-23

Target: `ARCHITECTURE-SPINE.md` AD-9 as changed on `feat/42-fake-operator` (`git diff HEAD`), read with AD-1 to AD-15.
Contract checked against: `/home/mathieu/dev/github.com/scaleway-operator/src/reconcilers.rs` and `src/resources.rs`.
`_bmad-output/forge/fake-operator/forged-idea.md` is cited where it holds a value the spine lacks; it does not bind other units.

Method: for each hole, two units that each obey every AD to the letter but do not work together.

## Verdict

AD-9 is much closer to the operator than before, but five gaps remain where units that each follow the spine still do not fit together. Three are high: steady-state writes, the delete path, and `ScalewaySecret` preconditions. Each can be closed with one or two sentences in AD-9.

## H1 (high): steady state and `NotFound` cause `resourceVersion` churn, which collides with AD-4

- **Location:** AD-9 Lifecycle, How it writes, Causes (`NotFound` row, "on every pass ... with no memory"); AD-4 Writing and Conflicts; AD-15 `kubetest`.
- **Pair A (fake vs server).** AD-9 says the fake writes status "with the whole struct" and "moves each object one step per pass". It never says that a `Synced` object with no cause annotation gets no write, or that ids and IPs are generated once. Fake 1 rebuilds the whole struct on every pass and draws a fresh random `scaleway_id` or `public_ip` each time. That obeys the letter, and the `resourceVersion` changes every 2 s. Under AD-4, the server's merge patch carries the `resourceVersion` the user saw and maps a `409` to `conflict` with no retry. Every edit form open longer than one step delay then fails.
- **Pair B (`NotFound` row vs "no memory").** The row says "ids cleared, then re-created". The rule says the effect is written "on every pass while the annotation is present, with no memory". Two readings both fit:
  - Fake 1 writes only the cleared state while the annotation is present.
  - Fake 2 runs the cycle clear, re-create, clear, and so on. An `Instance` alternates between `Syncing` with no id and `Syncing` with an id, and never reaches `Synced`. A `LoadBalancer` alternates between `Pending` and `Syncing`.

  A Gherkin step "Then the load balancer shows Pending" passes against Fake 1 and is flaky against Fake 2, a race inside a 2 s window. Fake 2 also bumps `resourceVersion` every pass, as in pair A.
- **Pair C (`kubetest` vs kind).** The kubetest design in `forged-idea.md` bumps `resourceVersion` on every write. The real API server skips a no-op patch and does not bump. So a Go test that checks "no new version after the steady state" behaves differently in `kubetest` than in the cluster. A fake that writes on every pass looks equally harmless in both, so the churn in pair A is never caught by a Go test.
- **Tightening (AD-9, How it writes):** "The fake patches only when the computed object differs from the one it read. Generated values (ids, IPs, `created_at`) are drawn once and kept until a cause clears them. While `NotFound` is present, it writes only the cleared state (`Instance`: `Syncing`; `LoadBalancer`: `Pending`). Re-creation starts on the first pass after the annotation is removed." AD-15: "`kubetest` does not bump `resourceVersion` on a patch that changes nothing."

## H2 (high): the delete path is no longer specified, and preconditions are not lifted on delete

- **Location:** AD-9 Lifecycle and Preconditions. The diff removed the old bullet "deletion: it removes the finalizer on delete". Deletion now appears only in the `Locked` row.
- **Pair (fake vs Gherkin delete scenarios, and the AD-6 "no safeguard" deletion).** The Lifecycle lists three steps and none of them is deletion. A fake built from the Lifecycle alone never removes its finalizer, and every `DELETE` from the UI hangs with `deletionTimestamp` set. That fake still meets the Contract bullet only by reading the Rust code, which is not where a builder looks for the lifecycle.
- **Second pair.** The Preconditions bullet says "When either fails, it writes `Error`" and has no exception for deletion. The real operator skips both lookups when `deletion_requested` (`reconcilers.rs`, `reconcile_instance_inner`, the `if !deletion_requested && !circuit_open` branch). Then `decide_next_action` returns `RemoveFinalizer` or `DeleteInstance` first.

  Scenario: a user deletes the `NamespaceRole`, then an `Instance` in that namespace. The real operator deletes it. A fake that follows AD-9 to the letter writes `Error` on every pass and the object never goes away.
- **Tightening (AD-9, Lifecycle):** add step 0: "When `deletionTimestamp` is set, before any precondition: it removes its finalizer with the whole remaining list, unless a `Locked` cause holds a `LoadBalancer` (`TerminationBlocked`, finalizer kept)."

## H3 (high): `ScalewaySecret` preconditions are missing, so the fake reports `Synced` where the operator reports `Error`

- **Location:** AD-9 Preconditions ("for `Instance` and `LoadBalancer`"), Identity ("no access to Secrets"), Prevents ("tests passing against behavior the real operator never has").
- **Operator facts:**
  - `decide_next_action_secret` adds the finalizer.
  - Then it writes `Error` when `spec.source` is not configured, when the source Kubernetes Secret is missing (`ErrorKsSecretNotFound`) or when its key is missing (`ErrorKsKeyNotFound`).
  - At creation it also writes `Error` when the namespace has no `project-id` annotation (`CreateAndSyncSecret`, `get_project_id_from_namespace_resource`).
- **Pair (fake vs Gherkin re-import and fill-the-gaps scenarios, AD-5 `required-secrets`, AD-12 credentials).** A re-imported `ScalewaySecret` whose source Secret is not created yet reaches `Synced` under the fake and `Error` under the operator. So a scenario that checks "the UI shows which Secrets to create" can pass against a state the operator never produces. That is exactly what Prevents rules out. The same holds for a `ScalewaySecret` in a namespace without the annotation.
- **Constraint:** the fake cannot see Secrets, by design. Read-only access to Secrets would conflict with "no access to Secrets", and RBAC `get` returns values.
- **Tightening (AD-9, Preconditions):**
  - "For `ScalewaySecret`, after the finalizer, it checks the namespace `project-id` annotation as for the other kinds."
  - "It does not check the source Secret: a scenario that needs a missing source Secret or key sets `outcome: Error` and `message` to the operator's text. No scenario may assert `Synced` as proof that a source Secret exists."

## H4 (medium): values shared with the future kind-cluster issue are only in the forge file

- **Location:** AD-9 Where it runs and Lifecycle ("a step delay set in its Deployment"); Structural Seed table ("`make` targets (#14, #15)"). The forge file moves deployment to a separate issue.
- **Pair (#42 manifests vs the issue that deploys `tests/cluster/fake-operator/` into kind).** The spine names none of these:
  - the image reference: the forge has `scaleway-manager-fake-operator:local`;
  - the make target that builds it: the forge has `make build-fake-operator-image`;
  - the namespace: the forge has `fake-operator`;
  - the environment variable for the step delay: named nowhere.

  The kind issue runs `kind load docker-image <ref>` and Gherkin step definitions must wait longer than 3 × delay. Both build from the spine, so each can pick its own name, and the Deployment then sits in `ErrImageNeverPull` or `ImagePullBackOff`, or the steps time out.
- **Tightening (AD-9, Where it runs):** "Image `scaleway-manager-fake-operator:local`, built by `make build-fake-operator-image`, loaded with `kind load`; Deployment `imagePullPolicy: Never`; namespace `fake-operator`; step delay env `FAKE_OPERATOR_STEP_DELAY` (Go duration, default `2s`). Gherkin waits are derived from that value." The env var name is a proposal; the spine does not set one yet.

## H5 (medium): `error_message` source, read-only role order, and `source-version` effect are underspecified

- **Location:** AD-9 Causes paragraph and Preconditions.
- **Pair 1 (fake vs Gherkin message assertions).** Three sources can write `status.error_message`:
  - the `message` annotation;
  - the "refused cause" message;
  - the operator's own text, for example `Revoked`'s static generic constant (`reconcilers.rs`: "error_message must be the static generic Revoked constant").

  No precedence is set. AD-9 also does not say whether `message` without `outcome` is copied into a `Synced` status, while the operator sets `error_message: None` on `Synced`. Fake A gives the annotation precedence and Fake B gives the operator's text, so the same step "Then the error message is X" passes for one and fails for the other.
- **Pair 2 (fake vs operator order).** The read-only role check (`BlockReadOnlyRole`: role not `Editor`, `Admin` or `OrganizationOwner`) runs after `AddFinalizer` in the operator, so the finalizer is added. AD-9 does not mention the role value. `forged-idea.md` lists it as a no-finalizer precondition, which is the wrong order. Under AD-9 a fake may either ignore the role, giving `Synced` for a `Viewer` role, or block before the finalizer. Both differ from the operator.
- **Pair 3 (`source-version`).** AD-9 says the annotation "sets `current_version` to `n`". It does not say:
  - the initial `current_version`, which the forge sets to 1;
  - the resulting `sync_state`;
  - how a non-integer `n` is handled;
  - the precedence against `outcome` (for example, `Revoked` with `source-version` present).
- **Tightening:**
  - "`message` applies only while `outcome` is present and replaces the operator's text. Without `outcome`, a `Synced` status has no `error_message`."
  - "A namespace role other than `Editor`, `Admin` or `OrganizationOwner` gives `Error` after the finalizer is added, as in `BlockReadOnlyRole`."
  - "`source-version` applies only when no `outcome` is present. It sets `current_version: n` and `Synced`. A value that is not a positive integer is a refused cause."

## H6 (low): how Gherkin steps write the annotations is not specified

- **Location:** AD-9 Gherkin steps; AD-6 Edit rule; AD-4 Writing.
- **Pair (Gherkin steps vs the UI).** AD-9 says steps steer only through annotations, but not through which client.
  - If a step writes them through the UI's edit path, AD-6 allows edits only at `Synced`. The step can then never remove `outcome: Error` from an object in `Error`, so the object is stuck.
  - If a step writes them with an admin kubeconfig, it bumps `resourceVersion` under any form the browser has open. That is a legitimate `409`, but undocumented.
- **Tightening:** "Steps write the annotations with the test cluster's admin credentials (`kubectl annotate`), never through the UI."

## Checked and not a hole

- **Export:** AD-5 drops `fake.scaleway-manager.test/` annotations at export, so the fake's causes do not survive a round trip. Re-import keeps whatever annotations a hand-written file carries. Only the fake reads them, and only in the test cluster.
- **Status writes vs AD-4:** the fake's status and finalizer patches carry no `resourceVersion`. The operator's `update_status` and `add_finalizer` do the same (`Patch::Merge`, default `PatchParams`), so this matches the contract. The merge-patch encoding matches too: the whole struct, with a `None` field serialized as `null` (`resources.rs` has no `skip_serializing_if`).
- **AD-3 vs Identity:** the server pod uses `automountServiceAccountToken: false` and the fake's Deployment mounts its token. These are separate pods, and `internal/kube` stays token-agnostic. No clash.
- **`LoadBalancer` `FinalizerRemovedWithoutScalewayDelete`:** the operator writes this `sync_state` (`reconcilers.rs`, line 1231) when the IAM Secret is missing at deletion. The fake cannot produce it without Secret access. It is out of the cause table by design, and `web/policy.js` must still label it, under AD-6 and the Status display convention.
