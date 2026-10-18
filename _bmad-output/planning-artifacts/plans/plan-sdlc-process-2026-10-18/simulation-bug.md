---
topic: Walk-through of the bug route on a concrete case, to find what is not yet executable
status: draft
created: 2026-10-18
case: An external user reports that a resource in Error is shown as Synced (issue #40)
inputs:
  - _bmad-output/brainstorming/brainstorm-sdlc-local-workflow-2026-10-16/sequence-bug-process.md
  - plan.md
---

# Simulation of the bug route

This file walks `sequence-bug-process.md` step by step on the case above, as written in the diagram, and marks where a step could not be executed (`G1`, `G2`…). The gaps and the decisions taken with the author follow. Nothing here has been carried into the sequence diagrams, `plan.md` or the GitHub issues yet; that happens after the author reviews the three simulation files (`simulation-long.md`, `simulation-short.md`, `simulation-bug.md`).

## Walk-through

| # | Who | What happens | Produced | GitHub |
| --- | --- | --- | --- | --- |
| 1 | External user | Opens #40: "a resource in `Error` is shown as `Synced`". | | Issue #40, status Backlog |
| 2 | Human | Types `/sdlc-work 40`. | | |
| 3 | `sdlc-work` | Reads #40, proposes the bug route ("visible defect, bounded, no new behavior"); the human confirms. | Branch `fix/40-error-status`; `work/issue-40-error-status/.memlog.md` | Label `route:bug`; status In Progress |
| 4 | `sdlc-work` | Proposes the tests: a Gherkin scenario (the bug is visible) and a code test on the status mapping; the human confirms. | | |
| 5 | Test-writer subagent | Writes the tests that reproduce the bug. **G5**: does it see the code? | `features/…/error-status.feature`, code test | |
| 6 | `make` | Runs the new tests: they must fail "for the right reason". **G2**: who judges the reason? | | |
| 7 | Builder subagent | Receives the bug report and the tests, fixes until the bug tests pass and the frozen tests stay green. **G1**: the bug tests are not frozen yet, so the builder can weaken them unseen. | Fix | |
| 8 | `sdlc-work` | After two failures: improves the bug report, starts a new builder. **G3**: where is the improved report written? The issue belongs to the external user. | | |
| 9 | `sdlc-work` | If the new builder fails too: asks the human whether to switch to the long route. | | |
| 10 | `make` | Checks, coverage, mutation, fingerprints, behavior diff. | | |
| 11 | `bmad-code-review` | "Lighter review". **G4**: lighter how? | Findings | |
| 12 | `make freeze` | Records the bug tests as frozen tests (`decisions.md`: after the fix). | Fingerprint manifest | |
| 13 | `sdlc-work` | Writes the migration file. | `docs/migrations/next/40.md` (`Migration: no impact`) | |
| 14 | `make evidence` | Rewrites the evidence record. | `work/issue-40-error-status/evidence.md` | |
| 15 | `sdlc-work` | Opens the PR ("Closes #40"), posts the evidence and the behavior diff. | | PR; status Review |
| 16 | Human | Plays the bug's Gherkin scenario by hand, approves and merges. | | Status Done |

Variant considered at step 5: a bug that only shows against real Scaleway (for example a duplicate at re-import). **G6**: the fake operator cannot reproduce it, and automated tests never reach Scaleway.

## Gaps and decisions

| ID | Gap | Decision |
| --- | --- | --- |
| G1 | Bug tests frozen only after the fix: a builder can weaken them unseen | Frozen as soon as their failure is confirmed, before any builder. Order: test writer → failure confirmed → `make freeze` → builders. Step 12 disappears. Supersedes `decisions.md`. |
| G2 | Nobody checks that tests fail for the right reason | The human, on evidence: `sdlc-work` shows each test's failure message next to the reported symptom ("expected Error, got Synced"); the human confirms yes or no before the freeze. |
| G3 | Where the improved bug report lives | `work/issue-{n}-{slug}/bug-report.md`, written at triage from the issue, enriched at each failure; the builder receives it. The user's issue is not edited; a comment summarizes what was learned. |
| G4 | "Lighter review" undefined | Same 3 lenses and verifier as the long route; one dry pass is enough; at most 2 rounds; beyond, alert and proposal as in the long route. |
| G5 | Does the test writer see the code | Yes, read-only: reproducing a bug needs the interface and entry points. It writes tests only; the builder is another subagent. Blindness stays the rule of the long route. |
| G6 | Bug reproducible only against real Scaleway | 1) Try a reproduction with the fake operator (make it write the observed status). 2) Otherwise a `@manual` scenario that the human plays before and after the fix (manual cluster target, U14). 3) If the cause is in the operator, propose moving the issue to `scaleway-operator`. |

## Resulting sequence

1. Triage, `bug-report.md` written from the issue.
2. Tests proposed and confirmed.
3. Test writer (reads the code, writes tests only).
4. Failure shown next to the symptom; the human confirms.
5. `make freeze`.
6. Builders (two-failure rule, `bug-report.md` enriched).
7. `make` checks.
8. Review: 3 lenses, verifier, 1 dry pass, at most 2 rounds.
9. Migration file, `make evidence`, PR.
10. The human plays the scenario and merges; the Project workflows set Done.

## Impact on the plan, to carry later

- `decisions.md`: bug tests frozen before the builders.
- U19: bug route as above.
- `sequence-bug-process.md`: update the diagram.
