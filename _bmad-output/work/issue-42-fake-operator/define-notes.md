# Define notes: #42 fake operator

## Parallax

Run `wf_dce30ae0-cbc`, 3 readers, none dead, on `_bmad-output/forge/fake-operator/forged-idea.md`.

### Important

#### P1. Token and CA mount path and how the token reaches the pod

- Question: Should the fake operator turn off automount and use an explicit projected token volume, and if so, at the standard serviceaccount path or at /var/run/secrets/fake-operator?
- Positions: readers 1: automountServiceAccountToken: false plus an explicit projected volume (token + kube-root-ca.crt) at the standard /var/run/secrets/kubernetes.io/serviceaccount; fixed paths, injectable in main for tests; readers 2: automountServiceAccountToken: true (default mount); token path overridable with FAKE_OPERATOR_TOKEN_FILE; readers 3: automountServiceAccountToken: false plus an explicit projected volume at /var/run/secrets/fake-operator/{token,ca.crt}; paths fixed in the binary
- Proposal: Turn off automount and use an explicit projected volume at the standard /var/run/secrets/kubernetes.io/serviceaccount path, with paths passed into main so tests can inject them.
- Answer: author: automountServiceAccountToken false; explicit projected volume (token and kube-root-ca.crt) at /var/run/secrets/kubernetes.io/serviceaccount; paths passed into main so tests inject them.

#### P2. Non-404 failure reading the NamespaceRole or the Namespace

- Question: When reading the NamespaceRole or Namespace fails with something other than 404, should the fake write an Error status on the object, or just log and skip it?
- Positions: readers 1: NamespaceRole read failure writes Error 'Kubernetes API error'; Namespace read failure writes Error 'Configuration error: Cannot access namespace `<ns>`: `<error>`'; readers 2: Both are logged and the object is skipped this pass; no status written; readers 3: NamespaceRole read failure writes Error 'Kubernetes API error'; Namespace read failure is logged and the object skipped
- Proposal: Mirror the operator: write Error 'Kubernetes API error' for a NamespaceRole read failure and 'Configuration error: Cannot access namespace …' for a Namespace read failure.
- Answer: decided by the contract (the operator code): write Error "Kubernetes API error" for a NamespaceRole read failure and the operator Configuration error message for a Namespace read failure.

#### P3. Revoked on a new ScalewaySecret: finalizer before or after

- Question: Should a brand-new ScalewaySecret carrying the Revoked cause get its finalizer before the Error status, or never get it while Revoked is present?
- Positions: readers 1, 2: Revoked is evaluated before the finalizer step; a new secret with Revoked gets Error 'Source Secret unavailable' and no finalizer; readers 3: The finalizer is added first (no check runs before it), then Revoked is evaluated
- Proposal: Follow the operator's order: Revoked is evaluated before the finalizer, so no finalizer is added while it is present.
- Answer: decided by the contract (the operator code): Revoked is evaluated before the finalizer; no finalizer while it is present on a secret without scaleway_id.

#### P4. Refused cause on a deleting object with scaleway_id

- Question: When an object is being deleted and carries an unsupported cause, should deletion proceed or be blocked with an Error?
- Positions: readers 1, 3: A refused cause never blocks deletion; the finalizer is removed; readers 2: Writes Error with the refused-cause message and keeps the finalizer
- Proposal: Let deletion proceed: a refused cause never blocks deletion.
- Answer: author: a refused cause never blocks deletion; the finalizer is removed.

#### P5. Message annotation with NotFound

- Question: With outcome NotFound and a message annotation, should error_message show the message or stay empty?
- Positions: readers 1, 3: A non-empty message replaces error_message, including the null that NotFound writes; readers 2: Ignored for NotFound, since that effect writes no message
- Proposal: Ignore the message for NotFound so the status matches what the operator writes (error_message null).
- Answer: author: ignored for NotFound, error_message stays null as the operator writes; the message annotation applies only to causes whose effect writes a message.

#### P6. Role check versus cause order for an object without scaleway_id

- Question: On a not-yet-created Instance or LoadBalancer in a read-only namespace, should a cause annotation win over the read-only role error?
- Positions: readers 1, 2: The cause is applied before the read-only role check (e.g. NotFound on a new object with a Viewer role gives the NotFound cleared state); readers 3: The read-only role check runs before creation-time causes (a Viewer role gives the role Error even when a cause is present)
- Proposal: Let the cause win, applied right after the finalizer step, so tests can force outcomes regardless of role.
- Answer: author: the role wins, as the operator: finalizer added, then the read-only role Error; creation-time causes apply only once the role is valid.

### Details

#### D1. Message annotation with a refused cause

- Question: With an unsupported cause and a message annotation, should error_message show the refusal text or the supplied message?
- Positions: readers 2, 3: Ignored; the refused-cause message is kept so the misuse stays visible; readers 1: Not addressed explicitly (rule says it replaces error_message whenever a cause is present and a status is written)
- Proposal: Keep the refusal text and ignore the message.
- Answer: author (details list validated together): the refusal text is kept and the message annotation is ignored.

#### D2. Default error_message for the Error cause

- Question: Should the default Error message be 'Scaleway API error: 500' or 'Scaleway API error: 500 Internal Server Error'?
- Positions: readers 1, 3: 'Scaleway API error: 500'; readers 2: 'Scaleway API error: 500 Internal Server Error'
- Proposal: Use whatever the operator's for_status produces for a non-JSON 500 body, which reader 2 reports as 'Scaleway API error: 500 Internal Server Error'.
- Answer: author (details list validated together): Scaleway API error: 500 Internal Server Error, confirmed at v0.1.15.

#### D3. Refused-cause message quoting

- Question: Should the refused-cause message quote the value with single or double quotes?
- Positions: readers 1, 2: fake-operator: cause '`<value>`' is not supported for `<Kind>`; readers 3: fake-operator: cause "`<value>`" is not supported for `<Kind>`
- Proposal: Single quotes, matching the operator's message style.
- Answer: author (details list validated together): single quotes: `fake-operator: cause '<v>' is not supported for <Kind>`.

#### D4. Invalid source-version message

- Question: Should an invalid source-version use its own dedicated message or the refused-cause message?
- Positions: readers 1, 3: Treated as a refused cause, with an error message naming the bad value; readers 2: Its own message: fake-operator: source-version '`<value>`' is not a positive integer
- Proposal: Use the dedicated 'source-version … is not a positive integer' message.
- Answer: author (details list validated together): a dedicated message: `fake-operator: source-version '<v>' is not a positive integer`.

#### D5. source-version present when the secret first reaches Synced

- Question: If source-version is already set when a new secret is first synced, should current_version go straight to n or go to 1 first and then to n?
- Positions: readers 3: The first Synced write uses n instead of 1 for current_version; readers 1: The first Synced write sets current_version 1; the next pass applies n; readers 2: Not addressed
- Proposal: Go to 1 first, then n on the next pass, so the version step can always be seen.
- Answer: author (details list validated together): current_version goes to 1 first, then to n on the next pass.

#### D6. Empty outcome annotation value

- Question: Should an empty outcome annotation be ignored or reported as an unsupported cause?
- Positions: readers 1: Refused cause (Error); readers 2, 3: Treated as absent
- Proposal: Treat it as absent.
- Answer: author (details list validated together): an empty outcome is treated as absent.

#### D7. Failure scope inside a pass (401, failed list)

- Question: When listing one kind fails, should the fake still process the other kinds in that pass?
- Positions: readers 1, 3: A failed list skips only that kind; other kinds continue; readers 2: A 401 or failed list skips the whole pass
- Proposal: Skip only the failed kind and continue with the others.
- Answer: author (details list validated together): a failed list skips only that kind; the other kinds continue.

#### D8. Pass scheduling and shutdown

- Question: Should a SIGTERM let the current pass finish, or cancel it after the in-flight request?
- Positions: readers 1, 3: One pass every step delay; on SIGTERM the current pass finishes, then exit 0; readers 2: Next pass starts one step delay after the previous ends; on SIGTERM the context is cancelled and the process exits after the in-flight request
- Proposal: Cancel the context and exit after the in-flight request, with the next pass scheduled one step delay after the previous ends.
- Answer: author (details list validated together): the next pass starts one step delay after the previous one ends; on SIGTERM the context is cancelled and the process exits 0 after the in-flight request.

#### D9. Missing KUBERNETES_SERVICE_HOST/PORT

- Question: Should the fake exit 1 at start when the API host or port variable is missing?
- Positions: readers 1, 3: Exit 1 at start; readers 2: Not addressed
- Proposal: Yes, exit 1 with one error log.
- Answer: author (details list validated together): exit 1 at start with one error log.

#### D10. Extra kubetest behaviors

- Question: Should kubetest include fault injection, 422 on finalizer add during deletion, and 415 on a wrong content type?
- Positions: readers 3: Fault injection per method/path, 422 on adding a finalizer during deletion, 405 on unknown methods; readers 2: 415 on a non-merge-patch content type; tokens recorded even on 401; readers 1: None of these
- Proposal: Include fault injection and 415, skip 422 since the fake never adds a finalizer to a deleting object.
- Answer: author (details list validated together): fault injection per method and path, and 415 on a wrong content type; no 422.

#### D11. internal/kube refuses an empty token

- Question: Should internal/kube refuse to send a request when the token is empty?
- Positions: readers 3: An empty token returns an error without sending a request; readers 1, 2: Not addressed
- Proposal: Yes, return an error before sending.
- Answer: author (details list validated together): internal/kube returns an error before sending when the token is empty.

#### D12. How acceptance tests drive passes

- Question: Should tests call a single-pass function directly with injected clock and randomness, or run the real loop with a tiny delay?
- Positions: readers 1, 3: Call an exported pass function directly with an injected clock and random source; no sleep; readers 2: Run the loop with a step delay of a few ms
- Proposal: Call the pass function directly with injected clock and randomness.
- Answer: author (details list validated together): tests call a single-pass function directly with an injected clock and random source; no sleep.

#### D13. Guard test location

- Question: Should the release-artifact guard test live in tests/make?
- Positions: readers 2, 3: tests/make; readers 1: Under tests/ (unspecified subfolder)
- Proposal: Put it in tests/make with the existing repository tests.
- Answer: author (details list validated together): tests/make.

#### D14. Migration note content

- Question: Should docs/migrations/next/42.md state 'Migration: no impact'?
- Positions: readers 3: docs/migrations/next/42.md reads 'Migration: no impact'; readers 1, 2: Only that the file exists (and passes check-migration for reader 2)
- Proposal: Yes, since the fake is test-only and changes no released artifact.
- Answer: author (details list validated together): Migration: no impact.

## Premortem

Method "Pre-mortem Analysis" (`bmad-advanced-elicitation`) on `forged-idea.md` and AD-9: three months later, the fake operator failed.

### R1. The fake drifted from the operator (important)

- Cause: the operator changed (a new `sync_state` or message) and the fake did not follow; AD-9 says "the operator code" without naming which version.
- Proposal: AD-9 and the spec name the imitated operator version, aligned with the CRD chart version pinned by AD-10; changing it requires reviewing the fake.
- Answer: author: the contract is the release scaleway-operator-v0.1.15 with CRD chart scaleway-operator-crds-v0.1.14, named in AD-9; the earlier analysis used a stale local branch (feat/milestone-1, 81 commits behind origin/main), so every AD-9 fact is re-verified at that tag; RDB (unreleased, main only) is deferred until released; changing the version requires reviewing the fake.

### R2. The fake rotted unused (important)

- Cause: the automated-test kind cluster issue and #15 stayed in Backlog.
- Proposal: create the kind-cluster issue at the delivery of #42 and link #15 to it.
- Answer: author: create the kind-cluster issue now; done as #53 (Backlog), blocked by #42, blocking #15; it carries the real-cluster smoke test (R3) and the Gherkin waits (R9).

### R3. kubetest diverged from the API server (important)

- Cause: Go tests green, the fake failing on a real cluster (status merge patch, `deletionTimestamp`).
- Proposal: a real-cluster smoke test is required in the kind-cluster issue; in #42, each kubetest behavior cites the Kubernetes documentation it imitates.
- Answer: author: in 42 each kubetest behavior is tested with a citation of the Kubernetes documentation URL it reproduces; the real-cluster check stays in 53.

### R4. The fake wrote fake status on real resources (important)

- Cause: its manifests were applied to the manual cluster, where the real operator runs; two controllers wrote the same objects.
- Proposal: the fake refuses to start (exit 1) when it detects the real operator; the manual-cluster make target never applies `tests/cluster/`.
- Answer: author: option A, at start the fake exits 1 when a Deployment labelled app.kubernetes.io/name=scaleway-operator exists in scaleway-system; RBAC adds a Role with list on deployments in scaleway-system only; the manual-cluster make target never applies tests/cluster/ (issue 53).

### R5. The server had to rewrite internal/kube (important)

- Cause: `internal/kube` was cut for the fake only, its URL builder covering only the kinds it used, against the closed list of AD-2.
- Proposal: write the AD-2 URL builder with its full closed list now; only the operations stay limited to what the fake uses.
- Answer: author: the AD-2 URL builder is complete in 42 (whole closed list, name escaping); operations stay limited to list, get, merge patch and status patch; watch, create and delete come with the server.

### R6. No liveness for the fake (important)

- Cause: nothing says how Kubernetes knows the fake is alive or stuck.
- Proposal: a minimal HTTP `/healthz` answering 200 while the last pass is recent, with probes in the Deployment; or no probe at all.
- Answer: author: option A, a standard-library HTTP /healthz answering 200 while the last pass is younger than 3 times FAKE_OPERATOR_STEP_DELAY, 503 otherwise, with a liveness probe in the Deployment; no readiness probe.

### R7. A Secret opt-in bug reached production (detail)

- Cause: Secret-dependent preconditions are not mimicked (accepted gap).
- Proposal: already accepted; the spec of #42 notes that the Secrets feature needs `@manual` scenarios for these cases.
- Answer: author (details list validated together): the spec notes that the Secrets feature needs manual scenarios for these cases.

### R8. Token expiry after one hour (detail)

- Cause: rereading the token was never tested, since tests last seconds.
- Proposal: a Go test where kubetest changes the expected token between two passes.
- Answer: author (details list validated together): a Go test where kubetest changes the expected token between two passes.

### R9. Flaky Gherkin waits (detail)

- Cause: waits shorter than 3 steps of 2 s.
- Proposal: the kind-cluster issue and the steps derive their waits from `FAKE_OPERATOR_STEP_DELAY`.
- Answer: author (details list validated together): covered by issue 53.

### R10. The trunk of #42 blocked (detail)

- Cause: frozen Go tests of different stories share the `internal/kube` and `kubetest` packages.
- Proposal: first real use of the rules of #48 and #50 (runner map, set aside); the plan avoids tests of different stories in one package where it can.
- Answer: author (details list validated together): the plan avoids tests of different stories in one package where it can; otherwise the rules of 48 and 50 apply.

## Contract verification at v0.1.15

Agent re-check of AD-9 against `scaleway-operator-v0.1.15` (CRDs `0.1.14`), after the first analysis was found to read a stale local branch (`feat/milestone-1`). Folded into AD-9 by contract:

- The ScalewaySecret finalizer is `scaleway.mathieubodin.io/secret-finalizer`.
- ScalewaySecret checks:
  - the region check comes before anything, deletion included;
  - `NamespaceRole` and the read-only role ("cannot sync secrets") are checked before the finalizer;
  - no `spec.source.kubernetes_secret` gives `Error` after the finalizer;
  - `project-id` is checked without a UUID check.
- A spec-list failure (zone, instance type, `lb_type`, region) writes no status and keeps the finalizer.
- `observed_generation`: a spec change pushes a new version.
- `NotFound` sets `state` to `unknown` (Instance) or `""` (LoadBalancer). A first `Error` carries the Rust defaults. A 500 gives `Scaleway API error: 500 Internal Server Error`.
- The `Error` cause on a synced ScalewaySecret has no effect, since the operator makes no Scaleway call in steady state.
- Not mimicked: `last_synced_resource_version`, `FinalizerRemovedWithoutScalewayDelete`, the circuit breaker.

### V1. ScalewaySecret Syncing

- Question: should `Syncing` get its own pass for a ScalewaySecret, or follow the operator, which writes `Syncing` and `Synced` in one pass?
- Answer: author: follow the operator, both in one pass.

## Research

No external factual question left open: Kubernetes behaviors are cited per kubetest test (R3), operator behavior was checked in its code at `v0.1.15`. No `bmad-deep-recon` run.

## Spec-lint

### Round 1 (`wf_0daaad1a-e88`)

Eight criteria failed "only one possible reading" (CAP-1, CAP-2, CAP-4 to CAP-9); all passed "says how it is verified". Answer, author, validated together:

- CAP-1: Bearer on all four operations; merge-patch content type on the two patches only; every AD-2 closed-list entry except the readiness endpoints; the CA is the `ca.crt` file passed in, a server certificate from another CA is refused.
- CAP-2: each of the eight API-server behaviors of the companion's kubetest section has at least one test citing its documentation URL; fault injection has its own test, no URL.
- CAP-4: no cause: finalizer removed, no check but the ScalewaySecret region; `outcome: Error`: finalizer kept, no status patch; `outcome: Locked` on a LoadBalancer: `sync_state: TerminationBlocked` with its message, finalizer kept; refused `outcome`: finalizer removed; ScalewaySecret invalid region: finalizer kept, region `Error`.
- CAP-5: one test per message of the Checks table, secondary messages included, for each kind a row applies to; region also on delete; spec lists one test per field per kind.
- CAP-6: one test per branch of each cell (`Revoked` with and without `scaleway_id`; ScalewaySecret `Error` at creation and once synced); refused: each refused cell plus an unknown value `Foo`; `message` applied for `Error`, `Locked`, `Revoked`, ignored with no cause, `NotFound` and a refused cause, each tested.
- CAP-7: no write request (PATCH) on a pass with no change, reads allowed; no patch on a NamespaceRole, no annotation or label change on any object, no Event; the finalizer patch is the only metadata write.
- CAP-8: exit 1 when `KUBERNETES_SERVICE_HOST` or `KUBERNETES_SERVICE_PORT` is missing, each tested; `/healthz` 200 when the last pass ended less than 3 × `FAKE_OPERATOR_STEP_DELAY` ago, 503 at or beyond.
- CAP-9: the manifests grant exactly the companion's verbs and resources and nothing on `secrets`; `fake-operator` absent from `Dockerfile`, every file under `charts/` and `.github/workflows/deliver.yml`.

### Round 2 (`wf_08bd9337-c17`)

All nine criteria failed "only one possible reading". Answer, author, validated together:

- CAP-1: namespace names follow DNS-1123 label, resource names DNS-1123 subdomain; each entry is compared with an exact expected URL in a table test.
- CAP-2: one test per sub-behavior (object and `/status` patch; bump, keep, `409`; `401` and token recording), each citing its URL; fault injection has one table test, no URL required.
- CAP-3: `current_version` exactly +1; `scaleway_id`, `public_ip`, `vip_address`, `created_at` unchanged over three more passes; the whole status compared with the companion table after each pass.
- CAP-4: each case on each kind it applies to; "its message" is the operator's fixed message; a refused cause on delete removes the finalizer with no status patch; "no check runs" is shown by deleting an object whose namespace has neither NamespaceRole nor `project-id`.
- CAP-5: `Cannot access namespace` for Instance, LoadBalancer and ScalewaySecret; the test fixes the error in kubetest and asserts the exact prefix `Configuration error: Cannot access namespace <ns>:` plus one space, followed by non-empty text.
- CAP-6: every cell has its test; two-branch cells one test per branch; `Foo` per kind; valid `source-version` on a new and on a synced secret. New behavior: `Locked` on a LoadBalancer outside deletion has no effect, tested.
- CAP-7: a pass with no change is one whose computed object equals the stored one; no annotation or label is changed by the fake.
- CAP-8: the real-operator guard runs at start only; the token is reread from the file path passed in, rewritten by the test between passes. New behavior: `/healthz` answers 503 until the first pass ends. A failed list skips that kind for that pass only, the other kinds run, and the pass counts as ended.
- CAP-9: rule by rule (`apiGroups`, `resources`, `verbs`) for the ClusterRole and for the Role, checked in `scaleway-system`.

### Round 3 (`wf_315969d5-d48`)

Seven criteria still failed "only one possible reading"; CAP-1 and CAP-7 passed. Author: fix the substantive ambiguities, then the spec gate, no fourth round.

- Fixed:
  - CAP-3: generated fields per kind, counted over three passes after `Synced`; the ScalewaySecret `Syncing` then `Synced` patches are checked in order in the kubetest request log.
  - CAP-4: the no-check case says which checks are skipped, and the ScalewaySecret in it has a valid region; the refused causes are named per kind.
  - CAP-5: finalizer presence follows the "before" and "after" columns; `Kubernetes API error` is checked per kind; each spec-list field is checked on the kinds that have it.
  - CAP-8: an environment variable that is unset or empty counts as missing.
  - CAP-9: an exact RBAC rules table in the companion.
- Left to the story specs and cold-start: the form of tests (one `Test` function or `t.Run` subtests, exactly one or at least one) and which failure a test injects.
