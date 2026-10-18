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
| Epic | Parent issue + `epic` label + native sub-issues. Verified: creating sub-issues works on this personal repository (epic #2, sub-issues #3 to #32 and #35; unit U*n* = issue #*n+2* for U1 to U30, U31 = #35). |
| Route labels | `route:short`, `route:bug`, `route:long`; `epic` added on the parent issue of long work. Default labels stay. |
| Custom BMad module | `mathieubodin-sdlc`, under `skills/` (`bmad_builder_output_folder`), in the multi-skill layout documented by BMad Builder: `.claude-plugin/marketplace.json` at the repository root and a setup skill `mathieubodin-sdlc-setup` carrying `assets/module.yaml` and `assets/module-help.csv`. Its setup skill runs no script: the installer registers the rows. |
| Skill names | `sdlc-work` (replaces `/work` everywhere) and `sdlc-gate-page`. The `x:y` form exists only for Claude Code plugins; the BMad installer copies skills flat into `.claude/skills/`. |
| Dynamic workflows | Stages without human interaction run as saved Claude Code workflows in `.claude/workflows/` (versioned, outside the BMad module and the `bmad-help` catalog): `sdlc-triage`, `sdlc-parallax`, `sdlc-spec-lint`, `sdlc-explore`, `sdlc-cold-start`, `sdlc-rails`, `sdlc-build`, `sdlc-review`. A workflow accepts no input mid-run, so `sdlc-work` (skill) keeps every interaction and launches a workflow between two of them; alerts become return values. Reason: counters and caps enforced by script, intermediate results kept out of the session context, per-agent audit in `/workflows`. |
| Alignment with manual BMad use | Where a BMad skill can be run by hand for the same job, its `_bmad/custom/<skill>.toml` override is the single definition and the workflow reads it: `sdlc-work` runs `_bmad/scripts/resolve_customization.py` (BMad's resolver: defaults, team and `.user.toml` overrides) and passes the resolved prompts in `args`. Prompts are written as plain prompt text usable by both. Pairs: `sdlc-review` ↔ `bmad-code-review` (`review_layers`), `sdlc-spec-lint` ↔ `bmad-review` (`spec-lint` lens), `sdlc-rails` and `sdlc-build` ↔ `bmad-build` (custom keys `sdlc_test_writer_prompt`, `sdlc_builder_prompt`, used by `implementation_handoff`; verified that the resolver passes custom keys). The workflow adds what the skill cannot do (votes, loops, counters, parallel worktrees). Not applicable: `sdlc-triage`, `sdlc-parallax`, `sdlc-cold-start`, `sdlc-explore`. |
| Workflow tooling | Node tooling pinned in `package.json` and its lockfile. Workflow scripts linted by oxlint and formatted by oxfmt, because Biome cannot parse their top-level `return`; Biome for the harness, the tests and JSON (`make lint-workflows`). Control flow tested with a `node:test` harness that fakes the Claude Code hooks; coverage with `c8`, 100 % required on the workflow scripts (`make test-workflows`). |
| Build merges | `sdlc-build` returns the finished builder branches and the failed stories; `sdlc-work` merges them into the work branch one by one with `make check-frozen` after each. Trunk and leaves are two runs: trunk, merge, then leaves, merge. |
| Triage | Method inspired by TypeSafe's System One model (docs.typesafe.ai), without using Jev itself: atomic questions written in `docs/development-process.md`, each answered by 3 independent Claude agents with structured output; the workflow `sdlc-triage` combines the answers in code into a proposed route and reports the vote agreement as the uncertainty signal; the human confirms; answers and the human's decision are recorded in the memlog to build a history. The initial questions and combination rule are proposed and validated in U30, then adjusted with the history. Jev was not adopted: low volume, no history to calibrate thresholds, blast radius needs multi-hop reasoning on code (a documented weakness), external hosted dependency. To revisit with the history. |
| Spec-lint | Same method: workflow `sdlc-spec-lint`, atomic questions per acceptance criterion (names a verification method, is unambiguous), 3 votes, threshold as a vote proportion set in code. The questions live in a `spec-lint` lens of the `bmad-review` override, so a manual `bmad-review` asks the same questions once. |
| Documentation lint | `markdownlint-cli2`, configured in `.markdownlint-cli2.yaml`, run by `make lint-docs` locally and in CI (paths and rules in U8). |
| Git hooks | prek (`prek.toml`), as in `scaleway-operator`. prek only runs things: one tool per file type (markdownlint for Markdown, Biome for JS and JSON, prek builtins for YAML, TOML, line endings and shebangs), local hooks call the same `make` targets as CI. |
| Process memlog | `_bmad-output/work/issue-{n}-{slug}/.memlog.md`. Gate HTML pages, the evidence record, `define-notes.md`, the per-work `plan.md` and `bug-report.md` go in the same folder. `SPEC.md`, `stories.yaml` and the story specs stay where `bmad-spec` and `bmad-build` write them (`_bmad-output/specs/spec-{slug}/`) and are referenced from the memlog. |
| Branch and slug | Branch `{type}/{n}-{slug}` (type = Conventional Commits type planned for the PR); slug derived from the issue title, kebab-case, at most 4 words, confirmed with the route (long G1, G3). |
| Migration files after a release | Nothing is moved. "Deliver" keeps only the `docs/migrations/next/` files added or changed since the previous tag (`git diff --name-only <prev-tag>..HEAD`); all files when no tag exists. |
| Review loop | Workflow `sdlc-review`, parameterized by `args`: long route 2 dry passes, at most 5 rounds; bug route 1 dry pass, at most 2 rounds; short route one pass, verified findings fixed, no second pass. Lenses (security, correctness, coherence) and the verifier are defined in the `bmad-code-review` override, whose 4 default layers are disabled; the workflow adds the loop (`bmad-code-review` has none). Beyond the cap, the workflow returns the recurring findings; `sdlc-work` proposes a way out and the human chooses (long G23, bug G4, short G2). |
| Role of `bmad-build` | Used only during Plan, to write the story specs up to `ready-for-dev`. Test writers and builders run in the workflows `sdlc-rails` and `sdlc-build` (long G17, G21), with prompts defined in its override and shared with a manual run. Its review layers are disabled by override. |
| Token cost | Decided in the brainstorm section "Token cost tracking". Raw tokens per category and model in `work/issue-{n}-{slug}/cost.json`, keyed by session and workflow run, attributed through the process memlog (step windows and workflow run ids), pre-triage tokens attributed to the issue, the rest reported as unattributed. `make cost ISSUE=<n>` at every `sdlc-work` commit and at resume; summary in the evidence record and the PR; Project `Tokens` = input + output + cache creation; `make cost-report` for aggregates per step and route. Shell and jq. Global trailers kept. Delivered early (U31) so that epic #2's sessions are measured before their transcripts are purged. |
| Hand-back | Every process skill gets an `on_complete` override that writes its result to the active process memlog and hands control back to `sdlc-work` (long G4). |
| Setup | Run manually in a separate session with `bmad-architecture`, directly from the brief, its addendum, the forge session and `AGENTS.md` (no global product spec: each feature gets its own spec through the long route). `sdlc-work` refuses Plan unless the architecture spine exists and names the stack (long G13). |

Verified facts: `gh` token has the `project` scope; Project v2 #4 has the statuses Backlog, Triage, Define, In Progress, Review, Done; BMad is 6.12.4; `project_knowledge` points to `docs/`; `.memlog.md` files are no longer ignored (done).

## Units

Status: **ready** or **blocked (stack)**.

### U1. GitHub repository setup — done

- Create labels `route:short`, `route:bug`, `route:long` (`epic` already exists, created with this plan's epic).
- Keep the existing classic branch protection on `main` (checked on 2026-10-18: PR required with 0 approvals, required status check `conventional-title`, the job of `.github/workflows/pr-title.yml`, `enforce_admins` on, force pushes and deletions refused). No ruleset is created. The checks of U11 are added to it once they exist.
- Enable the Project v2 built-in workflows "Item closed → Done" and "Pull request merged → Done" (long G24).
- Acceptance: `gh label list` shows the four labels; a direct push to `main` is refused; closing a test issue moves it to Done.
- Depends on: nothing. Required checks are extended after U11.
- Verified on 2026-10-18: `gh label list` shows `epic`, `route:short`, `route:bug`, `route:long`; GraphQL lists "Item closed" and "Pull request merged" as enabled (switched on in the UI, the API cannot do it); a test issue (#36) added to the Project without status moved to `Done` about 10 s after being closed, then was deleted; a direct push of an empty commit to `main` was refused with `GH006` ("Changes must be made through a pull request", "Required status check \"conventional-title\" is expected") and the remote `main` stayed at `945c57b`.

### U2. `docs/development-process.md` — first version done, final pass pending

- Full process, condensed from `decisions.md`, the three `sequence-*.md`, the three simulations and the decisions above: routes, gates and their aids, two-failure rule, frozen tests, testing without Scaleway (`@manual`), GitHub mapping, Project v2 statuses, migration files, release.
- Includes the three corrected sequence diagrams from the brainstorm folder (`sequence-*-process.md`, updated with the simulations). In particular, bug tests are frozen before the builders (bug G1), which supersedes `decisions.md`.
- Triage and spec-lint questions (from U30 and U5) written in the document.
- Token cost tracking: a short summary and a link to `docs/token-cost-tracking.md` (U31), not a copy.
- Only what a future reader needs; no history of the brainstorm.
- Acceptance: `make lint-docs` green; every string constant (labels, statuses, paths, skill names, `Migration: no impact`) matches the code of the other units (coherence check at review).
- Depends on: nothing. Revisited at the end of the epic for consistency.
- First version on 2026-10-18: `make lint-docs` green; triage and spec-lint questions and the install command left as marked placeholders (U30, U5, U3). Independent review (correctness, coherence) checked labels, statuses, `worktree.baseRef`, `make` targets and variables, `sdlc-review` args, `conventional-title`, `Migration: no impact` against the code and the sources; its 8 findings were fixed (two-failure notes in the diagrams, `lenses` key, forge path, first freeze without human decision, epic number for the migration file, prek install, token summary in evidence and PR, file loaded as a persistent fact). The freeze finding also fixed the `make freeze` help text and U8. The install command was then written by U3.

### U3. Custom BMad module skeleton `mathieubodin-sdlc` — done

- `.claude-plugin/marketplace.json` (plugin `mathieubodin-sdlc`, version, skill paths); setup skill `skills/mathieubodin-sdlc-setup/` with `assets/module.yaml` (`code: mathieubodin-sdlc`) and `assets/module-help.csv`; the two skill folders (`sdlc-work`, `sdlc-gate-page`) with minimal `SKILL.md`; rows so that `bmad-help` recommends `sdlc-work` to start or resume work. Workflows are not part of the module.
- Install with `npx bmad-method install --custom-source <path>`; record the exact command in `docs/development-process.md`.
- Check first: the module layout expected by `--custom-source` on 6.12.4; whether issue #2869 (quick-update of local custom modules) still applies.
- `_bmad/_config/bmad-help.csv` is never edited by hand.
- Acceptance: after install, `_bmad/_config/bmad-help.csv` contains the module rows and the two skills are listed in `.claude/skills/`; a reinstall after a row change updates the catalog.
- Depends on: nothing.
- Verified on 2026-10-18 (bmad-method 6.12.4): in the installer code, a local `--custom-source` with `.claude-plugin/marketplace.json` uses discovery mode and resolves the module through its `-setup` skill; a skill counts only if its `SKILL.md` `name` equals its folder. Issue #2869 is still open and its cause is still in the code (`official-modules.js:282` calls `findModuleSource` without `bmadDir`): always pass `--custom-source`. `validate-module.py skills` passes with no finding. After `npx bmad-method@6.12.4 install --directory "$PWD" --custom-source "$PWD" --tools claude-code --yes`, `bmad-help.csv` has the 3 `SDLC Process` rows and `.claude/skills/` the 3 skills; a temporary row change appeared in the catalog after a reinstall and disappeared after the next one. A `--pin bmb=...` run left bmb on `channel: pinned`; `--all-stable` restored `stable` (v2.2.2, latest stable). Independent review (security, correctness, coherence): 4 low findings, fixed (args of `sdlc-work`, installed copies named in the doc, U3 not closed); the committed `localPath` of `manifest.yaml` is machine-specific and documented.

### U4. `_bmad/custom/` common overrides — done

- `persistent_facts = ["file:{project-root}/docs/development-process.md"]` for the process skills: `bmad-forge-idea`, `bmad-deep-recon`, `bmad-spec`, `bmad-review`, `bmad-architecture`, `bmad-build`, `bmad-code-review`, `bmad-walkthrough`. (`bmad-create-epics-and-stories` is not used. `bmad-advanced-elicitation` has neither `persistent_facts` nor `on_complete` in its `customize.toml`: no override, `sdlc-work` records its result, see U20.) `bmad-review` and `bmad-code-review` are run by hand only, their overrides being read by the workflows.
- `on_complete` on the same skills: when a `sdlc-work` process memlog is active (current branch `{type}/{n}-{slug}` and `_bmad-output/work/issue-{n}-{slug}/.memlog.md` exists, decided with the author; a manual run on a work branch hands back too; in a headless run, the skill only appends to the memlog), write the result to it and hand control back to `sdlc-work` (long G4).
- `bmad-forge-idea`: `activation_steps_append` to check the code before each question, and to have every recorded assumption validated by yes or no before `forged-idea.md` is distilled (long G5).
- Acceptance: each skill, when activated, states the process file among its loaded facts; at its end with an active memlog, the memlog has a new entry and `sdlc-work` proposes the next step.
- Depends on: U2.
- Verified on 2026-10-18: `_bmad/scripts/resolve_customization.py` returns, for each of the 8 skills, the process file in `persistent_facts` and the hand-back in `on_complete` (a string, replacing the empty default; an array of 2 entries for `bmad-forge-idea`, whose `activation_steps_append` has 1 entry). A real activation of `bmad-review` by a subagent on a temporary branch `docs/999-handback-test` with a memlog: the process file was loaded as its only persistent fact; at its end it appended `- (result by bmad-review) ...` to the memlog and recommended no next step, pointing to `/sdlc-work 999` instead; branch and folder then deleted. The part "`sdlc-work` proposes the next step" waits for U17. Independent review: 5 findings, fixed: placeholders written `<type>/<n>-<slug>` so as not to collide with the `{slug}` variable of forge, spec, build and architecture; `--text` in single quotes without quote, backtick or dollar sign (no command substitution from text coming from an issue); forge assumptions validated before `forged-idea.md` is distilled, not after (moved to `activation_steps_append`); U20 pre-mortem note; `bmad-build` during Plan (steps 01 and 02) never runs its `on_complete`, so `sdlc-work` records the story specs itself (U6, U20).

### U5. `bmad-review` spec-lint lens and workflow `sdlc-spec-lint` — ready

- `_bmad/custom/bmad-review.toml`: a `spec-lint` lens whose instruction lists the atomic questions ("names a verification method?", "unambiguous?") as plain prompt text. A manual `bmad-review` applies them once.
- `.claude/workflows/sdlc-spec-lint.js`. `args`: `SPEC.md` path and the resolved `spec-lint` lens (passed by `sdlc-work` from the resolver).
- An agent lists the acceptance criteria; for each criterion, each question is answered by 3 independent agents with structured output; a criterion fails when the proportion of "yes" votes is below the threshold set in code.
- Returns the failing criteria with the votes, for `sdlc-work` to turn into questions.
- Initial questions and threshold proposed by the agent and validated by the human in this unit.
- Acceptance: harness tests of the vote combination; a real run on a `SPEC.md` with a criterion lacking its verification method reports it, and a manual `bmad-review` with the `spec-lint` lens reports it too.
- Depends on: U26, U4.

### U6. `bmad-build` override: story specs for Plan — ready

- `bmad-build` is used only during Plan: for each story of `stories.yaml`, it runs steps 01 and 02 and stops at status `ready-for-dev`, producing `{spec_folder}/stories/{id}-{slug}.md` (long G17). Check how to stop it there (override, or the way `sdlc-work` invokes it).
- `review_layers` and `oneshot_review_layers`: empty `instruction`.
- Custom keys `sdlc_test_writer_prompt` (given the story spec only, never the code) and `sdlc_builder_prompt` (story spec + frozen tests), read by `sdlc-rails` and `sdlc-build` through the resolver.
- `implementation_handoff` for manual use, one story, sequential: test writer with `sdlc_test_writer_prompt` → `make freeze` → builder with `sdlc_builder_prompt`.
- Acceptance: on a toy `stories.yaml`, one story spec per story exists in `ready-for-dev` and no code was written; a manual `bmad-build` on one story writes and freezes tests before the code.
- Depends on: U4.

### U7. `bmad-code-review` override and workflow `sdlc-review` — ready

- `_bmad/custom/bmad-code-review.toml`: `review_layers` security, correctness and coherence (coherence mandatory: string constants between code and docs; actual content of security and config files), plus a verifier layer that reproduces each finding before it is kept; triage then findings (author's global review rule); prompts as plain prompt text; the 4 default layers disabled (empty `instruction`). A manual `bmad-code-review` runs one pass with these layers.
- `.claude/workflows/sdlc-review.js`. `args`: diff range, spec or bug report path, the resolved `review_layers` (passed by `sdlc-work` from the resolver), `dryPasses`, `maxRounds`, `singlePass`.
- Each round: one agent per layer, every finding reproduced by the verifier before it is kept; dedup against all findings seen; a fixer agent applies verified findings and runs `make`.
- A pass is dry only if at least three distinct lenses found nothing. Stops after `dryPasses` dry passes, or returns the recurring findings when `maxRounds` is reached.
- Acceptance: harness tests (U26) for stop after 2 dry passes, return at round 5, single pass; a real run on a diff with a deliberate label mismatch between code and docs reports it and the verifier keeps it; a manual `bmad-code-review` on the same diff reports it too.
- Depends on: U26, U8, U4.

### U8. `make`: skeleton, docs lint, frozen tests, Git hooks — done

- `Makefile` with `help`; `lint-docs` (`markdownlint-cli2`, `.markdownlint-cli2.yaml`, checks only) and `fix-docs` (same with `--fix`). Paths: every `**/*.md` except `.claude/skills/bmad-*/**` and `_bmad/**` (installed by BMad, overwritten on each install); `_bmad-output/**` and our own skills in `.claude/skills/` are linted. Rules: markdownlint defaults, MD013 off (one paragraph per line, as BMad skills write), MD041 off (`.memlog.md` starts with front matter then a list), MD001, MD024, MD025 as in `scaleway-operator`.
- `freeze`: `make freeze FILES="..."` records the sha256 of the given test files (code tests and Gherkin features) in the committed manifest `frozen-tests.sha256` (`sha256sum` format), replacing the entries of those files and keeping the others; run on new tests, or after a human decision to change a frozen test.
- `check-frozen`: recomputes and fails on any changed or missing frozen file; passes when the manifest is absent or empty.
- Variable `MUTATION` (on by default, `MUTATION=0` switches it off, any other value is an error), used by U12.
- `prek.toml` (as in `scaleway-operator`): prek only runs things, one tool per file type. Builtin hooks `check-yaml`, `check-toml`, `mixed-line-ending`, shebang checks; local hooks `make fix-docs` and `make check-frozen`. JSON and trailing whitespace are left to Biome (U26) and markdownlint. Install once per clone with `prek install`; when a global `core.hooksPath` is set (on the author's machine it delegates every hook to `.git/hooks/`), `prek install` refuses and `prek install --git-dir "$(git rev-parse --absolute-git-dir)"` is used instead (checked: the global `pre-commit` then runs prek). `make lint-workflows` is added as a local hook in U26.
- Shell only, no Python.
- Acceptance: editing a frozen file makes `make check-frozen` fail; `make freeze` then makes it pass.
- Depends on: nothing.
- Verified on 2026-10-18 (markdownlint-cli2 v0.22.0, prek 0.3.11): `make lint-docs` green after `make fix-docs` (blank lines around headings, lists and tables in two `_bmad-output/` files) and one table delimiter row fixed by hand; `make check-frozen` exit 0 without manifest, 0 after `freeze` of two files, 2 after editing one, 0 after refreezing only that one (the other entry kept), 2 after deleting the other; `make freeze` without `FILES` or with a missing file exits 2; `make help MUTATION=2` exits 2; `prek validate-config` valid and `prek run --all-files` all passed. Note for U11: pin the `markdownlint-cli2` version in CI.

### U9. `docs/migrations/next/` and its checks — done

- Folder `docs/migrations/next/`, nothing else in it. Entry format, decided with the author: exactly `Migration: no impact`, or `## Upgrade` and `## Rollback` sections, each with text (documented in `docs/development-process.md`, "Migration files").
- `make check-migration ISSUE=<n> [BASE=origin/main]` (`scripts/check-migration.sh`): fails unless `docs/migrations/next/<n>.md` is added or changed by the commits of the branch since `git merge-base BASE HEAD` (decided with the author: a file inherited from an earlier PR of the same issue, or changed on `main` by another PR, does not count; uncommitted changes are not seen), or when its content in `HEAD` has another format.
- Every PR must reference its issue (`Closes #<n>`, the epic first for long work); `scripts/pr-issue.sh` reads `<n>` from the first `Closes #<n>` of the PR body outside HTML comments and fails when absent; CI (U11) uses it.
- Acceptance: a branch without the file fails; with it, passes.
- Verified on 2026-10-18 in throwaway git repositories: no file, a file inherited from an earlier PR, a file changed on `main` after the branch left it, an uncommitted change, an empty, missing or repeated section, a section holding only an HTML comment or an empty code fence, a `## Rollback` only inside a code fence, a level-1 heading cutting a section, extra text after `Migration: no impact`: exit 1. Both formats, `###` subheadings, code-only sections, trailing spaces, no final newline: exit 0. Issue `05`, `4x`, a `BASE` starting with `-` or not a commit: exit 2; an `ISSUE` holding a command substitution runs nothing. `pr-issue.sh`: first `Closes` taken, CRLF bodies, HTML comments (one or several lines) skipped, `#8x` and `#0` refused. Epic #2 carries `docs/migrations/next/2.md` (no impact: tooling only); `make check-migration ISSUE=2` passes once it is committed. Independent review (security, correctness, coherence): 11 findings, all fixed.
- Depends on: U8. From the merge of this unit on, every PR of the epic carries its migration file.

### U10. `make evidence` — ready

- Replays the checks available (`lint-docs`, `check-frozen`, `check-migration`, and the U12 to U14 targets once they exist) and rewrites the whole record in `_bmad-output/work/issue-{n}-{slug}/evidence.md`, whoever runs it.
- Includes the cost summary from `cost.json` (U31): totals per step, per model, unattributed.
- Acceptance: two consecutive runs on an unchanged tree give the same record, except timestamps.
- Depends on: U8, U9, U31.

### U11. PR CI checks — ready

- Workflow `pr-checks.yml` on every PR: `make lint-docs`, `make lint-workflows`, `make test-workflows`, `make check-frozen`, `make check-migration`. Code tests added by U12.
- The behavior diff stays local.
- Add the jobs to the required status checks of the `main` branch protection (U1).
- Acceptance: a PR breaking each check fails on the right job.
- `actions/checkout` with `fetch-depth: 0`: `make check-migration` needs the history down to the merge-base with `main` (a depth-1 clone exits 2).
- Pin the Node version used by CI (the harness runs `node --test` with a glob, Node 21 or later; verified with Node 24.15.0) and run `npm ci` before the Node targets.
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

- Without argument: questions, route proposal by the workflow `sdlc-triage` (U30) shown with its answers and vote agreement, optional milestone proposal, human confirmation, issue creation with its route label (`epic` for long work), add to Project v2 with its status, work branch `{type}/{n}-{slug}`, memlog init (long G1, G3).
- A chosen milestone that does not exist is created after confirmation: title checked as semver, theme asked and written in the description (long G2).
- With an external issue number not yet triaged: reads it and triages it; for a bug, writes `bug-report.md` from it (bug G3).
- Rerouting to the long route: same issue promoted (`route:long` + `epic`, status Define, comment "rerouted: reason"), branch and memlog kept, branch renamed if its type changes (short G4).
- Python allowed for internal scripts (for example Project v2 status updates).
- The memlog records the triage answers, the proposed route and the human's decision.
- Cost ledger (U31): the memlog records each step's start and end (session id, timestamps) and the id of every workflow run launched; tokens of the same session before triage are attributed to the created issue (step "triage"). Every commit made by `sdlc-work` runs `make cost ISSUE=<n>` first and updates the Project `Tokens` field; this applies to U18, U19 and U20 as well.
- Acceptance: on a test idea, the issue, label, Project status, branch and memlog exist and agree.
- Depends on: U3, U1, U2, U30, U31.

### U18. Skill `sdlc-work`: resume — ready

- `/sdlc-work <n>`: reads the local memlog, the Project v2 status, the gate comments on the epic and its sub-issues; states the step reached and proposes the next one. Without a local memlog, resumes from the last gate recorded on GitHub.
- Runs `make cost ISSUE=<n>` at resume (U31).
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

- Define: `bmad-forge-idea` → workflow `sdlc-parallax` → pre-mortem (`bmad-advanced-elicitation` has no `on_complete`: `sdlc-work` writes the risks to `define-notes.md` and keeps control) → research if proposed and accepted (long G8) → answers in `define-notes.md` (long G7) → `bmad-spec` (inputs `forged-idea.md` and `define-notes.md`) → spec-lint loop (workflow `sdlc-spec-lint` per round, questions in `sdlc-work`, alert after 3 rounds, long G9) → `spec.html`.
- Gates: `xdg-open` on the page (long G12); on validation, commit the work folder and the folders referenced by the memlog, push, comment with the commit link (long G11); on invalidation, propose a return step, the human chooses (long G10).
- Setup guard before Plan (long G13).
- Plan: reread the spine, `docs/solutions/`, previous specs and MemPalace (long G14); workflow `sdlc-explore`; write `work/…/plan.md` (long G15); Story Breakdown of `bmad-spec` (long G16); story specs through U6; cold-start loop (workflow `sdlc-cold-start` per round, questions and fixes in `sdlc-work`, alert after 3 rounds, long G18); `plan.html`; after the plan gate, one sub-issue per story spec.
- Rails: workflow `sdlc-rails` (one test writer per story, blind to the code, then a single `make freeze`), commit by `sdlc-work`, no human check (long G19, G20).
- Build: workflow `sdlc-build` for the trunk, merge by `sdlc-work`, then `sdlc-build` for the leaves, merge one by one with `make check-frozen` after each (long G21). A "back to Plan" return value sends the work back to Plan and its gate.
- Review: workflow `sdlc-review` (2 dry passes, at most 5 rounds, result in the memlog); on return at the cap, proposal and human choice (long G23).
- Before the PR: `/ce-compound`, MemPalace update, after-integration issues "simplify after #n" and, when relevant, "distill lessons" (short G1), migration file, evidence.
- PR body with `Closes` for the epic first, then every sub-issue, and the cost summary; sub-issues follow the epic's status (long G22). The epic carries the `Tokens` total; sub-issues have none.
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

### U25. Spike: base of `isolation: 'worktree'` — done

- Checked on 2026-10-18 with Claude Code 2.1.302, one-agent workflow run twice from `process/sdlc` at `d0074c9`:
  - default setting: the worktree started from `main` (`945c57b`), without the work branch files;
  - with `"worktree": {"baseRef": "head"}` in `.claude/settings.json`: it started from `d0074c9`, with the work branch files. Worktree and branch removed automatically after each run.
- Chosen approach: keep `worktree.baseRef: "head"` in the versioned `.claude/settings.json`; `.claude/worktrees/` added to `.gitignore`; no `git reset` in builders, but each builder checks that its `HEAD` equals the SHA passed in `args` and fails otherwise (U29).
- Written in `docs/development-process.md`, section "Parallel builders" (U2).
- Depends on: nothing.

### U26. Workflow tooling — done

- Node tooling pinned in `package.json` and `package-lock.json` (exact devDependencies, `npm ci`).
- Workflow scripts (`.claude/workflows/*.js`, harness fixtures) linted by oxlint (`.oxlintrc.json`, Claude Code hooks declared as globals, `no-undef` on) and formatted by oxfmt (`.oxfmtrc.json`); Biome (`biome.json`) lints and formats the harness, the tests and the project JSON files (generated `cost.json` excluded). `make lint-workflows` checks all of them; `make fix-workflows` fixes then checks.
- `node:test` harness (`tests/workflows/harness.js`) that loads a workflow script and provides fake `agent`, `parallel`, `pipeline`, `phase`, `log`, `args`, `budget` and `workflow` with scripted answers; `make test-workflows` with coverage by `c8` (`.c8rc.json`), failing below 100 % of the workflow scripts.
- `make fix-workflows` is added as a local hook in `prek.toml` on `.js` and `.json` files, like `make fix-docs` (U8).
- Acceptance: a deliberately broken script fails `make lint-workflows`; a sample script reaches 100 % branch coverage under the harness.
- Depends on: U8.
- Verified on 2026-10-18 (Node 24.15.0, Biome 2.5.15, oxlint 1.86.0, oxfmt 0.71.0, c8 12.0.0): Biome cannot parse a top-level `return` and has no option for it, so workflow scripts went to oxc (oxlint, oxfmt), whose parser always accepts it (web research: ESLint forces it off for ES modules with espree; Prettier's babel parser and Deno also accept it; `actions/github-script` restructures into a function instead, impossible here). A script in `.claude/workflows/` with a syntax error, an unknown hook (`no-undef`) or only a format issue each fails `make lint-workflows` (exit 2); `make fix-workflows` still formats when a lint error remains, then fails. `tests/workflows/fixtures/sample.js` reaches 100 % statements, branches, functions and lines with 9 tests; with one test, 54.83 % and exit 1, uncovered lines being those of the source. `prek run --all-files` green. Independent review (security, correctness, coherence): no install scripts, every package from registry.npmjs.org; 12 findings, 10 fixed (dead agent returns `null`, `budget` and `workflow` hooks, `Date()` forbidden, meta must begin the script, line numbers kept, fixes not stopped by a lint error, `cost.json` out of Biome, coverage gated, temp files removed, plain-data result documented); file names with spaces left out (scripts are kebab-case); the Node version is pinned in U11.

### U27. Workflow `sdlc-explore` — ready

- `.claude/workflows/sdlc-explore.js`. `args`: spec path, explorations to run (code, practices, frameworks).
- Parallel readers, one per exploration; returns a structured map for `plan.md`.
- Acceptance: harness test; a real run on an existing spec returns the three sections.
- Depends on: U26.

### U28. Workflow `sdlc-rails` — ready

- `.claude/workflows/sdlc-rails.js`. `args`: mode (`long` or `bug`), story spec paths or `bug-report.md`, the resolved `sdlc_test_writer_prompt` (U6).
- Long mode: one test writer per story with `sdlc_test_writer_prompt`, given the story spec only, never the code; then an agent runs `make freeze` for the written tests.
- Bug mode: one test writer that reads the code and writes tests only; an agent runs the new tests; no freeze, because the human must first confirm the failure reason (bug G2); `sdlc-work` runs `make freeze` after that confirmation.
- Returns the test files written and, in bug mode, each test's failure message.
- Acceptance: harness test; in a real long-mode run, the test writers' prompts contain no code path (checked in `/workflows`).
- Depends on: U26, U8, U6.

### U29. Workflow `sdlc-build` — ready

- `.claude/workflows/sdlc-build.js`. `args`: mode (`long` or `bug`), story spec paths or `bug-report.md`, work branch SHA (U25), the resolved `sdlc_builder_prompt` (U6).
- One builder per item with `isolation: 'worktree'` and `sdlc_builder_prompt`, given the story spec (or `bug-report.md`) and the frozen tests. Each builder first checks that its `HEAD` equals the work branch SHA from `args` (worktrees start from HEAD through `worktree.baseRef: "head"`, U25) and fails otherwise. Two-failure rule in script: after a failure, an agent improves the ticket (or enriches `bug-report.md`) and a new builder starts; after a second failure, the item is returned as "back to Plan" (long) or "ask the human" (bug).
- Returns the finished branches and the failed items; merging is done by `sdlc-work`.
- Acceptance: harness tests for success, one failure then success, two failures; a builder that edits a frozen test fails `make check-frozen`.
- Depends on: U25, U26, U8, U6.

### U30. Workflow `sdlc-triage` — ready

- `.claude/workflows/sdlc-triage.js`. `args`: the idea or issue text, plus facts gathered by `sdlc-work` (files likely touched, visible in the UI or not).
- Atomic questions (for example "introduces new behavior?", "touches the adoption keys: namespace, `project-id` annotation, CR names?", "describes an observed defect?", "visible in the UI?"), each answered by 3 independent agents with structured output; the combination rule in code yields a route (`short`, `bug`, `long`) and the vote agreement.
- Initial questions and combination rule proposed by the agent and validated by the human in this unit, written in `docs/development-process.md`.
- Acceptance: harness tests of the combination rule (unanimous, split, conflicting answers); a real run on three sample ideas yields the expected routes.
- Depends on: U26.

### U31. `make cost` and `make cost-report` — done

- Reference: `docs/token-cost-tracking.md` (ledger format, step names, attribution rules, where `make cost` runs in each route and who runs it, example, limits).
- `make cost ISSUE=<n>` (`scripts/cost.sh`): reads the `- (event) <keyword> key=value ...` lines of every process memlog (`route`, `step-start`, `step-end`, `workflow-run`, `session`), the transcripts of the sessions named by memlog `<n>` under `~/.claude/projects/<project>/` (main session, `subagents/`, `subagents/workflows/<runId>/`, one record per `message.id`), and writes `_bmad-output/work/issue-{n}-{slug}/cost.json`: tokens per category (input, output, cache creation, cache read) and per model, per step, plus an unattributed total. Pre-triage tokens are counted in the `triage` step, not as a separate share. The route is read from the memlog and stored in `cost.json`.
- Steps: long `triage`, `define`, `plan`, `rails`, `build`, `review`, `deliver`; bug `triage`, `rails`, `build`, `review`, `deliver`; short `triage`, `change`, `review`, `deliver`; `bootstrap` for epic #2 only.
- Entries keyed by session id and workflow run id; a recomputation updates only entries whose main session transcript is present, keeps the others, and drops sessions no longer named by the memlog.
- Prints the `Tokens` value (input + output + cache creation, all models, attributed steps only) for `sdlc-work` to write to the Project field.
- `make cost-report` (`scripts/cost-report.sh`): aggregates every `cost.json` as two Markdown tables, per work and per route and step.
- Shell and jq only.
- Bootstrap: `work/issue-2-sdlc-process/` with a memlog listing epic #2's sessions under the `bootstrap` step (`b8e09581`, `6b67a654`, `b0833aac`, `5ee1d1dd`, chosen by the author; `45a99285`, `4a876ab8`, `85d508f7` left out) and its `cost.json`. Must happen before those transcripts are purged (30 days by default, first session on 2026-10-16).
- Acceptance: on this session's transcript, the totals match a direct jq sum over the same files; a second run without the transcripts keeps the entries; `make cost-report` lists epic #2.
- Depends on: U8.
- Verified on 2026-10-18: `cost.json` of issue 2 equals, per model and category, a direct jq sum of the same files grouped by `message.id`; a run with `COST_TRANSCRIPTS_DIR` set to an empty folder leaves `cost.json` identical; `make cost-report` lists issue 2. Synthetic fixtures (two memlogs sharing one session) check: pre-triage, windows, declared and undeclared runs, subagents, duplicated lines, an open window closed by another work's triage, a `session` line overriding another memlog's window, a session removed from the memlog, fractional seconds, a non-UTC `at` rejected with its line, unknown keys ignored, a non-numeric issue rejected. Independent review (security, correctness, coherence) found these cases; all fixed. Found on the way: the global `claude-token-tracking` Stop hook counts every transcript line without grouping by `message.id` (about 2.2 times the real total for this session).

## Order

```text
Trunk:  U1, U2, U3, U8, U25  (independent, can start in parallel)
        → U31 (early: epic #2 transcripts expire)
        → U4, U9, U26 → U10, U11, U15
        → U5, U6, U7, U21, U22, U23, U27, U28, U29, U30  (leaves, parallel)
        → U17 → U18 → U19 → U20
        → U24, final pass on U2
Blocked (stack): U12, U13, U14, U16
```

## Open in this plan

- Milestone title field (U15): to verify.
- How to stop `bmad-build` at `ready-for-dev` (U6).
