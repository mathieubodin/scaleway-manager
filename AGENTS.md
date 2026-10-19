# scaleway-manager

Web interface hosted in a Kubernetes cluster (often a throwaway kind cluster) that drives Scaleway through custom resources. `scaleway-operator` (separate repository, same author) reconciles them into infrastructure. Stack chosen (Setup #34): Go server and plain JavaScript page, standard library only. Planning lives in `_bmad-output/planning-artifacts/`.

## Working rules

- Never fill a gap with an assumption. Leave it blank and marked, or ask one yes/no question.
- Never reassure. State facts and results only.
- Honor every rule the user gives. Never replace a rule with your own judgment; if you think a rule should be broken, say so before acting.
- Be concise. Verbosity is forbidden.
- Every file committed to the project is written in English, whatever language the conversation uses. This matches `document_output_language` in `_bmad/`, which only BMAD skills read.

## Policy

- The backend talks only to the Kubernetes API. Never call the Scaleway API from this project; that is the operator's job.
- Never add a confirmation or any safeguard to deletion.
- Never use `default`, `kube-system`, `kube-public`, `kube-node-lease` or `scaleway-system` for Scaleway resources; a dedicated namespace is required.
- Every managed Scaleway resource is a CR. Do not display Scaleway objects that have none.
- An export (JSON) never contains a Secret object or value (the Secret `scaleway-ns-creds-{namespace}` holds the IAM key); export references only.
- An export keeps, unchanged, the namespace name, its `scaleway.mathieubodin.io/project-id` annotation, each CR's name and each CR's `spec`: the operator adopts existing resources by these keys (Instance: `spec.zone` + `spec.name` + project, `spec.name` being the Scaleway name, not the CR's `metadata.name`; LoadBalancer and ScalewaySecret: tags with namespace + CR `metadata.name`). Without them, re-import creates duplicates in Scaleway.

## Stack rules

The architecture spine holds the full rules: `_bmad-output/planning-artifacts/architecture/architecture-scaleway-manager-2026-10-19/ARCHITECTURE-SPINE.md`.

- The server, the page and the fake operator use only the Go standard library and the browser platform: no framework, no library, no `require` in `go.mod`. Test, lint and build tools are allowed at pinned versions, outside `go.mod` and `web/`.
- Every Kubernetes call the server makes carries the logged-in user's token, through `internal/kube`; the server pod's service account has no API rights.
- Export and re-import are JSON; re-import only creates.
- Custom resources stay raw JSON; updates are JSON merge patches carrying the `resourceVersion` the user saw.
- Secret values never reach the browser.
- The fake operator (`cmd/fake-operator`) is for automated tests only; Gherkin steps steer it through its annotations, never by writing `status`.

## Versioning

- Everything in `_bmad-output/` is committed to Git: brainstorms, plans, briefs, forge sessions. This overrides the global rule "brainstorm and plan files are never committed" for this project.
- `.memlog.md` files are committed too: `bmad-spec` treats its memlog as the canonical source and derives `SPEC.md` from it, and work must be resumable on another machine.

## Where things are

- Architecture spine (stack, invariants): `_bmad-output/planning-artifacts/architecture/architecture-scaleway-manager-2026-10-19/ARCHITECTURE-SPINE.md`
- Brief: `_bmad-output/planning-artifacts/briefs/brief-scaleway-manager-2026-10-16/brief.md`
- Scoping session (historical; the architecture spine prevails where they differ): `_bmad-output/forge/scaleway-manager-web-ui/forged-idea.md`
- Operator CR types and adoption: `/home/mathieu/dev/github.com/scaleway-operator/src/resources.rs` and `src/scaleway.rs`
- Lessons learned: `docs/solutions/`, by category, with YAML front matter (`module`, `tags`, `problem_type`); relevant when working in a documented area.

## Development process

- Every piece of work starts or resumes with `/sdlc-work` (`/sdlc-work <n>` for issue n). The process is `docs/development-process.md`.
- Never push to `main`; every change goes through a PR.
- A frozen test (listed in `frozen-tests.sha256`) changes only after the human decides it, then `make freeze`.
- No automated test touches Scaleway; `@manual` scenarios are played by the human.
- Every PR adds or changes `docs/migrations/next/<issue>.md`.

## Running and verifying

- `make help` lists the targets; `make check` runs the checks that need no issue number.
