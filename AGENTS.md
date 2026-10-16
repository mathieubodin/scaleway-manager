# scaleway-manager

Web interface hosted in a Kubernetes cluster (often a throwaway kind cluster) that drives Scaleway through custom resources. `scaleway-operator` (separate repository, same author) reconciles them into infrastructure. Project is at the brief stage: no code, no stack chosen. Planning lives in `_bmad-output/planning-artifacts/`.

## Working rules

- Never fill a gap with an assumption. Leave it blank and marked, or ask one yes/no question.
- Never reassure. State facts and results only.
- Honor every rule the user gives. Never replace a rule with your own judgment; if you think a rule should be broken, say so before acting.
- Be concise. Verbosity is forbidden.

## Policy

- The backend talks only to the Kubernetes API. Never call the Scaleway API from this project; that is the operator's job.
- Never add a confirmation or any safeguard to deletion.
- Never use `default`, `kube-system`, `kube-public` or `kube-node-lease` for Scaleway resources; a dedicated namespace is required.
- Every managed Scaleway resource is a CR. Do not display Scaleway objects that have none.
- A YAML export never contains a Secret's value (the Secret `scaleway-ns-creds-{namespace}` holds the IAM key); export references only.
- An export keeps, unchanged, the namespace name, its `scaleway.mathieubodin.io/project-id` annotation and each CR's name: the operator adopts existing resources by these keys (Instance: zone + name + project; LoadBalancer and ScalewaySecret: tags with namespace + CR name). Without them, re-import creates duplicates in Scaleway.

## Versioning

- Everything in `_bmad-output/` is committed to Git: brainstorms, plans, briefs, forge sessions. This overrides the global rule "brainstorm and plan files are never committed" for this project.
- Exception: `.memlog.md` files are git-ignored (working memory, not deliverables).

## Where things are

- Brief: `_bmad-output/planning-artifacts/briefs/brief-scaleway-manager-2026-10-16/brief.md`
- Scoping session: `_bmad-output/forge/scaleway-manager-web-ui/forged-idea.md`
- Operator CR types and adoption: `/home/mathieu/dev/github.com/scaleway-operator/src/resources.rs` and `src/scaleway.rs`

## Running and verifying

- No command exists yet (stack not chosen). Fill in once code exists.
