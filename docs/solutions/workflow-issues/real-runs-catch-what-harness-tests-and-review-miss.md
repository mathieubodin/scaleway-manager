---
title: Run each agent workflow for real on a flawed input before calling it done
date: 2026-10-19
category: workflow-issues
module: SDLC process tooling (.claude/workflows, skills/sdlc-work, scripts)
problem_type: workflow_issue
component: development_workflow
severity: medium
applies_when:
  - "Building a Claude Code workflow (.claude/workflows/*.js) tested only with faked agent hooks"
  - "Writing skill references that agents follow across branches, clones or sessions"
  - "Writing shell and jq scripts whose output feeds a field or a decision"
  - "Harness tests reach 100 % coverage and a per-unit review passes, yet the workflow has never run for real"
symptoms:
  - "Triage rule sent 2 of 3 sample ideas to the wrong route"
  - "Token ledger charged a whole session to a new triage"
  - "make cost printed Tokens 0 with exit code 0 on a malformed line (jq 1.7)"
root_cause: missing_workflow_step
resolution_type: workflow_improvement
related_components: [testing_framework, tooling]
tags: [workflows, agents, real-run, test-harness, jq, token-cost, triage, sdlc]
---

# Run each agent workflow for real on a flawed input before calling it done

## Context

Epic #2 built the development process tooling: Claude Code workflows in `.claude/workflows/*.js`, tested by the `node:test` harness `tests/workflows/harness.js` (`make test-workflows`, c8 coverage at 100 %); the `sdlc-work` skill and its references in `skills/sdlc-work/references/`; shell and jq scripts in `scripts/`. Every unit also got an independent two-phase review (security, correctness, coherence).

The harness fakes the hooks: `agent()` returns what the test's `respond` returns, a throw stands for a dead agent, `budget` spends nothing (`tests/workflows/harness.js`, header comment). So the tests check the script's control flow against agent answers the test author imagined. They cannot see what real agents answer, what state another branch holds, or what a real tool returns. Several defects passed both tests and review and were found only by one real run (evidence: the "Done on 2026-10-19" lines of U5, U17, U18, U21, U30 and the U31 "Changed on 2026-10-19" line in `_bmad-output/planning-artifacts/plans/plan-sdlc-process-2026-10-18/plan.md`).

Earlier sessions of the same epic showed the same pattern (session history):

- In a real run, an agent with `isolation: 'worktree'` started from `main`, not from the work branch; fixed with `worktree.baseRef: "head"` in `.claude/settings.json`, and the builder's SHA check kept as a safeguard (U25).
- Biome, chosen on paper, cannot parse a workflow script's top-level `return`; workflow scripts moved to oxlint and oxfmt (U26).
- c8 reported 0 % with every test green until `.c8rc.json` matched the paths V8 reports; the 100 % was then checked against a partial run (54.83 %) (U26).
- Commands written for bash broke in zsh only when run: `"$c:refs/heads/main"` read `:r` as a modifier, `PIPESTATUS` does not exist.
- A green CI counted only after its logs were read, and after a deliberately broken PR failed each job for its own reason (U11).

## Guidance

- Before calling a workflow, skill reference or script done, run it once for real, on a small input that is deliberately flawed or ambiguous: a vague idea, a criterion with two readings, an untracked file contradicting the docs, a spec naming no file. A clean input only confirms the happy path.
- State a token budget before the run and record the cost after it (run id, agent count, subagent tokens, duration, as the plan's "Done" lines do). Real runs cost about 42,000 to 466,000 subagent tokens each in this epic.
- Clean up everything the run creates: throwaway branches never pushed, test issues deleted, worktrees, clones and fixture folders removed.
- Reproduce what the harness fakes cannot: state on another branch or in a fresh clone, real agent behaviour (what voters or readers actually answer, how many items they return), real tool exit codes and output.
- Count real routes or outputs against the expected ones (e.g. 3 ideas, 3 expected routes; how many divergences, how many of them useful). A run that "works" but returns the wrong route or noise is a failure.
- Let real runs drive rule changes, with the human: show what the run returned, decide the rule change together, rerun, record the decision.
- Check the exit code and the value of every command whose output is written somewhere, not only stderr.

## Why This Matters

Measured in this epic:

- Triage (`sdlc-triage`, U30): before the fix, 1 of 3 real routes right. A Delete bug went to long (`data` voted yes: "a deletion changes what is kept") and a typo went to bug (a typo is a defect). Harness tests were green at 100 % coverage; they could not tell that the questions were too broad.
- Token ledger (`make cost`, U17 trial): the triage of test issue #38 was charged 3.86 million tokens, because every earlier message of the session fell under rule 4 (pre-triage messages go to the triage); the other work's memlog, which would have closed the window, was on another branch. The triage itself costs about 320,000 subagent tokens (U30 figure).
- `make cost` printed `Tokens` 0 with exit code 0 on a malformed line under jq 1.7: a wrong value would have been written to the Project `Tokens` field with no failure.

Without the real runs these would have shipped: wrong routes for every bug and typo, inflated cost figures, silent zeros.

## When to Apply

- A workflow, skill or script whose behaviour depends on agent answers, on Git state outside the current branch, or on an external tool's exit code.
- Before marking a unit "done", after harness tests and review pass, not instead of them.
- After a rule change: rerun the same inputs and compare.

## Examples

**Triage rule order and the `data` question** (`.claude/workflows/sdlc-triage.js`, `docs/development-process.md` "Triage").
Tests and review saw: correct vote counting, `MIN_YES` 2 of 3, dead voters, uncertainty; 100 % coverage. Real runs on three ideas showed: YAML export → long (right), Delete bug → long (`wf_eecdfcb1-b12`), typo → bug (`wf_7ce99353-a23`). Fix, decided with the author: `data` narrowed to exports, what is sent and Secrets; `smallChange` checked before `defect`. The rule is now `newBehavior`, `adoptionKeys` or `data` → long, else `smallChange` → short, else `defect` → bug, else long (comment at the top of the file). Reruns: Delete bug → bug (`wf_8393a167-3e9`), typo → short (`wf_9cf7507c-b5d`), all votes unanimous.

**Cross-branch window and `since`** (`scripts/cost.sh`, `docs/token-cost-tracking.md` rule 4).
Tests saw: synthetic fixtures with two memlogs sharing one session, both in the same tree, and `cost.json` equal to a direct jq sum (U31, 2026-10-18). The real start of issue #38 (U17, `wf_6e51848f-054`) showed: the previous work's memlog lives on another branch, so its `step-start` window is never closed in this tree, and rule 4 gave the whole session (3.86 million tokens) to the triage. Fix: optional `since` on the triage `step-start`; `sdlc-work` sets it to the end of the step the session had open in another work; rule 4 now takes only messages not before `since`. Checked on a fixture repository: a message before `since` becomes unattributed (110 instead of 111).

**jq 1.7 exit code** (`scripts/cost.sh`, comment after `parse_events`).
Tests saw: a non-UTC `at` "rejected with its line" through `error()`; the message appeared on stderr. The real run showed `Tokens` 0 and exit code 0. Reproduced in a scratch dir with jq-1.7: an error raised after a `?` exits 0 when a later input line makes the `?` fail silently.

```sh
$ printf 'a\nb\n' | jq -R 'capture("(?<k>a)")? | error("boom")'; echo "exit=$?"
jq: error (at <stdin>:1): boom
exit=0
$ printf 'b\na\n' | jq -R 'capture("(?<k>a)")? | error("boom")'; echo "exit=$?"
jq: error (at <stdin>:2): boom
exit=5
```

Fix: a malformed `at` or `since` is returned as a `{bad: "<memlog>:<line>: ..."}` record instead of raised; the script collects `.bad` values and exits 1 naming the line.

**Gate comments and memlog in a fresh clone** (`skills/sdlc-work/SKILL.md` "Gate comments", `skills/sdlc-work/references/resume.md`, U18).
The real trial on test issue #39 (pushed branch, one unpushed commit, an agent in a fresh clone) showed that on `main` `.memlog.md` files were still ignored (`git add -f` needed; the skill now checks `git ls-files`) and listed the agents' unclear points, fixed. The independent review that followed found the gate order defect: the gate commit was made before the memlog recorded the gate, so a fresh clone saw `define` still in progress. Fix: decision `gate ... validated`, `step-end` and the next `step-start` are written first, then `make cost`, then the gate commit.

**Parallax output volume** (`.claude/workflows/sdlc-parallax.js`, U21).
Tests saw correct merging of readers' specs. The real run (`wf_1503d7de-b31`) on a vague `forged-idea.md` returned 20 divergences, many trivial. Fix with the author: each divergence gets a weight (important or detail) and a proposed answer; the rerun (`wf_2032e59a-948`) returned 9 important and 9 detail.

**Workflow lookup by name** (U5, `sdlc-spec-lint`).
The real run found that a workflow saved during a session may not be found by name before the next session (a later one was found the same day); launch it by `scriptPath` when the name is not found. No harness can see this.

**Real runs that confirmed** (U7, U28, U29): `sdlc-review` fixed a planted `route:bugfix` vs `route:bug` mismatch in an untracked file and showed that a manual `bmad-code-review` misses untracked files in uncommitted mode (`git add -N` first); `sdlc-rails` wrote two failing tests and froze them; `sdlc-build` started from the right SHA and touched no frozen test. These count as done because the outputs were counted against expected ones.

## Related

- `_bmad-output/planning-artifacts/plans/plan-sdlc-process-2026-10-18/plan.md` (U5, U7, U17, U18, U21, U28 to U31)
- `tests/workflows/harness.js`
- `docs/token-cost-tracking.md`
- `docs/development-process.md` ("Triage")
