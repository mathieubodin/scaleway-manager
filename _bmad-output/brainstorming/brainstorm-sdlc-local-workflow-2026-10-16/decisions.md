---
topic: Local work workflow based on the SDLC, then mapping to GitHub items
status: complete
created: 2026-10-16
updated: 2026-10-17
---

# SDLC-based local workflow: decisions

Work order set by the author: 1) define the SDLC, 2) build the local workflow, 3) map each step to GitHub items, 4) define how the process materializes for agents. GitHub was not discussed before step 3.

## Sources

- SFEIR, [Un SDLC piloté par l'IA : le cycle SFEIR à 11 phases](https://www.sfeir.com/articles/sdlc-ai/)
- SFEIR, [SDLC augmenté par l'IA](https://www.sfeir.com/concepts/sdlc-augmente/)
- SFEIR, [Agentic Engineering](https://www.sfeir.com/concepts/agentic-engineering/)
- SFEIR, [SDLC IA et platform engineering](https://www.sfeir.com/articles/sdlc-ia-platform-engineering/)
- SFEIR, [L'amont : Define et Plan](https://www.sfeir.com/articles/sdlc-ai-amont-define-plan/)
- SFEIR, [ADLC](https://www.sfeir.com/methodologies/adlc/)
- Chris Williams, [ADLC.md](https://github.com/voodootikigod/adlc/blob/main/ADLC.md); READMEs of [behavior-diff](https://github.com/voodootikigod/adlc/blob/main/packages/behavior-diff/README.md), [gate-manifest](https://github.com/voodootikigod/adlc/blob/main/packages/gate-manifest/README.md) and [prosecute](https://github.com/voodootikigod/adlc/blob/main/packages/prosecute/README.md)
- Claude Code docs: [sandboxing](https://code.claude.com/docs/en/sandboxing), [subagents](https://code.claude.com/docs/en/sub-agents), [worktrees](https://code.claude.com/docs/en/worktrees)
- GitHub docs: [manually running a workflow](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/manually-run-a-workflow), [events that trigger workflows](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows), [Actions billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)
- [playwright-bdd](https://github.com/vitalets/playwright-bdd), [Playwright ARIA snapshots](https://playwright.dev/docs/aria-snapshots)

The SFEIR pages and `ADLC.md` were read through a tool summary, not as raw text.

## SDLC definition

> The SDLC is the ordered sequence of phases that takes an intent to delivered, then maintained, software. In its agentic form, agents do the work between three human gates (spec, plan, delivery), each supported by a decision-aid process, and automated checks. Each cycle enriches the memory of the next one.

## Overview

```text
[Setup]  once per project, outside the cycle
   └─ ▸ Setup gate

CYCLE (one per piece of work)
   Triage (agent proposes, human confirms)
     → short:        change → make → 1 review pass → ▸ light gate
     → intermediate: bug test(s) → fix → review → ▸ gate → tests frozen
     → long:         Define ─▸ spec gate
                     Plan ─▸ plan gate
                     rails → build → verification → review → lessons
                       ─▸ delivery gate (= PR)
                     after integration
   Integration into main through a PR (no direct push)

[Release]  milestone closure or "Deliver" button, in GitHub
```

## Gates

- Three human gates in the cycle: spec, plan, delivery. The plan has its own gate (SFEIR), whereas the ADLC has two gates.
- Every gate comes with a process that helps the human decide.

## Setup

- Runs once per project, outside the cycle.
- Produces the decisions that define the project (stack, content of the project memory, etc.) and initializes the project memory.
- Everything is recorded in files, probably Markdown.
- Decisions stay open: they can be revisited, accepting the consequences on deliverables already produced.
- Driven by a skill that questions the human inside the session (not a subagent: a subagent cannot talk with the human).
- Setup gate: the last step of that conversation. Aid: a visual HTML page that presents the decisions and places them in the whole project.

## Triage

- The cycle starts with a triage that routes the work to a process: short, intermediate or long.
- Who triages: the agent proposes a justified route, the human confirms. Work that turns out bigger than expected is rerouted to the long process.
- Triage criteria: not defined yet. ADLC lead: four lanes (trivial, bounded, substantial, architectural), chosen by risk × blast radius.

### Short process

- For example a typo, or a code simplification after integration.
- One automated review pass, without a loop, before the human gate (ADLC "trivial" lane).
- Ends with a light human gate: the human reviews the proposed change and the record produced by `make`, then approves integration. If the change is bigger than expected, the human can take more time, or replay the Gherkin scenarios.

### Intermediate process (bugs)

- ADLC "bounded" lane. No spec, no plan.
- First write the test that reproduces the bug (it fails), then fix until it passes without breaking the frozen tests; lighter review; human gate before integration.
- Bug visible in the UI: a Gherkin scenario, plus a code test when relevant (the agent proposes, the human confirms). Invisible bug: a code test only.
- Once the bug is fixed, its tests join the frozen tests, with their fingerprint recorded.
- Two-failure rule applies: after two failures, stop the agent, improve the bug report, start a new agent. If the new agent fails too, hand back to the human and ask whether to switch to the long process.

## Define (long process)

Ten steps:

| # | Step | Source |
| --- | --- | --- |
| 1 | Explore the problem | Project forge |
| 2 | Question the human, checking the code before each question | ADLC P1 |
| 3 | Make assumptions explicit and have them validated | SFEIR |
| 4 | Parallax: 3 to 5 fresh agents each write the spec they would execute; every divergence becomes a question | ADLC P1 |
| 5 | Premortem: "three months from now the project failed, write the postmortem"; every risk becomes a question | ADLC P1 |
| 6 | External research | Brief addendum |
| 7 | Set the scope, keep track of rejections | Brief and forge |
| 8 | Success criteria | Brief |
| 9 | Write the spec; every acceptance criterion names its verification method | SFEIR and ADLC |
| 10 | Spec-lint and ambiguity threshold (machine gate) | ADLC P1 |

Spec gate ("Is this what I meant?"):

- a visual HTML page, to understand;
- a Markdown document, the reference for what is proposed;
- the gate decision validates or invalidates the Markdown content.

## Plan (long process)

Seven steps, as a starting base:

| # | Step | Source |
| --- | --- | --- |
| 1 | Reread the project memory | SFEIR |
| 2 | Explore the existing code | SFEIR |
| 3 | Explore practices | SFEIR |
| 4 | Explore frameworks | SFEIR |
| 5 | Propose 2 or 3 approaches with their trade-offs | SFEIR, ADLC |
| 6 | Split into tasks, with the contract at each boundary; mark the sequential trunk and the parallelizable leaves | ADLC P2 |
| 7 | Cold-start test: a fresh agent lists what each task is missing (machine gate) | ADLC P2 |

Explorations 2 to 4 run in parallel. The task split goes through the plan gate.

Plan gate: same scheme as the spec gate (HTML + reference Markdown, validated or invalidated).

## Rails, build, verification, review (long process)

- Tests are written before the code, from the spec, by an agent that does not see the implementation (ADLC P3, "rails").
- Tests are frozen by detection, without a lock: their fingerprint is recorded when they are written; the make target recomputes it and fails if a test changed (rails-guard principle, ADLC). Any change to a test requires the human's decision, then the fingerprints are recomputed.
- Build: one fresh agent per task, which receives only the ticket and the frozen tests (ADLC P4).
- Build failure: after two failures, stop the agent; improve the ticket and start a new agent; if it fails too, go back to the plan (and its gate). No switch to a more powerful model.
- Verification: coverage measured by a tool (author's global rule) and mutation testing on the code changed by the task (ADLC). Mutation testing is a trial and must be easy to switch off (for example a Makefile variable).
- Review: the author's global review rule is the base (triage then findings; security, correctness and coherence reviewers; actual content of security and config files; string consistency between code and docs). Two ADLC additions (P5):
  - every finding is reproduced by a verifier before it is kept;
  - loop until two consecutive passes without a verified finding, within a maximum number of rounds after which the human is alerted. Maximum: 5 rounds, an adjustable starting value (assistant's proposal; the sources give no number). A pass counts as dry only if at least three distinct lenses found nothing (prosecute README). A loop that does not converge signals a contradictory spec or a bad split (ADLC principle 8).
- Lessons before the delivery gate: review lessons are recorded with `/ce-compound` (author's global rule, SFEIR Compound-1). Code simplification is deferred until after integration.

## Delivery gate (long process)

In the cycle, delivering means integrating the work into `main`. Publishing the image and Helm chart is the release (see below).

This gate validates a behavior and evidence, not a document. Three elements:

1. **Evidence record.** A make target replays the checks and rewrites the whole record, whoever runs it.
2. **Behavior diff.**
   - It covers the rendered UI, as the user handles it, not the backend API. Its goal: detect that the tool drifts from its intended behavior.
   - The Gherkin scenarios are replayed automatically in a browser, before and after the change.
   - Tool: playwright-bdd, with Playwright ARIA snapshots (page structure) as the main capture. Screenshots may be added later.
   - The accepted capture becomes the reference for the next delivery.
3. **Run by the human** of the Gherkin scenarios, written to be playable by hand.

## After integration (long process)

Three steps, as a trial (ADLC P7, distill):

1. Simplify the code (duplicates, dead code, extractions) under the protection of the frozen tests.
2. Turn recurring lessons into rules: an automated check, or a question added to Define.
3. Update the project memory, reread by Plan in the next cycle.

The simplification changes code after the delivery gate: it goes through the short process, provided the frozen tests stay green and the behavior diff is empty.

## Testing without Scaleway

- No automated test run or driven by an AI may cause cost at Scaleway. Scenarios that create real Scaleway resources are played by hand by the human only. Automated tests do not leave the local machine.
- A fake local operator, used only by automated tests, writes the CR `status` (Pending, Syncing, Synced, Error, TerminationBlocked) without ever calling Scaleway. The real operator is not installed in the test cluster.
- The Gherkin tag `@manual` marks scenarios that touch real Scaleway: never run automatically, played by the human. Other scenarios are replayed automatically against the fake operator.
- Limit: the fake operator may drift from the real one; `@manual` scenarios remain the reality check.
- A make target, run by the human, builds the image and Helm chart locally, creates a kind cluster, and installs the real operator and the local chart. The human then plays manual tests against Scaleway. It runs no scenario automatically.

## GitHub mapping

| Workflow | GitHub |
| --- | --- |
| A long piece of work (a feature) | Epic: a parent issue |
| Each task from the Plan split | Sub-issue, used as the fresh agent's ticket |
| A short piece of work or a bug | A plain issue, without sub-issues |
| Several pieces of work forming a release | Milestone |

Constraint: `mathieubodin/scaleway-manager` is a personal repository; issue types (including "Epic") are organization-only. Sub-issues in a personal repository are not confirmed (conflicting sources, not tested). Options: parent issue + `epic` label, or moving the repository to a free organization.

### Starting a piece of work

- Idea from the author or a contributor: the process starts in their Claude session. The session asks questions, proposes the triage, then creates the issue with its label after confirmation. A long piece of work is created as an epic, and Define starts in the same session.
- Issue opened by an external user: the session reads the existing issue and triages it.
- Milestone: the agent may propose one at triage; the human chooses which milestone the issue ends in. An issue may have no milestone.

### Branches and parallel work

- One branch per piece of work.
- Parallel build (ADLC P4): one git worktree per task, started from the work branch (Claude Code setting `worktree.baseRef: "head"`; by default a worktree starts from `main`); one agent per code area; tasks are merged one by one into the work branch, the others rebase; one single PR to `main`.

### Spec and plan gates in GitHub

- The spec, the plan and their HTML pages are committed on the work branch, in `_bmad-output/`.
- The session records the decision as a comment on the epic ("validated" or "invalidated: reason"), with a link to the commit of the Markdown.
- After the plan gate, the session creates the sub-issues, one per task. The epic serves as the work log.

### Delivery gate in GitHub

- The delivery gate is the PR. It is opened once the review is finished and the lessons are recorded.
- The evidence record, the behavior diff (ARIA captures) and the `/ce-compound` result are posted as comments.
- The human plays the Gherkin scenarios, then approves and merges: the merge is the decision.
- The PR references the issue or epic ("Closes #…").
- No direct push to the remote `main` branch.

### Project v2

<https://github.com/users/mathieubodin/projects/4>. Statuses: Backlog → Triage → Define → In Progress → Review → Done ("Planned" renamed "Triage", "Define" added by the author).

- Backlog: issue open, triage not started.
- Triage: triage in progress, or triage finished but work not started. The route label tells the two apart.
- Define: triage finished, long work started, spec in progress until the spec gate.
- In Progress: from Plan to the end of the review; a short piece of work or a bug comes here right after triage.
- Review: PR open, delivery gate pending.
- Done: PR merged.

The session updates the status at each step.

### CI

- Lightweight CI to start with: on every PR, the fast checks (lint, code tests, fingerprints of frozen tests). The behavior diff (kind, fake operator, browser) stays local.
- The repository is public: standard GitHub-hosted runners are free. The real cost is waiting time.

### Release

- Versions follow semver.
- Image on the GitHub container registry (ghcr.io). Helm charts as OCI on ghcr.io, as `scaleway-operator` already does (`oci://ghcr.io/<owner>/charts`).
- Closing a milestone triggers a release. A patch release (bug fix) must be possible without a milestone: a PR with a `fix:` commit, then a proposed patch version.
- No release-please and no chain of PRs triggering PRs (the `scaleway-operator` mechanism is considered complicated and shaky). Goal: a readable and predictable mechanism.
- A milestone title is exactly its semver version (for example `v0.3.0`). The milestone theme goes in its description.
- One workflow named "Deliver", with two triggers:
  - milestone closed: the version is the milestone title;
  - "Run workflow" button (`workflow_dispatch`): the human types the tag to create.
- One single branch point for the origin of the version, then a common trunk: check (valid semver, tag does not exist, no open issue in the milestone); create the tag; build; publish to ghcr.io; create the GitHub release. A `concurrency` rule prevents two simultaneous deliveries.
- Verified in the GitHub docs: the button and its inputs exist; the workflow file must be on the default branch; an event produced with `GITHUB_TOKEN` does not create a new workflow run (except `workflow_dispatch` and `repository_dispatch`); the `milestone` event with type `closed` exists. To check at implementation: how to read the milestone title from the event.

### Rollback and migrations

- No published version is ever removed. A faulty version is fixed by a patch.
- The release note contains the command to reinstall the previous version.
- The migration process, in both directions, is part of what is delivered: it is produced during the cycle, not generated by GitHub.
- Two-level guard:
  - every PR must add or change its own file in `docs/migrations/next/`, named after its issue number (one file per issue avoids conflicts between parallel PRs). A file without impact contains exactly `Migration: no impact`, which "Deliver" matches word for word. Mechanical check, no visual check by the human;
  - the "Deliver" workflow refuses to publish without these files. If every entry states no impact, the release has a single "no impact" entry. If at least one other entry exists, the "no impact" entries are removed and only the others appear in the release note.

## Step 4: agentic materialization

- Sequence diagrams: `sequence-long-process.md` (building blocks, start and resume, long process), `sequence-short-process.md`, `sequence-bug-process.md`.
- Steps of the long process are not tied to sessions. The long process can start in one session, stop at any point, and resume in another session.
- Entry point: one orchestrator skill, `/work`. Without argument it starts from an idea (triage, issue, branch). With an issue number (`/work 42`) it resumes: it reads the local process memlog, the Project v2 status, the gate comments on the epic and the sub-issues, states the step reached and proposes the next one. At each step it calls the right skill (existing BMad or new) and updates the memlog, the status and the GitHub traces. `bmad-help` remains available for orientation and recommends `/work` through the catalog.
- `AGENTS.md` holds only a short "Development process" section: the `/work` entry point and the hard rules no agent may break. It must stay small: the author asked explicitly not to grow its size. The full process lives in `docs/development-process.md`, loaded by the process skills through `persistent_facts` and read by `bmad-help` (`project_knowledge` points to `docs/`).
- What to create (base, to confirm at implementation):

  | Need | Block |
  | --- | --- |
  | Orchestration, triage, resume, GitHub updates (`gh`) | New skill `/work` |
  | Parallax (3 to 5 fresh subagents) | New skill |
  | Cold-start test (one fresh subagent per task) | New skill |
  | HTML pages of the spec and plan gates | New skill |
  | Spec-lint and ambiguity threshold | Override of `bmad-review` (one more lens) |
  | Tests written without seeing the code | Override of `bmad-build` (`implementation_handoff`) |
  | Review loop (3 lenses, verifier, 2 dry passes, 5 rounds) | Override of `bmad-code-review` (`review_layers`) |
  | Test fingerprints, mutation testing, evidence record, behavior diff | `make` targets (behavior diff with playwright-bdd) |
  | Fake operator | Project code |

- No commit at every small step: resuming may rely on what is on the local machine.
- `.memlog.md` files are committed again (removed from `.gitignore`, `AGENTS.md` updated): `bmad-spec` treats its memlog as the canonical source of `SPEC.md`.
- Approach: express the process by customizing BMad, so that `bmad-help` becomes the entry point to resume ("where am I, what next"). Limit: `bmad-help` reads local files only, not GitHub.
  - Configure existing skills first, through overrides in `_bmad/custom/` (the only files the installer never touches): `persistent_facts`, `activation_steps_prepend/append`, `on_complete`, and skill-specific fields (`bmad-build` `implementation_handoff` and `review_layers`, `bmad-code-review` `review_layers`, `bmad-review` `lenses`).
  - Create new skills only for the gaps (triage, parallax, spec-lint, cold-start, gate pages, GitHub updates, and an orchestrator that keeps one process memlog per piece of work).
  - Python is allowed for the internal scripts of the skills we add.
  - Register the process in the `bmad-help` catalog through a custom BMad module, versioned in the repository under `skills/`, carrying our skills and their `module-help.csv` rows, installed with `npx bmad-method install --custom-source <path>`. The catalog `_bmad/_config/bmad-help.csv` is the installer's assembly of every module's `module-help.csv` and is never edited by hand. The bmb setup scripts are not used (out of step with BMad 6.12.4). A module change takes effect after reinstall. Known risk: issue #2869 (quick-update fails for local custom modules, reported on 6.12.0; status on 6.12.4 not verified).

## Rejected

- Python-based solutions for the project and its tests: a language the author does not want to maintain. Exception: internal scripts of the BMad skills we add.
- Hooks to lock the record: considered error-prone.
- Claude Code sandbox as a lock: topic stopped by the author. The sandbox is shared by the whole session and its subagents; there is no per-agent sandbox.
- Cucumber.js + Playwright (option B): capture management would have to be rewritten.
- Releasing on every cycle: replaced by the milestone or "Deliver" button release.
- release-please, as used by `scaleway-operator`.

## Open

- Triage criteria, to refine with use.
- Names of the route labels (short, intermediate, long, epic), in English, to set at implementation.
- Step 4: how the process materializes for agents. How it translates into `AGENTS.md`; whether agents must be created; a sequence diagram centered on the Claude session started by the human.
