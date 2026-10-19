# Long route

From the confirmed route to the PR for a feature (`docs/development-process.md`, "Long route", and `simulation-long.md` next to the plan of epic #2). Steps: `define`, `plan`, `rails`, `build`, `review`, `deliver`. Each opens with event `step-start` and closes with its `step-end`; every workflow run gets its `workflow-run` line right after launch; no commit while a workflow runs. The work folder is `_bmad-output/work/issue-<n>-<slug>/`; `<n>` is the epic.

## Define (step `define`, board status Define)

1. `bmad-forge-idea` (Skill tool), on the idea and the issue body. It checks the code before each question, has every assumption validated, writes `_bmad-output/forge/<slug>/forged-idea.md` and hands control back through the memlog.
2. Launch `sdlc-parallax` with `{ ideaPath: "<forged-idea.md>" }`. Write each divergence to `define-notes.md`, section "Parallax".
3. Pre-mortem: `bmad-advanced-elicitation` with the method "Pre-mortem Analysis" on `forged-idea.md` ("three months from now the project failed, write the postmortem"). It has no hand-back: write each risk to `define-notes.md`, section "Premortem", yourself, and keep control.
4. Questions, decided with the author: each divergence and each risk is classified important (what the user sees or does, data, security, cost, acceptance) or detail, with a proposed answer (the parallax returns them classified; classify the risks the same way). Ask the important ones one at a time; show the details as one list of proposed answers, validated or corrected together. Every answer goes to `define-notes.md` under its question.
5. Research: when `define-notes.md` holds an external factual question without answer, propose `bmad-deep-recon`; the user decides; record a skipped research in "Research".
6. `bmad-spec` with `forged-idea.md` and `define-notes.md` as inputs: `_bmad-output/specs/spec-<slug>/SPEC.md`, with scope, non-goals, success signal, and a verification method for every acceptance criterion.
7. Spec-lint loop: resolve `spec_lint_questions` (`resolve_customization.py --skill {project-root}/.claude/skills/bmad-review ... --key workflow`, field `.workflow.spec_lint_questions`) and launch `sdlc-spec-lint` with `{ specPath, questions }`. Each failing criterion becomes a question to the user; the answer goes to `define-notes.md`, `bmad-spec` updates `SPEC.md`, and a new round runs. After 3 red rounds, tell the user and ask how to go on. No gate page while it is red.
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
4. Write the work folder's `plan.md`: explorations (with their sources), approaches, choice, contracts at each boundary between stories, sequential trunk and parallel leaves.
5. Story Breakdown of `bmad-spec` (interactive): `stories.yaml` in the spec folder.
6. Story specs: for each story, `bmad-build` steps 01 and 02 only, up to `ready-for-dev` (`docs/development-process.md`, "Plan and plan gate"): at its CHECKPOINT 1 answer "Approve and stop", except for a story with `spec_checkpoint: true`, which the user reviews; pass every question `bmad-build` asks to the user; commit each story spec before the next one (`docs: write the story spec <id> of #<n>`, `make cost` first); never run `bmad-build` again on a `ready-for-dev` story during Plan.
7. Cold-start loop: launch `sdlc-cold-start` with `{ storyPaths }`. Blocking items: fix `plan.md`, `stories.yaml` or the story specs, asking the user when a fix needs a decision, and run a new round; after 3 red rounds, tell the user and ask how to go on. When green, write the useful items of that round to the work folder's `cold-start.md`, per story.
8. `sdlc-gate-page` with `{ path: <work folder>, kind: plan }`.

## Plan gate

1. As the spec gate, on `plan.html`; the references are `plan.md` and the story specs.
2. Validated: SKILL.md, "Gate comments" (`step-end plan`, `step-start rails`, gate commit of the work and spec folders), then one sub-issue per story spec: the story spec's title, its content as body (`--body-file`), label `route:long`, added as a native sub-issue of the epic (`gh api repos/{owner}/{repo}/issues/<n>/sub_issues -F sub_issue_id=<id of the new issue>`), and added to the board with status `In Progress`.
3. Invalidated: as the spec gate, the return steps being the plan or Define.

## Rails (step `rails`)

1. Launch `sdlc-rails` with `{ mode: "long", items: <story spec paths>, testWriterPrompt: <sdlc_test_writer_prompt> }` (resolved from `bmad-build`, as in `short-bug.md`).
2. Show the suspect tests and the stories left without tests (`failed`); the user decides what to do before the build.
3. Commit the frozen tests and `frozen-tests.sha256`: `test: add the tests of #<n>` (`make cost` first).

## Build (step `build`)

1. Trunk first: launch `sdlc-build` with `{ mode: "long", items: <trunk story specs>, baseSha: <git rev-parse HEAD>, builderPrompt }`; commit the story specs marked `ticketImproved` first, then merge each finished branch as in `short-bug.md` (branch pattern, `--no-commit`, `make check-frozen`, `make check`, `make cost`, commit `feat: merge story <id> of #<n>`, worktree and branch removed).
2. Then the leaves, the same way, from the new `HEAD`, merged one by one with `make check-frozen` after each.
3. `backToPlan` true: show the failed stories with their reasons; the work goes back to Plan and its gate (new `step-start plan`); the finished branches stay merged.
4. `wrong base`: something was committed during the run; launch again.

## Review (step `review`)

Launch `sdlc-review` as in `short-bug.md`, with `specPath` the `SPEC.md`, `dryPasses: 2`, `maxRounds: 5`. Memlog result and commit as there. Not converged: show the recurring findings and propose back to Plan, back to Define, or accept and list them in the PR; the user chooses.

## Before deliver

1. `/ce-compound` on what the work taught; its result goes to the PR.
2. MemPalace: record the decisions of this work in the `scaleway_manager` wing (check duplicates first).
3. Issues in Backlog: a `route:short` issue `simplify after #<n>` with the simplification leads seen in review, and, when lessons recur, a "distill lessons" issue; show both to the user before creating them.

Then `references/deliver.md`, with: the PR body's first lines `Closes #<n>` then one `Closes #<k>` per sub-issue; the `/ce-compound` result; and the sub-issues following the epic, set to `Review` with it once the PR is open.
