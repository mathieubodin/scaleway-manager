---
title: Ask what is authoritative on the expected behavior before Define, and hold the validated spec after the gate
date: 2026-10-23
category: workflow-issues
module: SDLC process (long route, Define and review, docs/development-process.md)
problem_type: workflow_issue
component: development_workflow
severity: high
applies_when:
  - "Define for a feature that mimics an existing system (fake operator mimicking scaleway-operator)"
  - "Forge, parallax, pre-mortem and spec-lint working from forged-idea.md only"
  - "A review finding after the spec gate contradicts the validated spec or a frozen test"
  - "Successive review runs each find a finer fidelity gap and never converge"
symptoms:
  - "forged-idea.md stated a premise (last_synced_resource_version never written) contradicting the operator's decide_next_action_secret"
  - "No Define step read scaleway-operator v0.1.15 although the idea said the same order as the real operator"
  - "Frozen tests changed three times after the spec gate on review findings"
  - "Three sdlc-review runs never converged"
root_cause: missing_workflow_step
resolution_type: workflow_improvement
related_components: [testing_framework, documentation]
tags: [define, forge, authoritative-source, fake-operator, scaleway-operator, frozen-tests, spec-gate, sdlc]
---

# Ask what is authoritative on the expected behavior before Define, and hold the validated spec after the gate

## Context

Epic #42 built `cmd/fake-operator`, a stand-in that must behave like scaleway-operator v0.1.15 (`/home/mathieu/dev/github.com/scaleway-operator`, readable at that version with `git show scaleway-operator-v0.1.15:src/reconcilers.rs`). The process is `docs/development-process.md`; the history is in `_bmad-output/work/issue-42-fake-operator/.memlog.md`; the spec is `_bmad-output/specs/spec-fake-operator/`.

Two failures followed each other.

1. **Define never read the system the feature had to mimic.** Forge, parallax (3 agents given `forged-idea.md` only), pre-mortem (on `forged-idea.md`) and spec-lint (on `SPEC.md`; it checks clarity, not truth) never received the operator. `define-notes.md`, `forged-idea.md` and the spec memlog never mention `reconcilers.rs` or `decide_next_action`. The forged idea (`_bmad-output/forge/fake-operator/forged-idea.md`) cited the operator in its own words ("the same order as the real operator", "the effect the real operator writes") and still fixed a premise that contradicts it: "State model, no memory: removing a cause lets the object recover (Synced)" and "`last_synced_resource_version`: never written". In the operator, `decide_next_action_secret` (`reconcilers.rs` ~1662) decides on `last_synced_resource_version` against the source Secret's `resourceVersion` (step 7). Every ScalewaySecret finding of three reviews sat at that seam: leaving Revoked, source-version raise-only, null `observed_generation`, and a secret in Error with a `scaleway_id` recovering to Synced (in the operator, `AlreadySynced` never touches `sync_state`).
2. **After the spec gate, review findings overrode the validated spec.** Findings judged against the operator were taken as proof that the spec was wrong. The model recommended changing frozen tests three times: `deletion_test.go`, the Revoked case of `review_fixes_test.go`, and `causes_test.go` `TestCausesSourceVersionWaitsForSynced` (the author refused the last one). Three `sdlc-review` runs (5, 5, then 1 round) never converged.

Root cause, as stated with the author: the model carried out the frame of each step (forge questions the author, parallax compares specs, spec-lint checks wording) and never asked what the task needed first. When asked outside the process ("if I ask you to implement a service that works like the operator, what is your first reaction?"), it answered at once: read the operator at the pinned version, starting with its decision functions. The model can do this, but it does so only when asked directly. An existing doc, `docs/solutions/best-practices/verify-operator-behavior-in-the-code-that-writes-the-field.md`, already said to verify operator behavior in code. It did not prevent the miss, because no step made the agent look.

## Guidance

**Define preamble, before forge (issue #61).** Ask two generic questions whose answers depend on the feature:

- (a) What is the authority on the expected behavior: code at an exact version, a standard, a document, or the author?
- (b) Which resources should be read first to supply the missing context? Give a short ordered list to start with, not an exhaustive list of gaps.

Ask the author whether any resources are missing. The author validates the list, the agent reads the resources before forge, and forge starts from what was read. Do not add a step such as "read the operator": that step fits this feature only. The step must be the generic question.

**After the spec gate, the validated spec holds until proven otherwise.** A single finding is not that proof. A finding that contradicts the spec or a frozen test is handled one of two ways:

- dismissed, or
- recorded in the memlog with a targeted scenario or test case that would settle it, and listed in the PR.

Such a finding never changes a frozen test. Reopening the spec requires an explicit return to Define, decided by the author.

## Why This Matters

- Define's checks (parallax divergence, pre-mortem, spec-lint) only check the artifacts against each other. If the source of truth is never brought in, a premise that contradicts it passes every gate, and the cost shows up later as review findings against frozen tests.
- A written rule ("verify operator behavior in code") does nothing unless a step triggers it. Only a question asked at a fixed point in the process triggers it reliably.
- If every finding can reopen the spec, review never converges: each round reargues the spec, and frozen tests lose their purpose.

## When to Apply

- At the start of Define for any feature defined against an external authority: mimicking, emulating, faking or adopting the behavior of another system, or conforming to a standard or document.
- After the spec gate (Plan, Rails, Build, review), whenever a finding claims the spec or a frozen test is wrong.

## Examples

Preamble answers that #42 should have produced before forge:

- (a) Authority: scaleway-operator at tag `scaleway-operator-v0.1.15`.
- (b) Read first: `src/reconcilers.rs` decision functions (`decide_next_action_secret` and its siblings), then `src/resources.rs` status fields.

A finding recorded instead of applied (from the #42 memlog):

> Synced ScalewaySecret with `scaleway_id`; delete its NamespaceRole (`sync_state` Error); recreate it with role Editor. Expected per spec: Synced on the next pass. Claimed by the finding per operator v0.1.15: stays Error.

The frozen test stays unchanged. The scenario is listed in the PR, and only the author can decide on a return to Define.

## Related

- `docs/solutions/best-practices/verify-operator-behavior-in-the-code-that-writes-the-field.md`: the per-claim technique (find the line that writes the value); the Define preamble is what makes an agent apply it.
- `docs/solutions/workflow-issues/map-each-frozen-test-to-its-runner-after-rails.md`: the freeze contract at merge time.
- Issue #61: the process change.
