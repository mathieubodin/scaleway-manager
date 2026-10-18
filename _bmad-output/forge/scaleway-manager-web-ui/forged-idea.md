# scaleway-manager — forged idea

## What it is

A web service hosted in a Kubernetes cluster (typically a throwaway kind cluster, started when needed and destroyed after). Its backend creates, reads, updates and deletes Kubernetes custom resources; `scaleway-operator` (and later KRO) reconcile them into real Scaleway infrastructure. The UI is the master on board: it takes control of what exists in a Scaleway project, through the credentials it is given.

## Staged, one after the other

1. Test `scaleway-operator`.
2. Test quick deployment of objects into Scaleway.
3. Test compositions of infrastructure objects with KRO, which could lead to a landing zone.

## Locked

- The cluster is the control plane. Every Scaleway resource the UI manages is represented by a CR. The UI never lists Scaleway resources that have no CR; existing infrastructure is taken over by writing matching CRs (the operator adopts by name/tags).
- Delete has no confirmation and no "forget" gesture. It can break things, deliberately.
- Reach: once a namespace can manage a Scaleway project, the UI does whatever it wants there, existing infrastructure included.
- The UI must fill gaps in the cluster (create, edit, update what is missing), not only report them.
- kind is disposable. Durable state = YAML export from the UI and later re-import.
- At least one non-default namespace is required before talking to Scaleway. The four default namespaces (`default`, `kube-system`, `kube-public`, `kube-node-lease`) are never valid support for Scaleway resources. Fixed list, nothing broader.
- Forms are written by hand, per resource type, iteratively.
- Edit is allowed only on resources that are `Synced`.

## Rejected

- First slice "list Instances + create form + delete": too small, ignores the operator's real workflow and the cluster check.
- Guided wizard that creates namespace + NamespaceRole + Secret in one go: unbalanced.
- Two-section home page: only an idea, design not done.

## Open (do not decide here)

- How to ensure the cluster can talk to Scaleway: to verify, validate, implement and hard-code later. Note: the operator health endpoint already returns 503 on an invalid global token; per-namespace credentials are not covered.
- What each namespace section shows; form design.
- Multiple Scaleway accounts: possible, minor.

## Facts to build on (from `scaleway-operator` code)

- CRDs in group `scaleway.mathieubodin.io/v1`: `Instance`, `LoadBalancer`, `ScalewaySecret`, `NamespaceRole` (cluster-wide). No Kapsule CRD yet.
- `status.sync_state`: Pending / Syncing / Synced / Error / TerminationBlocked; `error_message` available.
- Prerequisites before creating an Instance: CRDs installed, operator healthy, Secret `scaleway-credentials` in `scaleway-system`, namespace annotated `scaleway.mathieubodin.io/project-id`, `NamespaceRole` named like the namespace (Editor/Admin to create), Secret `scaleway-ns-creds-{namespace}`.
