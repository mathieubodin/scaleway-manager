# Cold-start: #42

Rounds 1 (`wf_724308f2-deb`) and 2 (`wf_f1f5c655-bad`) were red; their fixes are in `_bmad-output/specs/spec-fake-operator/go-interfaces.md`.

## Round 3 (`wf_682f3126-35d`)

Green: no blocking item, no dead agent. Fixed after the round: precedence of `go-interfaces.md` on behavior, function-local annotation constants in story 4, the kubetest pitfall line of `plan.md` (D10), and the migration note left to the deliver step. Useful items, per story:

### 1-kubernetes-client.md

- Return values of String(), Kind() and Namespaced() for a Resource outside the closed list (e.g. Resource(0) or Resource(99))
- How the http.Client handles redirects (CheckRedirect) and proxies (Transport.Proxy: none, or ProxyFromEnvironment)
- Whether sentinel errors are returned bare or wrapped with context (e.g. fmt.Errorf("%w: ..."))

### 2-fake-api-server.md

- How the token is taken from the Authorization header: is only `Bearer TOKEN` accepted, and what is recorded and answered for a header with another scheme (e.g. `Basic x`) or `Bearer` with an empty value?
- Value of `metadata.resourceVersion` in a list response before any change. The global counter "starts at 1": is the current value "0" or "1" when the store is empty?
- Definition of a no-op patch: is "changes nothing" a deep comparison of the merged `map[string]any`, where `json.Number` "1" and "1.0" count as different? Does it run after the ignored keys are dropped (status on the object endpoint, metadata.name, namespace and resourceVersion)?
- Content-Type matching for 415: an exact string match, or the media type parsed so that parameters such as `; charset=utf-8` are accepted?
- Contradiction between plan.md and go-interfaces.md on adding a finalizer while `deletionTimestamp` is set. The plan's pitfalls say "kubetest refuses it too"; the round-2 clarification (D10) says it does not refuse it.
- Field list of `Request`: the story spec gives `{Method, RequestURI, ContentType, Body}`, and go-interfaces.md adds `Token string`.
- Which path `Fail` matches `pathPrefix` against: the decoded `r.URL.Path` or the escaped path or RequestURI.
- Whether `generation` increments when `spec` changes on kinds that are not custom resources (Namespace, Secret, Deployment, CRD), and what changes for a kind that has no `spec`.

### 3-fake-operator-engine.md

- Which document wins where the story spec and go-interfaces.md disagree on things other than signatures. The story gives a 7-step pass order and has main.go call signal.Notify on SIGTERM only. go-interfaces.md gives an 8-step order, with revokedBeforeFinalizer plus applyCause between checkBefore and the finalizer, and has main.go notify SIGTERM and SIGINT. It also adds pieces the story does not mention: an operator.token field, newOperator, readToken, writeStatus, setFinalizer, defaultStatus, currentStatus, and the revokedBeforeFinalizer stub in causes.go.
- The full status written by a ScalewaySecret version push (Synced plus generation change): is error_message written as null, and is current_version still sent as a JSON integer (last_synced_resource_version is already said to be always null)?
- Instance creation pass: the value of public_ip, and LoadBalancer creation pass: the values of vip_address and created_at, when the object already has a status (for example after an Error written at creation by story 4 or 5). Are they written as null or kept?
- Whether this story's change must add or update docs/migrations/next/42.md. AGENTS.md requires it for every PR; the story's Code Map says 'Touch nothing else'.

### 4-deletion-and-operator-checks.md

- Names of the annotation constants story 4 declares (outcome and message keys), and confirmation that the `fake.scaleway-manager.test/message` key also gets a story-4-local constant
- How checkAfter gets the role (Instance, LoadBalancer) and the namespace annotation (ScalewaySecret): a second GET in checkAfter, or reuse of what checkBefore read; and what a read failure there writes
- Behavior when the `project-id` annotation is present but empty
- Whether the Rust `spec.name cannot be empty` check (reconcilers.rs validate_spec, Instance) is deliberately left out of specListsValid
- Handling of an absent or non-string `spec.region`, `spec.zone`, `spec.instance_type` or `spec.lb_type`
- The other status fields on a Locked deletion (TerminationBlocked)

### 5-causes-by-annotation.md

- Whether story 5 edits operator.go at all
- Which status fields are kept when an Error is written for a refused cause, an invalid source-version, or Revoked without scaleway_id
- Exact parsing rule for a 'positive integer' source-version

### 6-runtime.md

- Whether `operator.pass` returns nil when one or more per-kind `List` calls fail, i.e. whether a non-nil return from `pass` always means 'failed before listing' (token read).
- The log record for a pass error in the loop: `msg` text, level and attributes.
- What happens when the health server's `Shutdown` exceeds its 5 s timeout or returns an error during SIGTERM handling (exit code, log message, whether `Close` is called).
- What to do when the health server's `Serve` fails after a successful bind (log and exit 1, or keep looping).
- Behavior when a signal arrives during the start sequence, before the loop begins (for example while the guard `List` is running).

### 7-packaging-and-isolation.md

- Names of the manifest objects: ServiceAccount, ClusterRole, ClusterRoleBinding, Role, RoleBinding, Deployment, container name, pod labels/selector, and confirmation that the ServiceAccount and Deployment live in namespace `fake-operator`
- Who writes `tests/make/fake_operator_test.go`: the builder (Tasks lists it) or the frozen rails tests
- Exact place of the new CI step in the `build-image` job: right after `- run: make build-image` (before 'Run the image tests') or after the image-tests step
