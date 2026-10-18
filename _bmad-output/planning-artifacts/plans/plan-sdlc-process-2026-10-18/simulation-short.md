---
topic: Walk-through of the short route on a concrete case, to find what is not yet executable
status: draft
created: 2026-10-18
case: Remove the duplication between export and re-import after #27 is merged (issue #32)
inputs:
  - _bmad-output/brainstorming/brainstorm-sdlc-local-workflow-2026-10-16/sequence-short-process.md
  - plan.md
---

# Simulation of the short route

This file walks `sequence-short-process.md` step by step on the case above, as written in the diagram, and marks where a step could not be executed (`G1`, `G2`…). The gaps and the decisions taken with the author follow. Nothing here has been carried into the sequence diagrams, `plan.md` or the GitHub issues yet; that happens after the author reviews the three simulation files (`simulation-long.md`, `simulation-short.md`, `simulation-bug.md`).

The case is the post-integration simplification named in `decisions.md`: the frozen tests must stay green and the behavior diff must be empty.

## Walk-through

| # | Who | What happens | Produced | GitHub |
| --- | --- | --- | --- | --- |
| 0 | — | The epic #27 (YAML export) has just been merged. **G1**: nothing starts the after-integration steps (simplify, turn lessons into rules, update the memory); the session that ran #27 may be gone. | | |
| 1 | Human | Types `/sdlc-work`, then "remove the duplication between export and re-import". | | |
| 2 | `sdlc-work` | Proposes the short route ("no behavior change, covered by frozen tests"); the human confirms. | Branch `refactor/32-simplify-export`; `work/issue-32-simplify-export/.memlog.md` | Issue #32, label `route:short`, status In Progress |
| 3 | `sdlc-work` | Makes the change itself on the work branch, without a subagent. | Code change | |
| 4 | `make` | Checks, fingerprints, behavior diff: frozen tests green, diff empty. | | |
| 5 | `bmad-code-review` | One pass, no loop. **G2**: what happens to its findings? | Findings | |
| 6 | `sdlc-work` | Writes the migration file. | `docs/migrations/next/32.md` (`Migration: no impact`) | |
| 7 | `make evidence` | Rewrites the evidence record. | `work/issue-32-simplify-export/evidence.md` | |
| 8 | `sdlc-work` | Opens the PR ("Closes #32"), posts the evidence and the behavior diff. | | PR; status Review |
| 9 | Human | Light gate: reviews the change and the evidence record; plays the Gherkin scenarios only if the change looks bigger than expected; approves and merges. | | Status Done |

Variants considered:

- Step 4 for a visible short change (for example a label rename): **G3**: the behavior diff is not empty, and the diagram does not say what follows.
- Any step: the change turns out bigger than expected. **G4**: rerouting to the long route is not described (labels, status, branch, memlog).

## Gaps and decisions

| ID | Gap | Decision |
| --- | --- | --- |
| G1 | Nobody starts the after-integration steps of a long piece of work once it is merged | Before opening the PR of a long piece of work, `sdlc-work` creates a `route:short` issue "simplify after #n" (with the leads seen in review) and, when recurring lessons exist, a "distill lessons" issue. Both wait in Backlog. The project memory (MemPalace) is updated before the PR. |
| G2 | Findings of the single review pass | Verified findings are fixed, `make` runs again (frozen tests, behavior diff), no second review. Findings left unfixed are listed in the PR for the light gate. |
| G3 | Behavior diff not empty for a visible short change | Posted on the PR; the merge accepts it and the capture becomes the new reference. Only a post-integration simplification requires an empty diff (otherwise rerouting is proposed). A frozen scenario that must change still needs the human's decision, then `make freeze`. |
| G4 | Rerouting a short or bug piece of work to the long route | Same issue promoted after the human's confirmation: `route:short` or `route:bug` → `route:long` + `epic`, status Define, comment "rerouted: reason". Branch and memlog kept; the work already done is an input to forge. Branch renamed when its type changes (for example `fix/` → `feat/`). |

## Resulting sequence

1. Triage (the issue may already exist, created before the PR of the long work it follows).
2. Change by the session.
3. `make` checks; behavior diff empty for a simplification, shown at the gate otherwise.
4. One review pass; verified findings fixed; `make` again; unfixed findings listed for the gate.
5. Migration file, `make evidence`, PR.
6. Light gate; the merge sets Done through the Project workflows.

## Impact on the plan, to carry later

- U19: short route as above.
- U20: after-integration issues created before the PR of a long piece of work.
- U17, U19, U20: rerouting to the long route.
- `sequence-short-process.md`: update the diagram.
