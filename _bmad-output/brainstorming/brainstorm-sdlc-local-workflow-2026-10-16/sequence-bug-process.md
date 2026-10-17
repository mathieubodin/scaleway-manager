---
topic: Intermediate process (bug), sequence centered on the Claude session
status: draft
created: 2026-10-17
updated: 2026-10-17
---

# Intermediate process (bug): sequence diagram

No spec, no plan. The building blocks and the start and resume sequence are described in `sequence-long-process.md`.

```mermaid
sequenceDiagram
    autonumber
    actor H as Human
    participant W as Session running /work
    participant K as Skills (BMad + new)
    participant A as Subagents (fresh)
    participant T as make
    participant G as GitHub (gh)

    Note over H,G: After triage, status In Progress
    W->>H: Proposed tests: Gherkin if the bug is visible, plus a code test if relevant
    H->>W: Confirms
    W->>A: Test writer reproduces the bug
    A-->>W: Failing test or tests
    W->>T: make: tests fail for the right reason
    loop Until the bug tests pass, frozen tests intact
        W->>A: Builder (bug report + tests)
        A-->>W: Fix, or failure
    end
    Note over W,A: Two failures: improve the bug report, new builder
    opt New builder fails too
        W->>H: Switch to the long process?
        H->>W: Decides
    end
    W->>T: make: checks, coverage, mutation, fingerprints, behavior diff
    W->>K: bmad-code-review, lighter review
    K->>A: Reviewers
    A-->>K: Findings
    W->>T: make: record the bug tests as frozen tests
    W->>W: write docs/migrations/next/ISSUE.md
    W->>T: make: evidence record
    W->>G: Open PR, post evidence and behavior diff, status Review

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Gate
    H->>H: Plays the bug Gherkin scenario
    H->>G: Approves and merges the PR
    W->>G: Status Done
    end
```

## Test rules

- Bug visible in the UI: a Gherkin scenario, plus a code test when relevant (the agent proposes, the human confirms).
- Invisible bug: a code test only.
- Once the bug is fixed, its tests join the frozen tests, with their fingerprint recorded.
