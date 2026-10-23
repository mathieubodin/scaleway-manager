# Long route

From the confirmed route to the PR for a feature (`docs/development-process.md`, "Long route", and `simulation-long.md` next to the plan of epic #2). Steps: `define`, `plan`, `rails`, `build`, `review`, `deliver`. Each opens with event `step-start` and closes with its `step-end`; every workflow run gets its `workflow-run` line right after launch; no commit while a workflow runs. The work folder is `_bmad-output/work/issue-<n>-<slug>/`, the spec folder `_bmad-output/specs/spec-<slug>/`; `<n>` is the epic.

Text written by agents (story titles and bodies, review leads) follows the SKILL.md rule for text from people: through a file, never in a command line. Before such text goes to GitHub, look for `@<login>` mentions and closing keywords (`close`, `fix`, `resolve` and their forms, followed by `#<k>`); show any to the user and put them in backticks. A story id goes into a command or a commit message only when it matches `^[A-Za-z0-9._-]+$`.

## Define (step `define`, board status Define)

Preamble, before step 1: answer, for this feature, (a) what is authoritative on the expected behavior: code at an exact version, a standard, a document, or the author; (b) which resources to read first to fill the missing context: a short ordered list to start with, not an exhaustive list of gaps. Show both as a TL;DR, ask the user whether resources are missing, and let the user validate the list. Read the resources, then write `define-notes.md`, section "Sources": the authority, each resource with its exact reference, and what was read; memlog decision `define sources validated: <authority>`. Forge starts from what was read: pass it the "Sources" section with the idea.

1. `bmad-forge-idea` (Skill tool), on the idea and the issue body. It checks the code before each question, has every assumption validated, and hands control back through the memlog. Only its "Hardened" exit writes `_bmad-output/forge/<slug>/forged-idea.md`. "Killed": tell the user and ask whether to close the epic (`gh issue close <n> --reason 'not planned'`) or stop here. "Clearer" without a forged idea: ask the user how to go on.
2. Launch `sdlc-parallax` with `{ ideaPath: "<forged-idea.md>" }`; readers in `deadReaders` are reported. Write each divergence to the work folder's `define-notes.md`, section "Parallax".
3. Pre-mortem: `bmad-advanced-elicitation` with the method "Pre-mortem Analysis" on `forged-idea.md` ("three months from now the project failed, write the postmortem"). It has no hand-back: write each risk to `define-notes.md`, section "Premortem", yourself, add a memlog result `pre-mortem: <count> risks in define-notes.md`, and keep control.
4. Questions, decided with the author: each divergence and each risk is classified important (what the user sees or does, data, security, cost, acceptance) or detail, with a proposed answer (the parallax returns them classified; classify the risks the same way). Ask the important ones one at a time; show the details as one list of proposed answers, validated or corrected together. Every answer goes to `define-notes.md` under its question.
5. Research: when `define-notes.md` holds an external factual question without answer, propose `bmad-deep-recon`; the user decides; record a skipped research in "Research".
6. `bmad-spec` with `forged-idea.md` and `define-notes.md` as inputs: `SPEC.md` in the spec folder, with scope, non-goals, success signal, and a verification method for every acceptance criterion.
7. Spec-lint loop: resolve `spec_lint_questions` (`uv run {project-root}/_bmad/scripts/resolve_customization.py --skill {project-root}/.claude/skills/bmad-review --project-root {project-root} --key workflow`, field `.workflow.spec_lint_questions`) and launch `sdlc-spec-lint` with `{ specPath, questions }`. The round is red when `failing` is not empty or `noCriteria` is true (a spec without criteria, or a dead listing agent). Each failing criterion, or the missing criteria, becomes a question to the user; the answer goes to `define-notes.md`, `bmad-spec` updates `SPEC.md`, and a new round runs. After 3 red rounds, tell the user and ask how to go on. No gate page while it is red.
8. `sdlc-gate-page` with `{ path: <work folder>, kind: spec }`.

## Spec gate

1. Print the absolute path of `spec.html` and run `xdg-open` on it; the reference is `SPEC.md`. Ask: validated, or invalidated with a reason.
2. Validated: follow SKILL.md, "Gate comments" (decision, `step-end define`, `step-start plan`, `make cost`, gate commit of the work, forge and spec folders, push, comment), then `scripts/project-item.sh status <n> 'In Progress'`.
3. Invalidated: memlog decision and comment (SKILL.md, "Gate comments"); propose a return step from the reason (forge if the problem is ill-posed, parallax or pre-mortem if a risk is missing, `bmad-spec` if it is the writing); the user chooses; go back to that step of Define.

## Setup guard

Before Plan: `_bmad-output/planning-artifacts/architecture/` must exist and name the stack. Otherwise tell the user that Setup (a separate session with `bmad-architecture`) comes first, and stop.

## Plan (step `plan`)

1. Reread the project memory: the architecture spine, `docs/solutions/` when it exists, the previous `SPEC.md` files under `_bmad-output/specs/`, and a MemPalace search in the `scaleway_manager` wing on the feature's subject.
2. Launch `sdlc-explore` with `{ specPath }`; a section in `failed` is reported to the user.
3. Propose 2 or 3 approaches with their trade-offs, from the map; the user chooses (clickable). `bmad-architecture` only when the choice changes the spine.
4. Write the work folder's `plan.md`: explorations (with their sources), approaches, choice, contracts at each boundary between stories, the sequential trunk in its order, and the parallel leaves.
5. Story Breakdown of `bmad-spec` (interactive): `stories.yaml` in the spec folder.
6. Commit `plan.md`, `stories.yaml` and the work folder (`docs: plan #<n>`, `make cost` first): `bmad-build` stops on uncommitted changes.
7. Story specs: for each story of `stories.yaml`, run `bmad-build` on that story, naming the spec folder and the story id, so that it writes `<spec folder>/stories/<id>-<slug>.md`; steps 01 and 02 only, up to `ready-for-dev` (`docs/development-process.md`, "Plan and plan gate"). At its CHECKPOINT 1 answer "Approve and stop", except for a story with `spec_checkpoint: true`, which the user reviews; pass every question `bmad-build` asks to the user; commit each story spec before the next one (`docs: write the story spec <id> of #<n>`, `make cost` first); never run `bmad-build` again on a `ready-for-dev` story during Plan.
8. Cold-start loop: launch `sdlc-cold-start` with `{ storyPaths }`. Rewrite the work folder's `cold-start.md` at each round with that round's useful items, per story. Blocking items, or stories whose agent died (`dead`): fix `plan.md`, `stories.yaml` or the story specs, asking the user when a fix needs a decision, commit, and run a new round; after 3 red rounds, tell the user and ask how to go on.
9. `sdlc-gate-page` with `{ path: <work folder>, kind: plan }`.

## Plan gate

1. As the spec gate, on `plan.html`; the references are `plan.md` and the story specs.
2. Validated: SKILL.md, "Gate comments" (`step-end plan`, `step-start rails`, gate commit of the work and spec folders). Then one sub-issue per story spec that has none yet (existing ones: `gh api repos/{owner}/{repo}/issues/<n>/sub_issues --jq '.[].title'`): title and body (the story spec's content) through files; no `route:*` label (a sub-issue is not a separate work); attach it with `gh api repos/{owner}/{repo}/issues/<n>/sub_issues -F sub_issue_id=<id>`, `<id>` the numeric id from `gh api repos/{owner}/{repo}/issues/<k> --jq .id`; add it to the board with `scripts/project-item.sh status <k> 'Backlog'`; memlog result `sub-issue <k> story=<id>`.
3. Invalidated: as the spec gate, the return steps being the plan or Define.

## Rails (step `rails`)

1. Contract stubs: before the test writers, write in the work branch a stub for every signature the plan's contracts fix (the code those stories will own): the same names and types, a body that returns a "not implemented" error or a zero value (a panic only where nothing else fits), so that a frozen test fails on behavior, not on compilation, and one failing stub does not stop the other tests of its package. The stubs are committed with the frozen tests (step 5). They keep `make lint-go` and its pre-commit hook green on every commit of the build; no hook is skipped. A builder replaces the bodies and never changes a signature: changing one is a return to Plan.
2. Launch `sdlc-rails` with `{ mode: "long", items: <story specs without frozen tests yet>, testWriterPrompt: <sdlc_test_writer_prompt> }` (resolved from `bmad-build`, as in `short-bug.md`). After a return to Plan, only new or changed story specs get new tests.
3. Show the suspect tests and the stories left without tests (`failed`); the user decides what to do with each (rewrite by hand, delete the suspect file, or go on) before the build.
4. Runner map: write the work folder's `runner-map.md`, one row per file of `frozen-tests.sha256`, with the runner that reaches it and the story whose code makes it pass. Runners are make targets reached by `make check` (`make test-workflows` for `tests/workflows/**/*.test.js`, `make test-go`, `make lint-chart` for the Helm unit tests) or CI jobs of `.github/workflows/pr-checks.yml` (the `build-image` job for `tests/image/*.test.sh`). Report every frozen file with no runner to the user before the build, who decides its runner (a make target or CI job, then named in its story spec) or drops the file; the build does not start while a file has no runner. Rationale: `docs/solutions/workflow-issues/map-each-frozen-test-to-its-runner-after-rails.md`.
5. Commit the frozen tests, the contract stubs, `frozen-tests.sha256` and `runner-map.md`: `test: add the tests of #<n>` (`make cost` first).

## Build (step `build`)

1. Set every sub-issue to `In Progress` (`scripts/project-item.sh status <k> 'In Progress'`): sub-issues follow the epic.
2. Trunk, one story at a time, in the order of `plan.md`: launch `sdlc-build` with `{ mode: "long", items: [<that story spec>], baseSha: <git rev-parse HEAD>, builderPrompt }` and merge its branch before the next trunk story starts, so that each one is built on the code of the previous ones. Merge as in `short-bug.md` (branch pattern, `--no-commit`, `make check-frozen`, `make check`, `make cost`, commit `feat: merge story <id> of #<n>`, worktree and branch removed), with this merge rule: `make check-frozen` passes, and every `make -k check` failure is a frozen test of a story not built yet according to `runner-map.md` (`-k`, so that a failing target does not hide the targets after it). Go, as a fallback behind the contract stubs of Rails step 1 (only for a frozen test that calls a symbol outside the contract): a frozen `*_test.go` that refers to symbols its story has not written yet breaks the compilation of its whole package, so `make lint-go` and `make test-go` fail there, and the tests of built stories in that package do not run. Those failures are expected (a `make lint-go` failure is checked against the file's `make test-go` row of `runner-map.md`), and the package is then checked with those files set aside: rename each frozen `*_test.go` of a story not built yet in it to `<file>.aside` (Go ignores it), run `go vet <package>` and `go test <package>`, which must pass, then rename them back and run `make check-frozen`, before any commit. Before the launch, append those expected failures to the `builderPrompt` of that run, so that the builder neither stops on them nor touches them, with the Go files to set aside the same way to run its own tests, and to rename back before it commits. At the merge, list them in the memlog result `merge story <id> expected-failures=<files, or none> set-aside=<Go packages checked with files set aside, or none>`. Any other `make check` failure blocks the merge: abort the merge and tell the user. After the last story (the last leaf, or the last trunk story without leaves) `make check` must be fully green, with no expected failure left. Before merging, commit every story spec the improver edited (`ticketImproved`, or a failed story).
3. A story in `failed` with `backToPlan`: stop here, before the leaves; show the failed stories with their reasons; the work goes back to Plan (new `step-start plan`), the finished stories stay merged, and only new or changed story specs go through the plan gate's sub-issues, rails and build again. `wrong base`: something was committed during the run; launch again.
4. Leaves: one `sdlc-build` run with all of them, from the new `HEAD`, merged one by one the same way, with the same merge rule (a leaf not merged yet counts as not built yet). The run has a single `builderPrompt`: list the expected failures and the Go files to set aside there per leaf, and say that the frozen tests of a builder's own story are never expected failures for that builder.
5. `stories.yaml` fields: a story with `done_checkpoint: true` pauses after its merge for the user to look at it; a story with `invoke_dev_with` is built in its own run, with that text appended to `builderPrompt`.

## Review (step `review`)

Launch `sdlc-review` as in `short-bug.md`, with `specPath` the `SPEC.md`, `dryPasses: 2`, `maxRounds: 5`. Memlog result and commit as there. The validated spec holds until proven otherwise: a finding that contradicts `SPEC.md` or a frozen test is never fixed and never changes a frozen test; the user dismisses it, or it is recorded as a memlog result `open finding <id>: <claim>; scenario: <targeted scenario or test case that would settle it>` and listed in the PR. Not converged: show the recurring findings and propose to accept and list them in the PR, back to Plan, or back to Define; the user chooses. Only a return to Define reopens the spec. A return goes through that step and its gate again, and on to rails and build only for the new or changed story specs.

## Deliver (step `deliver`)

Open `deliver` here (event `step-start step=deliver`); `references/deliver.md` then does not open it again.

1. `/ce-compound` on what the work taught; commit what it writes under `docs/solutions/` (`docs: record the lessons of #<n>`, `make cost` first); its result goes to the PR.
2. MemPalace: record the decisions of this work in the `scaleway_manager` wing (check duplicates first).
3. Issues in Backlog: a `route:short` issue "simplify after" this work with the simplification leads seen in review, and, when lessons recur, a "distill lessons" issue; their bodies go through files, the leads inside a fenced code block; show both to the user before creating them, then `scripts/project-item.sh status <k> 'Backlog'`.

Then `references/deliver.md`, with: the PR body's first lines `Closes #<n>` then one `Closes #<k>` per sub-issue; the `/ce-compound` result; and every sub-issue set to `Review` once the PR is open.
