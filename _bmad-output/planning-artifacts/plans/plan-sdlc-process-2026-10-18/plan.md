---
topic: Implementation plan of the SDLC-based development process
status: draft
created: 2026-10-18
source: _bmad-output/brainstorming/brainstorm-sdlc-local-workflow-2026-10-16/
---

# SDLC development process: implementation plan

Scope: "Step 4" and "GitHub mapping" of `decisions.md`. Each unit below is meant to become one GitHub sub-issue of a parent issue labeled `epic`.

Out of scope: the Setup skill (Setup is run manually; `sdlc-work` only guards it), the fake operator (project code), the choice of stack.

The simulations `simulation-long.md`, `simulation-short.md` and `simulation-bug.md` walk each route on a concrete case. Their decisions (referenced below as long G*n*, short G*n*, bug G*n*) supersede the brainstorm and are carried into the units.

## Decisions taken while planning

These supersede the brainstorm where they differ.

| Topic | Decision |
| --- | --- |
| Stack-dependent `make` targets (coverage, mutation, code tests, behavior diff, manual kind cluster) | Separate units, marked "blocked by stack choice". Stack-independent targets are delivered now. |
| Epic | Parent issue + `epic` label + native sub-issues. Verified: creating sub-issues works on this personal repository (epic #2, sub-issues #3 to #26, unit U*n* = issue #*n+2* for U1 to U24; U25 to U29 have no issue yet). |
| Route labels | `route:short`, `route:bug`, `route:long`; `epic` added on the parent issue of long work. Default labels stay. |
| Custom BMad module | `mathieubodin-sdlc`, under `skills/` (`bmad_builder_output_folder`). |
| Skill names | `sdlc-work` (replaces `/work` everywhere) and `sdlc-gate-page`. The `x:y` form exists only for Claude Code plugins; the BMad installer copies skills flat into `.claude/skills/`. |
| Dynamic workflows | Stages without human interaction run as saved Claude Code workflows in `.claude/workflows/` (versioned, outside the BMad module and the `bmad-help` catalog): `sdlc-parallax`, `sdlc-explore`, `sdlc-cold-start`, `sdlc-rails`, `sdlc-build`, `sdlc-review`. A workflow accepts no input mid-run, so `sdlc-work` (skill) keeps every interaction and launches a workflow between two of them; alerts become return values. Reason: counters and caps enforced by script, intermediate results kept out of the session context, per-agent audit in `/workflows`. |
| Workflow tooling | Lint and format with Biome (`biome.json`, `make lint-workflows`). Control flow tested with a `node:test` harness that fakes `agent`, `parallel`, `pipeline` and `args`; coverage with `c8` (`make test-workflows`). |
| Build merges | `sdlc-build` returns the finished builder branches and the failed stories; `sdlc-work` merges them into the work branch one by one with `make check-frozen` after each. Trunk and leaves are two runs: trunk, merge, then leaves, merge. |
| Triage criteria | None fixed. `sdlc-work` proposes a route justified by risk × blast radius; the human confirms. Documented as open. |
| Documentation lint | `markdownlint-cli2`, configured in `.markdownlint-cli2.yaml`, run by `make lint-docs` locally and in CI. |
| Process memlog | `_bmad-output/work/issue-{n}-{slug}/.memlog.md`. Gate HTML pages, the evidence record, `define-notes.md`, the per-work `plan.md` and `bug-report.md` go in the same folder. `SPEC.md`, `stories.yaml` and the story specs stay where `bmad-spec` and `bmad-build` write them (`_bmad-output/specs/spec-{slug}/`) and are referenced from the memlog. |
| Branch and slug | Branch `{type}/{n}-{slug}` (type = Conventional Commits type planned for the PR); slug derived from the issue title, kebab-case, at most 4 words, confirmed with the route (long G1, G3). |
| Migration files after a release | Nothing is moved. "Deliver" keeps only the `docs/migrations/next/` files added or changed since the previous tag (`git diff --name-only <prev-tag>..HEAD`); all files when no tag exists. |
| Review loop | Workflow `sdlc-review`, parameterized by `args`: long route 2 dry passes, at most 5 rounds; bug route 1 dry pass, at most 2 rounds; short route one pass, verified findings fixed, no second pass. Lenses (security, correctness, coherence) and the verifier live in the workflow only; no `bmad-code-review` override (`bmad-code-review` has no loop anyway). Beyond the cap, the workflow returns the recurring findings; `sdlc-work` proposes a way out and the human chooses (long G23, bug G4, short G2). |
| Role of `bmad-build` | Used only during Plan, to write the story specs up to `ready-for-dev`. Test writers and builders run in the workflows `sdlc-rails` and `sdlc-build` (long G17, G21). Its review layers are disabled by override. |
| Hand-back | Every process skill gets an `on_complete` override that writes its result to the active process memlog and hands control back to `sdlc-work` (long G4). |
| Setup | Run manually in a separate session with `bmad-architecture`. `sdlc-work` refuses Plan unless the architecture spine exists and names the stack (long G13). |

Verified facts: `gh` token has the `project` scope; Project v2 #4 has the statuses Backlog, Triage, Define, In Progress, Review, Done; BMad is 6.12.4; `project_knowledge` points to `docs/`; `.memlog.md` files are no longer ignored (done).

## Units

Status: **ready** or **blocked (stack)**.

### U1. GitHub repository setup — ready

- Create labels `route:short`, `route:bug`, `route:long`, `epic`.
- Create a ruleset on `main`: no direct push, PR required, required status checks (`pr-title`, then the checks of U9 once they exist).
- Enable the Project v2 built-in workflows "Item closed → Done" and "Pull request merged → Done" (long G24).
- Acceptance: `gh label list` shows the four labels; a direct push to `main` is refused; closing a test issue moves it to Done.
- Depends on: nothing. Required checks are extended after U9.

### U2. `docs/development-process.md` — ready

- Full process, condensed from `decisions.md`, the three `sequence-*.md`, the three simulations and the decisions above: routes, gates and their aids, two-failure rule, frozen tests, testing without Scaleway (`@manual`), GitHub mapping, Project v2 statuses, migration files, release.
- Includes the three corrected sequence diagrams from the brainstorm folder (`sequence-*-process.md`, updated with the simulations). In particular, bug tests are frozen before the builders (bug G1), which supersedes `decisions.md`.
- Triage criteria marked open.
- Only what a future reader needs; no history of the brainstorm.
- Acceptance: `make lint-docs` green; every string constant (labels, statuses, paths, skill names, `Migration: no impact`) matches the code of the other units (coherence check at review).
- Depends on: nothing. Revisited at the end of the epic for consistency.

### U3. Custom BMad module skeleton `mathieubodin-sdlc` — ready

- Module definition under `skills/`, the two skill folders (`sdlc-work`, `sdlc-gate-page`) with minimal `SKILL.md`, and `module-help.csv` rows so that `bmad-help` recommends `sdlc-work` to start or resume work. Workflows are not part of the module.
- Install with `npx bmad-method install --custom-source <path>`; record the exact command in `docs/development-process.md`.
- Check first: the module layout expected by `--custom-source` on 6.12.4; whether issue #2869 (quick-update of local custom modules) still applies.
- `_bmad/_config/bmad-help.csv` is never edited by hand.
- Acceptance: after install, `_bmad/_config/bmad-help.csv` contains the module rows and the two skills are listed in `.claude/skills/`; a reinstall after a row change updates the catalog.
- Depends on: nothing.

### U4. `_bmad/custom/` common overrides — ready

- `persistent_facts = ["file:{project-root}/docs/development-process.md"]` for the process skills: `bmad-forge-idea`, `bmad-advanced-elicitation`, `bmad-deep-recon`, `bmad-spec`, `bmad-review`, `bmad-architecture`, `bmad-build`, `bmad-walkthrough`. (`bmad-create-epics-and-stories` and `bmad-code-review` are no longer part of the process.)
- `on_complete` on the same skills: when a `sdlc-work` process memlog is active, write the result to it and hand control back to `sdlc-work` (long G4).
- `bmad-forge-idea`: `activation_steps_append` to check the code before each question, and at the end have every recorded assumption validated by yes or no (long G5).
- Acceptance: each skill, when activated, states the process file among its loaded facts; at its end with an active memlog, the memlog has a new entry and `sdlc-work` proposes the next step.
- Depends on: U2.

### U5. `bmad-review` override: spec-lint lens — ready

- One more entry in `workflow.lenses` (`code = "spec-lint"`), applied to specs: every acceptance criterion names its verification method; ambiguities are listed.
- Ambiguity threshold (machine gate): **open, value not set by the brainstorm**. To decide in this unit.
- Acceptance: run on an existing `SPEC.md` with a criterion lacking its verification method; the lens reports it.
- Depends on: U4.

### U6. `bmad-build` override: story specs for Plan — ready

- `bmad-build` is used only during Plan: for each story of `stories.yaml`, it runs steps 01 and 02 and stops at status `ready-for-dev`, producing `{spec_folder}/stories/{id}-{slug}.md` (long G17). Check how to stop it there (override, or the way `sdlc-work` invokes it).
- `review_layers` and `oneshot_review_layers`: empty `instruction`.
- No `implementation_handoff` override: test writers and builders run in `sdlc-rails` and `sdlc-build` (U28, U29).
- Acceptance: on a toy `stories.yaml`, one story spec per story exists in `ready-for-dev` and no code was written.
- Depends on: U4.

### U7. Workflow `sdlc-review` — ready

Replaces the former `bmad-code-review` override, which is dropped.

- `.claude/workflows/sdlc-review.js`. `args`: diff range, spec or bug report path, `dryPasses`, `maxRounds`, `singlePass`.
- Each round: triage then findings (author's global review rule), lenses security, correctness and coherence (coherence mandatory: string constants between code and docs; actual content of security and config files); every finding reproduced by a verifier before it is kept; dedup against all findings seen; a fixer agent applies verified findings and runs `make`.
- A pass is dry only if at least three distinct lenses found nothing. Stops after `dryPasses` dry passes, or returns the recurring findings when `maxRounds` is reached.
- Acceptance: harness tests (U26) for stop after 2 dry passes, return at round 5, single pass; a real run on a diff with a deliberate label mismatch between code and docs reports it and the verifier keeps it.
- Depends on: U26, U8.

### U8. `make`: skeleton, docs lint, frozen tests — ready

- `Makefile` with `help`; `lint-docs` (`markdownlint-cli2`, `.markdownlint-cli2.yaml`). Paths covered by the lint (in particular `_bmad-output/`): **open**, to decide in this unit.
- `freeze`: records the sha256 of the given test files (code tests and Gherkin features) in a committed manifest; run only after a human decision.
- `check-frozen`: recomputes and fails on any difference.
- Variable `MUTATION` (on by default, `MUTATION=0` switches it off), used by U12.
- Shell only, no Python.
- Acceptance: editing a frozen file makes `make check-frozen` fail; `make freeze` then makes it pass.
- Depends on: nothing.

### U9. `docs/migrations/next/` and its checks — ready

- Folder `docs/migrations/next/`, format of an entry, exact no-impact content `Migration: no impact`.
- `make check-migration ISSUE=<n>`: fails unless `docs/migrations/next/<n>.md` is added or changed on the branch.
- Every PR must reference its issue (`Closes #<n>`); CI derives `<n>` from the PR body and fails when absent.
- Acceptance: a branch without the file fails; with it, passes.
- Depends on: U8. From the merge of this unit on, every PR of the epic carries its migration file.

### U10. `make evidence` — ready

- Replays the checks available (`lint-docs`, `check-frozen`, `check-migration`, and the U12 to U14 targets once they exist) and rewrites the whole record in `_bmad-output/work/issue-{n}-{slug}/evidence.md`, whoever runs it.
- Acceptance: two consecutive runs on an unchanged tree give the same record, except timestamps.
- Depends on: U8, U9.

### U11. PR CI checks — ready

- Workflow `pr-checks.yml` on every PR: `make lint-docs`, `make lint-workflows`, `make test-workflows`, `make check-frozen`, `make check-migration`. Code tests added by U12.
- The behavior diff stays local.
- Add the jobs to the required checks of the U1 ruleset.
- Acceptance: a PR breaking each check fails on the right job.
- Depends on: U8, U9, U1, U26.

### U12. `make`: code tests, coverage, mutation testing — blocked (stack)

- Coverage measured by a tool; mutation testing on the code changed by the task, switched off by `MUTATION=0`. Code tests added to `pr-checks.yml`.
- Depends on: stack choice, U8, U11.

### U13. `make`: behavior diff — blocked (stack)

- playwright-bdd, ARIA snapshots as the main capture, scenarios replayed before and after the change against the fake operator in kind; `@manual` scenarios never run; the accepted capture becomes the reference.
- Depends on: stack choice, fake operator, U8.

### U14. `make`: manual cluster for the human — blocked (stack)

- Builds the image and Helm chart locally, creates a kind cluster, installs the real operator and the local chart. Runs no scenario.
- Depends on: stack choice.

### U15. "Deliver" workflow: version, tag, release note — ready

- Triggers: `milestone` `closed` (version = milestone title) and `workflow_dispatch` with a `tag` input. One branch point for the version origin, then a common trunk.
- Checks: valid semver; tag does not exist; no open issue in the milestone (when one has that title).
- Creates the tag and the GitHub release. Release note from the migration files changed since the previous tag: only no-impact entries → a single "no impact" entry; otherwise no-impact entries dropped.
- `concurrency` group preventing two deliveries at once.
- To check: reading the milestone title from the event (`github.event.milestone.title` expected).
- The workflow must be on `main` to run; the first real run creates a real tag and release: the human chooses when.
- Acceptance: a dispatch with an existing tag or an invalid semver fails before creating anything.
- Depends on: U9.

### U16. "Deliver": build and publish to ghcr.io — blocked (stack)

- Build the image and the Helm chart, publish to `ghcr.io` (chart as OCI, `oci://ghcr.io/<owner>/charts`), add to the release note the command that reinstalls the previous version.
- Depends on: stack choice, U15.

### U17. Skill `sdlc-work`: start and triage — ready

- Without argument: questions, route proposal (risk × blast radius) and optional milestone proposal, human confirmation, issue creation with its route label (`epic` for long work), add to Project v2 with its status, work branch `{type}/{n}-{slug}`, memlog init (long G1, G3).
- A chosen milestone that does not exist is created after confirmation: title checked as semver, theme asked and written in the description (long G2).
- With an external issue number not yet triaged: reads it and triages it; for a bug, writes `bug-report.md` from it (bug G3).
- Rerouting to the long route: same issue promoted (`route:long` + `epic`, status Define, comment "rerouted: reason"), branch and memlog kept, branch renamed if its type changes (short G4).
- Python allowed for internal scripts (for example Project v2 status updates).
- Acceptance: on a test idea, the issue, label, Project status, branch and memlog exist and agree.
- Depends on: U3, U1, U2.

### U18. Skill `sdlc-work`: resume — ready

- `/sdlc-work <n>`: reads the local memlog, the Project v2 status, the gate comments on the epic and its sub-issues; states the step reached and proposes the next one. Without a local memlog, resumes from the last gate recorded on GitHub.
- Acceptance: resume on the same machine and from a fresh clone gives the same step for an issue past its spec gate.
- Depends on: U17.

### U19. Skill `sdlc-work`: short and bug routes — ready

- Short route (`simulation-short.md`): change by the session; `sdlc-review` with `singlePass`, unfixed findings listed in the PR (short G2); behavior diff empty for a post-integration simplification, otherwise posted and accepted by the merge (short G3).
- Bug route (`simulation-bug.md`): `sdlc-rails` in bug mode (test writer reading the code, writing tests only, bug G5); failure shown next to the reported symptom and confirmed by the human (bug G2); `make freeze` before any builder (bug G1); `sdlc-build` in bug mode (two-failure rule, `bug-report.md` enriched, bug G3), merged by `sdlc-work`; `sdlc-review` with 1 dry pass, at most 2 rounds (bug G4); bug only reproducible against real Scaleway: fake operator first, otherwise `@manual`, or move to `scaleway-operator` (bug G6).
- Both: migration file, `make evidence`, PR opened with evidence posted, status Review; Done comes from the Project workflows (U1).
- Behavior diff step skipped until U13 exists, and the skip is written in the evidence record.
- Depends on: U17, U1, U7, U8, U9, U10, U28, U29.

### U20. Skill `sdlc-work`: long route — ready

Follows the "Resulting sequence" of `simulation-long.md`:

- Define: `bmad-forge-idea` → workflow `sdlc-parallax` → pre-mortem → research if proposed and accepted (long G8) → answers in `define-notes.md` (long G7) → `bmad-spec` (inputs `forged-idea.md` and `define-notes.md`) → spec-lint loop, alert after 3 rounds (long G9) → `spec.html`.
- Gates: `xdg-open` on the page (long G12); on validation, commit the work folder and the folders referenced by the memlog, push, comment with the commit link (long G11); on invalidation, propose a return step, the human chooses (long G10).
- Setup guard before Plan (long G13).
- Plan: reread the spine, `docs/solutions/`, previous specs and MemPalace (long G14); workflow `sdlc-explore`; write `work/…/plan.md` (long G15); Story Breakdown of `bmad-spec` (long G16); story specs through U6; cold-start loop (workflow `sdlc-cold-start` per round, questions and fixes in `sdlc-work`, alert after 3 rounds, long G18); `plan.html`; after the plan gate, one sub-issue per story spec.
- Rails: workflow `sdlc-rails` (one test writer per story, blind to the code, then a single `make freeze`), commit by `sdlc-work`, no human check (long G19, G20).
- Build: workflow `sdlc-build` for the trunk, merge by `sdlc-work`, then `sdlc-build` for the leaves, merge one by one with `make check-frozen` after each (long G21). A "back to Plan" return value sends the work back to Plan and its gate.
- Review: workflow `sdlc-review` (2 dry passes, at most 5 rounds, result in the memlog); on return at the cap, proposal and human choice (long G23).
- Before the PR: `/ce-compound`, MemPalace update, after-integration issues "simplify after #n" and, when relevant, "distill lessons" (short G1), migration file, evidence.
- PR body with `Closes` for the epic and every sub-issue; sub-issues follow the epic's status (long G22).
- Depends on: U17, U18, U5, U6, U7, U21, U22, U23, U27, U28, U29.

### U21. Workflow `sdlc-parallax` — ready

- `.claude/workflows/sdlc-parallax.js`. `args`: path of `forged-idea.md`, number of agents (3 to 5).
- Each agent writes the spec it would execute from the same input; a synthesis agent returns every divergence as a question for the human (structured output).
- Acceptance: harness test of the control flow; a real run on an intentionally ambiguous input reports at least one divergence.
- Depends on: U26.

### U22. Workflow `sdlc-cold-start` — ready

- `.claude/workflows/sdlc-cold-start.js`. `args`: list of story spec paths.
- One fresh agent per story spec, with only that file; returns what each one is missing (structured output). One round per run; the loop and the questions stay in `sdlc-work`.
- Acceptance: harness test; a real run on a story spec missing its file paths reports it.
- Depends on: U26.

### U23. Skill `sdlc-gate-page` — ready

- Produces `spec.html` and `plan.html` in the work folder from the reference Markdown, to help the human understand; the Markdown stays the reference.
- Depends on: U3.

### U24. `AGENTS.md` "Development process" section — ready

- A few lines: entry point `/sdlc-work`, link to `docs/development-process.md`, hard rules (no direct push to `main`; frozen tests change only on a human decision; no automated test touches Scaleway, `@manual` scenarios are played by the human; every PR adds `docs/migrations/next/<issue>.md`).
- "Running and verifying": point to `make help`.
- Acceptance: the file grows by at most the section; no rule duplicated from `docs/development-process.md` beyond the hard rules.
- Depends on: U2, U8, U17.

### U25. Spike: base of `isolation: 'worktree'` — ready

- Check from which ref a workflow agent's worktree starts (`main` or the session's HEAD).
- If not HEAD: `sdlc-build` receives the work branch SHA in `args` and each builder starts with `git reset --hard <sha>` in its worktree.
- Acceptance: the observed base is written in `docs/development-process.md` with the chosen approach.
- Depends on: nothing.

### U26. Workflow tooling — ready

- Biome (`biome.json`), `make lint-workflows` (lint and format check of `.claude/workflows/`).
- `node:test` harness that loads a workflow script and provides fake `agent`, `parallel`, `pipeline`, `phase`, `log` and `args` with scripted answers; `make test-workflows` with coverage by `c8`.
- Acceptance: a deliberately broken script fails `make lint-workflows`; a sample script reaches 100 % branch coverage under the harness.
- Depends on: U8.

### U27. Workflow `sdlc-explore` — ready

- `.claude/workflows/sdlc-explore.js`. `args`: spec path, explorations to run (code, practices, frameworks).
- Parallel readers, one per exploration; returns a structured map for `plan.md`.
- Acceptance: harness test; a real run on an existing spec returns the three sections.
- Depends on: U26.

### U28. Workflow `sdlc-rails` — ready

- `.claude/workflows/sdlc-rails.js`. `args`: mode (`long` or `bug`), story spec paths or `bug-report.md`.
- Long mode: one test writer per story, given the story spec only, never the code; then an agent runs `make freeze` for the written tests.
- Bug mode: one test writer that reads the code and writes tests only; an agent runs the new tests; no freeze, because the human must first confirm the failure reason (bug G2); `sdlc-work` runs `make freeze` after that confirmation.
- Returns the test files written and, in bug mode, each test's failure message.
- Acceptance: harness test; in a real long-mode run, the test writers' prompts contain no code path (checked in `/workflows`).
- Depends on: U26, U8.

### U29. Workflow `sdlc-build` — ready

- `.claude/workflows/sdlc-build.js`. `args`: mode (`long` or `bug`), story spec paths or `bug-report.md`, work branch SHA (U25).
- One builder per item with `isolation: 'worktree'`, given the story spec (or `bug-report.md`) and the frozen tests. Two-failure rule in script: after a failure, an agent improves the ticket (or enriches `bug-report.md`) and a new builder starts; after a second failure, the item is returned as "back to Plan" (long) or "ask the human" (bug).
- Returns the finished branches and the failed items; merging is done by `sdlc-work`.
- Acceptance: harness tests for success, one failure then success, two failures; a builder that edits a frozen test fails `make check-frozen`.
- Depends on: U25, U26, U8.

## Order

```text
Trunk:  U1, U2, U3, U8, U25  (independent, can start in parallel)
        → U4, U9, U26 → U10, U11, U15
        → U5, U6, U7, U21, U22, U23, U27, U28, U29  (leaves, parallel)
        → U17 → U18 → U19 → U20
        → U24, final pass on U2
Blocked (stack): U12, U13, U14, U16
```

## Open in this plan

- Ambiguity threshold of the spec-lint gate (U5).
- Paths covered by the docs lint (U8).
- Worktree base for builders (U25) and milestone title field (U15): to verify.
- How to stop `bmad-build` at `ready-for-dev` (U6).
