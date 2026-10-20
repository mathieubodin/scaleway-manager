---
title: 'Process files list the new checks'
type: 'chore'
created: '2026-10-20'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-application-skeleton/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-application-skeleton/skeleton-contract.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Stories 1 to 3 add five checks, but the process documentation, the evidence record and the deliver step still describe the four old checks: the evidence record would skip the new ones and nobody would be reminded to make them required.

**Approach:** Update the three process files to the five targets and add the migration file of #41 (SPEC.md, Constraints; R1 of the premortem).

## Boundaries & Constraints

**Always:**

- The five target names exactly: `lint-go`, `test-go`, `check-go-mod`, `lint-chart`, `build-image`. `make check` runs `lint-docs`, `lint-workflows`, `test-workflows`, `check-frozen`, `lint-go`, `test-go`, `check-go-mod`, `lint-chart`; `build-image` runs in CI and by hand only.
- prek hooks after the trunk: `fix-docs`, `fix-workflows`, `check-frozen`, `lint-go`, `lint-chart`, builtin checks (`check-yaml` skipping `charts/*/templates/`).
- The module is edited in `skills/`, then reinstalled with `npx bmad-method@6.12.4 install --directory "$PWD" --custom-source "$PWD" --tools claude-code --yes`; the installer's changes are committed.
- Keep the existing wording style: plain English, one fact per sentence, no new section.

**Never:** change any Go, Dockerfile, chart, Makefile or CI file; edit `.claude/skills/` by hand; remove the behavior-diff "not run" line (#15).

</frozen-after-approval>

## Code Map

- `docs/development-process.md` line 305 -- "Code tests … come with #14": code tests now exist (`test-go`); coverage and mutation still come with #14.
- `docs/development-process.md` line 566 -- the evidence check list.
- `docs/development-process.md` lines 589 and 591, section "CI" -- `pr-checks.yml` jobs, prek hooks, what `make check` runs.
- `scripts/evidence.sh` lines 20-24 -- `checks=(...)` and `not_run=(...)` with their comment.
- `skills/sdlc-work/references/deliver.md` section 4 -- ends with "Give the user the PR link and the state of its checks".
- `.claude/skills/sdlc-work/references/deliver.md` -- installed copy, identical today.
- `docs/migrations/next/2.md`, `34.md` -- format examples.

## Tasks & Acceptance

**Execution:**

- [ ] `docs/development-process.md` -- line 305: code tests run with `make test-go`; coverage and mutation testing come with #14. Line 566: list the five new checks with the old ones; coverage, mutation and behavior diff once #14 and #15 are done. Section "CI": the ten jobs of `pr-checks.yml` and `conventional-title` (`pr-title.yml`), the prek hooks, `make check` with its eight checks, and that a new CI job becomes a required check after its PR merges.
- [ ] `scripts/evidence.sh` -- `checks` gains the five targets in this order after `check-frozen`: `lint-go test-go check-go-mod lint-chart build-image`; `not_run` becomes `Coverage, mutation testing: no target yet (#14)` and `Behavior diff: no target yet (#15)`; the comment above names #14 and #15.
- [ ] `skills/sdlc-work/references/deliver.md` -- in section 4, after the PR link line: when the PR adds a job to `.github/workflows/pr-checks.yml`, remind the user that after the merge each new job name must be added to the required status checks of `main` (`gh api -X POST repos/{owner}/{repo}/branches/main/protection/required_status_checks/contexts --input -` with the body `{"contexts":[<job names>]}`, admin rights), and record memlog result `required-checks reminder: <job names>`.
- [ ] Reinstall the module -- per Always; commit `.claude/skills/sdlc-work/references/deliver.md` and whatever else the installer changed.
- [ ] `docs/migrations/next/41.md` -- exactly `Migration: no impact`.

**Acceptance Criteria:**

- Given this story merged, when `make evidence ISSUE=41` runs, then `evidence.md` shows a section for each of the ten checks and lists only coverage, mutation testing and the behavior diff as not run.
- Given `grep -n 'four checks\|U12, stack' docs/development-process.md scripts/evidence.sh`, then nothing is printed.
- Given `diff skills/sdlc-work/references/deliver.md .claude/skills/sdlc-work/references/deliver.md`, then nothing is printed.
- Given `make check-migration ISSUE=41` and `make lint-docs`, then both exit 0.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Verification

**Commands:**

- `make lint-docs` -- expected: exit 0.
- `make check-migration ISSUE=41` -- expected: exit 0.
- `make evidence ISSUE=41` -- expected: the ten checks run, each with its output.
