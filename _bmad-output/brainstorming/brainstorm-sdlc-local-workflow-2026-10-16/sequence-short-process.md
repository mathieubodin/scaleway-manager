---
topic: Short process, sequence centered on the Claude session
status: draft
created: 2026-10-17
updated: 2026-10-18
---

# Short process: sequence diagram

For example a typo, or a code simplification after integration. The building blocks and the start and resume sequence are described in `sequence-long-process.md`.

Corrected on 2026-10-18 with the simulation [`simulation-short.md`](../../planning-artifacts/plans/plan-sdlc-process-2026-10-18/simulation-short.md) (gap IDs G*n*). The version simulated is in commit `081611a`.

```mermaid
sequenceDiagram
    autonumber
    actor H as Human
    participant W as Session running sdlc-work
    participant K as Skills (BMad + new)
    participant A as Subagents (fresh)
    participant T as make
    participant G as GitHub (gh)

    Note over H,G: After triage: label route:short, status In Progress, branch type/n-slug
    W->>W: Make the change on the work branch
    W->>T: make: checks, fingerprints, behavior diff
    alt Post-integration simplification
        T-->>W: Behavior diff must be empty, otherwise propose rerouting
    else Visible change (G3)
        T-->>W: Behavior diff kept for the gate
    end
    opt A frozen scenario must change
        W->>H: Change frozen test?
        H->>W: Decides
        W->>T: make freeze
    end
    W->>K: bmad-code-review, one pass
    K->>A: Reviewers + verifier
    A-->>K: Verified findings
    W->>W: Fix verified findings (G2)
    W->>T: make again, no second review
    opt Bigger than expected (G4)
        W->>H: Reroute to the long route?
        H->>W: Confirms
        W->>G: route:long + epic, status Define, comment rerouted
    end
    W->>W: Write docs/migrations/next/n.md
    W->>T: make evidence
    W->>G: Open PR, post evidence, behavior diff, unfixed findings, status Review

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Light gate
    H->>G: Reviews the change and the evidence record
    H->>H: Optional: plays the Gherkin scenarios
    H->>G: Approves and merges the PR
    G->>G: Issue closed, Project workflows set Done
    end
```
