# Addendum: scaleway-manager

Sources: forge session `_bmad-output/forge/scaleway-manager-web-ui/` and a web research pass run during this brief. The research was done through search and page fetches by a subagent; its dates are often partial and several items are unverified (listed below).

## Rejected alternatives (from the forge)

- First slice "list Instances, create form, delete": too small, ignores the operator's real workflow and the cluster check.
- Guided wizard creating namespace + `NamespaceRole` + Secret in one go: unbalanced.
- Two-section home page: only an idea until design is done.
- Showing Scaleway resources that have no CR: meaningless in this model (no CR, no operator).

## Facts from the `scaleway-operator` code

- CRDs in group `scaleway.mathieubodin.io/v1`: `Instance`, `LoadBalancer`, `ScalewaySecret` (namespaced), `NamespaceRole` (cluster-wide). No Kapsule CRD yet.
- `status.sync_state`: Pending, Syncing, Synced, Error, TerminationBlocked; `error_message` is available.
- Deleting a CR destroys the Scaleway resource through finalizers.
- Orphan adoption: `Instance` by name + zone + project, `LoadBalancer` and `ScalewaySecret` by tags.
- Prerequisites before creating an `Instance`: CRDs installed, operator healthy, Secret `scaleway-credentials` in `scaleway-system`, namespace annotated `scaleway.mathieubodin.io/project-id`, `NamespaceRole` named like the namespace (Editor or Admin to create), Secret `scaleway-ns-creds-{namespace}`.
- The operator health endpoint returns 503 on an invalid global token; per-namespace credentials are not covered.

## Comparable tools (research, as reported)

| Tool | Relevance | Source |
| --- | --- | --- |
| Headlamp (kubernetes-sigs) | Extensible UI; guided creation forms only for native types; shows and edits CRDs in YAML | github.com/kubernetes-sigs/headlamp |
| Headlamp kro plugin (alpha) | Views KRO definitions and instances | github.com/headlamp-k8s/plugins/pull/1182 |
| kro-ui (pnz1990, unofficial) | KRO dashboard; instance forms from the schema; read-only ("No mutating k8s API calls") | github.com/pnz1990/kro-ui |
| Komoplane (Komodor) | Crossplane troubleshooting, read-only | github.com/komodorio/komoplane |
| Upbound Console / UXP 2.0 | Commercial or community UI for Crossplane | upbound.io/uxp-2 |
| Backstage + TeraSky plugins | Forms from XRD, CRD and KRO; GitOps pull requests | github.com/TeraSky-OSS/backstage-plugins |
| Kubernetes Dashboard | Archived 2026-01-21 | github.com/kubernetes-retired/dashboard |
| Rancher Dashboard, Lens, Argo CD | Generic Kubernetes UIs; raw YAML for custom resources | various |

Scaleway side:

- Official Crossplane provider: github.com/scaleway/crossplane-provider-scaleway (Upjet, Apache-2.0, v0.6.0).
- Old official operator `scaleway/scaleway-operator`: archived 2022-11-21, RDB only.
- No Scaleway-specific UI driving custom resources was found. No Scaleway + KRO combination was found.
- KRO now lives under kubernetes-sigs/kro (v0.9.4, v1alpha1, per one 2026 article).

## Risks surfaced by the research

- Redundancy: a Headlamp plugin could cover much of the generic CRUD. The brief must justify not being one.
- Dependency: the UI's value is tied to the operator, which has few resource types today, unless the UI stays generic over CRDs.
- Security: a published backend with write access to cloud-resource CRDs and Scaleway keys, with no safety rail on delete.

## Not verified

Licences (Headlamp, Komoplane, TeraSky, Headlamp kro plugin), exact release years (Scaleway Crossplane provider v0.6.0, Komoplane), last-commit dates (kro-ui, TeraSky), open-source status of the Upbound Web UI, Port, Argo CD CRD UI, the official Scaleway console, the full resource list of the Crossplane provider, and whether the Crossplane Headlamp plugin generates edit forms.

## Open questions

- Why not a Headlamp plugin.
- How to ensure the cluster can talk to Scaleway: to verify, validate, implement and hard-code later.
- Hand-written forms versus KRO-composed CRDs the UI has never seen (stage 3).
- What each namespace section displays; form design.
- Identity used to write CRs (shared service account versus the user's identity): set aside for a single-user tool, but revisit since the project is to be published for any user.
