---
topic: Walk-through of the long route on a concrete case, to find what is not yet executable
status: draft
created: 2026-10-18
case: Export namespace resources as YAML and re-import them into a new kind cluster (epic #27, stories #28 to #31)
inputs:
  - _bmad-output/brainstorming/brainstorm-sdlc-local-workflow-2026-10-16/sequence-long-process.md
  - plan.md
---

# Simulation of the long route

This file walks `sequence-long-process.md` step by step on the case above, as written in the diagram at commit `081611a`, and marks where a step could not be executed (`G1`, `G2`…). The gaps and the decisions taken with the author follow. The decisions are carried into the sequence diagram and `plan.md`; not yet into the GitHub issues.

## Walk-through

### Start

| # | Who | What happens | Produced | GitHub |
| --- | --- | --- | --- | --- |
| 1 | Human | Types `/sdlc-work`, then "export the resources of a namespace as YAML and re-import them into a new kind cluster". | | |
| 2 | `sdlc-work` | Asks a few questions, one at a time (no code to check yet). | | |
| 3 | `sdlc-work` | Proposes the long route ("new feature, touches the adoption rule: duplicates in Scaleway if wrong, high blast radius") and a milestone `v0.1.0`, which does not exist. **G2**: who creates it? | | |
| 4 | Human | Confirms the route, chooses the milestone. | | |
| 5 | `sdlc-work` | Creates the epic, the branch and the process memlog. **G1**: branch name undefined. **G3**: slug chosen without a rule. | Branch `?`; `work/issue-27-yaml-export/.memlog.md` (local, not committed) | Issue #27, labels `route:long` + `epic`, status Define |

### Define (status Define)

| # | Who | What happens | Produced | GitHub |
| --- | --- | --- | --- | --- |
| 6 | `bmad-forge-idea` | Explores the problem with personas, asks questions. At its end, forge recommends its own next step. **G4**: nothing makes `sdlc-work` take control back. | `_bmad-output/forge/yaml-export/forged-idea.md` + memlog | |
| 7 | ? | "Then questions" (step 2) and "validate assumptions" (step 3). **G5**: no skill assigned; overlaps with forge. | | |
| 8 | `sdlc-parallax` | 3 to 5 fresh subagents each write the spec they would execute from `forged-idea.md`; divergences become questions; the human answers. **G7**: where are the answers written? | | |
| 9 | `bmad-advanced-elicitation` | Pre-mortem on `forged-idea.md`; risks become questions; the human answers. Same **G7**. | | |
| 10 | `bmad-deep-recon` | "If research needed". **G8**: who decides? | | |
| 11 | ? | Scope with rejections (step 7), success criteria (step 8). **G6**: no skill assigned. | | |
| 12 | `bmad-spec` | Writes the spec. Same **G7**: nothing passes it the parallax and pre-mortem answers. | `_bmad-output/specs/spec-yaml-export/SPEC.md` + memlog | |
| 13 | `bmad-review` (spec-lint lens) | Reports for example "capability re-import: no verification method; ambiguity: what if the namespace already exists?". **G9**: what happens when it fails? | Report | |
| 14 | `sdlc-gate-page` | Renders the gate page. | `work/issue-27-yaml-export/spec.html` | |

### Spec gate

| # | Who | What happens | Produced | GitHub |
| --- | --- | --- | --- | --- |
| 15 | `sdlc-work` | "Spec gate: open spec.html; the reference is SPEC.md." **G12**: how does the human open it? | | |
| 16 | Human | Validates, or invalidates with a reason. **G10**: after an invalidation, where does the work go back to? | | |
| 17 | `sdlc-work` | Commits the spec files, comments on the epic with the commit link. **G11**: which files? The link only works once the branch is pushed, and no push is planned. | Commit | Comment "validated" on #27; status In Progress |

### Plan (status In Progress)

| # | Who | What happens | Produced | GitHub |
| --- | --- | --- | --- | --- |
| 18 | `sdlc-work` | Rereads the project memory. **G14**: what is it? | | |
| 19 | `bmad-architecture` | Approaches and trade-offs, explorations (code, practices, frameworks) in subagents. **G13**: no stack is chosen; Setup is outside the plan, so tasks of code cannot be split. **G15**: this skill writes the project architecture document, not a per-work plan. | `planning-artifacts/architecture/architecture-{project}-{date}/` | |
| 20 | `bmad-create-epics-and-stories` | Splits into tasks, contracts, trunk and leaves. **G16**: this skill writes the project epic list, not one epic's tasks. | `planning-artifacts/epics.md` | |
| 21 | `sdlc-cold-start` | One fresh subagent per task lists what the task is missing. **G17**: the tasks it tests are not the tickets the builders will receive (`bmad-build` writes its story specs at build time). **G18**: what happens when items are missing? | Report | |
| 22 | `sdlc-gate-page` | Renders the gate page. | `work/issue-27-yaml-export/plan.html` | |

### Plan gate

| # | Who | What happens | Produced | GitHub |
| --- | --- | --- | --- | --- |
| 23 | Human | Validates or invalidates (same **G10**, **G11**, **G12** as the spec gate). | | |
| 24 | `sdlc-work` | Commits, comments, creates one sub-issue per task. | Commit | Comment on #27; sub-issues #28 to #31 |

### Rails, build, verification, review

| # | Who | What happens | Produced | GitHub |
| --- | --- | --- | --- | --- |
| 25 | `bmad-build` (test writer) | A subagent that does not see the code writes the tests from the spec. **G19**: does the human see them before they are frozen? **G20**: per story or all at once? | Gherkin features, code tests | |
| 26 | `make freeze` | Records the fingerprints. | Fingerprint manifest | |
| 27 | `bmad-build` (builders) | One builder per task, in its own worktree. **G21**: `bmad-build` drives the session one story at a time; it cannot run parallel leaves. | Code | |
| 28 | `make` | Checks, coverage, mutation, fingerprints. | | |
| 29 | `bmad-code-review` | Loop until two dry passes with 3 lenses, at most 5 rounds. **G23**: what happens after 5 rounds? | Verified findings | |
| 30 | `/ce-compound` | Records the lessons. | `docs/solutions/…` | |
| 31 | `sdlc-work` | Writes the migration file, runs the evidence record and the behavior diff. | `docs/migrations/next/27.md`; `work/issue-27-yaml-export/evidence.md` | |
| 32 | `sdlc-work` | Opens the PR ("Closes #27"), posts evidence, behavior diff, lessons. **G22**: when do #28 to #31 close, which status do they carry? | | PR; status Review |

### Delivery gate

| # | Who | What happens | Produced | GitHub |
| --- | --- | --- | --- | --- |
| 33 | Human | Plays the Gherkin scenarios (`bmad-walkthrough` as aid), approves and merges. | | Merge |
| 34 | `sdlc-work` | Sets the status to Done. **G24**: the session may be gone when the human merges in GitHub. | | Status Done |

## Gaps and decisions

| ID | Gap | Decision |
| --- | --- | --- |
| G1 | Branch name undefined | `{type}/{n}-{slug}`, type = the Conventional Commits type planned for the PR. Example: `feat/27-yaml-export`. |
| G2 | Nobody creates a missing milestone | `sdlc-work` creates it after the human's confirmation; title checked as valid semver; theme asked to the human and written in the description. |
| G3 | Slug chosen without a rule | Derived from the issue title, kebab-case, at most 4 words, shown in the same confirmation as the route; editable. |
| G4 | A called skill drives the session and recommends its own next step | An `on_complete` override on every process skill: "if a `sdlc-work` process memlog is active, write the result to it and hand control back to `sdlc-work`". If the session drifted or stopped, `/sdlc-work <n>` resumes from the memlog. |
| G5 | Define steps 2 and 3 without a skill | `bmad-forge-idea` covers steps 1 to 3, with an override: check the code before each question; at the end, have every recorded assumption (forge already logs them with `--type assumption`) validated by yes or no. |
| G6 | Define steps 7 and 8 without a skill | `bmad-spec` covers them (`Non-goals` and `Success signal` sections of its template). |
| G7 | Where parallax and pre-mortem questions and answers are written; how `bmad-spec` receives them | `_bmad-output/work/issue-{n}-{slug}/define-notes.md`, sections Parallax, Premortem, Research, each question with its answer. `bmad-spec` gets two inputs: `forged-idea.md` and `define-notes.md`. |
| G8 | Who decides that research is needed | `sdlc-work` proposes it when `define-notes.md` holds an external factual question without answer; the human decides yes or no. A skipped research is recorded. |
| G9 | Spec-lint fails | Each finding becomes a question for the human; the answer goes to `define-notes.md`; `bmad-spec` updates `SPEC.md`; spec-lint runs again. No gate page while spec-lint is red. Alert to the human after 3 rounds. |
| G10 | Return point after "invalidated: reason" | `sdlc-work` proposes a step from the reason (forge if the problem is ill-posed, parallax or pre-mortem if a risk is missing, `bmad-spec` or the plan if it is the writing); the human chooses. |
| G11 | Files committed at a gate; push needed for the commit link | Commit the whole work folder (process memlog included) and the folders referenced by the memlog (forge, spec), then push the branch, then post the comment with the commit link. Also enables resuming from another machine from the last gate. |
| G12 | How the human opens the HTML page | Local file: `sdlc-work` prints the absolute path and runs `xdg-open`. |
| G13 | No stack, Setup outside the plan | Setup is run manually in a separate session with `bmad-architecture`. `sdlc-work` refuses to start Plan unless the architecture spine (`_bmad-output/planning-artifacts/architecture/…`) exists and names the stack. No Setup skill. |
| G14 | "Project memory" undefined | The architecture spine, `docs/solutions/`, previous `SPEC.md` files under `_bmad-output/specs/`, and a MemPalace search in the `scaleway_manager` wing. |
| G15 | `bmad-architecture` writes the project architecture, not a per-work plan | `sdlc-work` writes `_bmad-output/work/issue-{n}-{slug}/plan.md`: memory reread, explorations (subagents), 2 or 3 approaches with trade-offs, choice, contracts at each boundary, sequential trunk and parallel leaves. `bmad-architecture` is called only when the choice changes the spine. |
| G16 | `bmad-create-epics-and-stories` writes the project epic list | Not used. The split comes from the Story Breakdown of `bmad-spec` (`stories.yaml` next to `SPEC.md`). |
| G17 | Cold-start and the gate do not see the builders' tickets | During Plan, `sdlc-work` runs `bmad-build` for each story of `stories.yaml` until status `ready-for-dev` (steps 01 and 02, no implementation), producing `{spec_folder}/stories/{id}-{slug}.md`. Cold-start tests these files, the plan gate validates them, each sub-issue carries the content of its story spec. |
| G18 | Cold-start reports missing items | Same rule as spec-lint: fixes to `plan.md`, `stories.yaml` or the story specs (questions to the human when needed), cold-start runs again, no gate page while red, alert after 3 rounds. |
| G19 | Human check of the tests before freezing | None. Tests are frozen directly. A wrong test shows during the build (two-failure rule) or at the delivery gate; changing it needs the human's decision. |
| G20 | When tests are written and frozen | For all stories before any build: one test-writer subagent per story, then a single `make freeze` and a commit on the work branch. The fingerprint manifest does not change during the build, so worktree merges do not conflict on it. |
| G21 | Parallel leaves need several builders | `sdlc-work` launches the builders itself: one subagent per story in an isolated worktree started from the work branch, with the story spec and the frozen tests; leaves in parallel; merged one by one; story spec status updated. `bmad-build` is used only during Plan; its `implementation_handoff` override (U6) is no longer needed. |
| G22 | When sub-issues close, which Project status they carry | The PR body lists `Closes` for the epic and every sub-issue: all close at the merge into `main`. In the Project, sub-issues follow the epic: In Progress when the build starts, Review when the PR opens, Done at the merge. |
| G23 | Review loop not converging after 5 rounds | `sdlc-work` shows the recurring findings and proposes: back to Plan (bad split), back to Define (contradictory spec), or accept the remaining findings and list them in the PR. The human chooses. |
| G24 | Status Done after the human merges, when no session is running | Project v2 built-in workflows "Item closed → Done" and "Pull request merged → Done" (project setting, no GitHub Actions). |

## Resulting sequence

Every called skill hands control back to `sdlc-work` through `on_complete` (G4).

1. Start: questions, route, milestone (created if missing), slug, branch `feat/27-yaml-export`, memlog; status Define.
2. Define: `bmad-forge-idea` (questions with code checks, assumptions validated) → `sdlc-parallax` → pre-mortem → research if accepted → answers in `define-notes.md` → `bmad-spec` → spec-lint loop → `spec.html`.
3. Spec gate: `xdg-open spec.html`; decision; commit, push, comment; status In Progress. On invalidation, return step proposed and chosen.
4. Setup guard: the architecture spine exists and names the stack.
5. Plan: memory reread → `work/…/plan.md` (explorations, approaches, choice, contracts, trunk and leaves) → `stories.yaml` → story specs up to `ready-for-dev` → cold-start loop → `plan.html`.
6. Plan gate: same as the spec gate; then one sub-issue per story spec.
7. Rails: one test writer per story, single `make freeze`, commit.
8. Build: builders in parallel worktrees, merged one by one; sub-issues In Progress.
9. Verification and review loop (alert and proposal after 5 rounds).
10. `/ce-compound`, after-integration issues (see `simulation-short.md`), migration file, evidence, behavior diff.
11. PR with `Closes` for the epic and all sub-issues; status Review.
12. Delivery gate: the human plays the scenarios and merges; the Project workflows set Done.

## Impact on the plan, to carry later

- U1: enable the Project v2 built-in workflows.
- U4: add the `bmad-forge-idea` override and the `on_complete` hand-back on every process skill.
- U6: rewrite; `bmad-build` serves Plan (story specs up to `ready-for-dev`), not the build. Disabling its review layers may become moot.
- U17: branch name, slug rule, milestone creation.
- U20: rewrite Define, gates, Plan, rails and build as above; Setup guard.
- `sequence-long-process.md`: diagram updated (done).
