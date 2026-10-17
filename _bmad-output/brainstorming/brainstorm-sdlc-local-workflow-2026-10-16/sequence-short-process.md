---
topic: Short process, sequence centered on the Claude session
status: draft
created: 2026-10-17
updated: 2026-10-17
---

# Short process: sequence diagram

For example a typo, or a code simplification after integration. The building blocks and the start and resume sequence are described in `sequence-long-process.md`.

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
    W->>W: Make the change on the work branch
    W->>T: make: checks, fingerprints, behavior diff
    W->>K: bmad-code-review, one pass, no loop
    K->>A: Reviewers
    A-->>K: Findings
    W->>W: write docs/migrations/next/ISSUE.md
    W->>T: make: evidence record
    W->>G: Open PR, post evidence and behavior diff, status Review

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Light gate
    H->>G: Reviews the change and the evidence record
    H->>H: Optional: plays the Gherkin scenarios if the change is bigger than expected
    H->>G: Approves and merges the PR
    W->>G: Status Done
    end
```

## Conditions

- A code simplification after integration goes through this process only if the frozen tests stay green and the behavior diff is empty.
- Work that turns out bigger than expected is rerouted to the long process.
