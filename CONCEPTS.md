# Concepts

Shared domain vocabulary for this project — entities, named processes, and status concepts with project-specific meaning. Seeded with core domain vocabulary, then accretes as ce-compound and ce-compound-refresh process learnings; direct edits are fine. Glossary only, not a spec or catch-all.

## Development process

### Work

One piece of work carried from an idea or an issue to its pull request, on its own branch, with its own process memlog. Long work is an epic whose stories are sub-issues; short work and bugs are plain issues.

### Route

The path a work follows, chosen at triage and confirmed by the human: short (a small edit, a light gate), bug (tests that reproduce the defect come first), or long (a feature, with a spec gate and a plan gate before any code).

A work can be rerouted to long when it turns out bigger than expected; it keeps its branch and its memlog.

### Triage

The first step of every work: independent agents answer fixed yes-or-no questions about the idea, a fixed rule turns the answers into a proposed route, and the human confirms or changes it.

### Gate

A point where the human validates or invalidates what the agents produced, reading a page that helps understanding while a reference document stays what is decided. Long work has a spec gate, a plan gate and the delivery gate, which is the pull request; its merge is the decision.

A validated gate is recorded in the process memlog before the gate commit, so that the pushed history already shows the next step.

### Process memlog

The append-only log of one work: its route, the start and end of each step, every workflow run, the triage answers, the gate decisions and the results of the skills it called. It is the source for resuming a work and for attributing token cost to its steps.

### Step

A stage of a route (for example define, plan, rails, build, review, deliver). In the process memlog a step opens and closes a time window in one session; tokens spent in that window are counted to that step.

A session has at most one open step: it closes the step of the work it leaves before opening one in another work, because each work's memlog lives on that work's branch.

### Rails

The tests of a story or a bug, written before its code by an agent that does not see the implementation, then frozen.

### Frozen test

A test whose fingerprint is recorded so that any later change is detected; it changes only after the human decides it.
