---
topic: Intermediate process (bug), sequence centered on the Claude session
status: draft
created: 2026-10-17
updated: 2026-10-18
---

# Intermediate process (bug): sequence diagram

No spec, no plan. Bug tests are frozen before any builder (bug G1), which supersedes `decisions.md`. The building blocks and the start and resume sequence are described in `sequence-long-process.md`.

Corrected on 2026-10-18 with the simulation [`simulation-bug.md`](../../planning-artifacts/plans/plan-sdlc-process-2026-10-18/simulation-bug.md) (gap IDs G*n*). The version simulated is in commit `081611a`.

```mermaid
sequenceDiagram
    autonumber
    actor H as Human
    participant W as Session running sdlc-work
    participant K as Skills (BMad + new)
    participant A as Subagents (fresh)
    participant T as make
    participant G as GitHub (gh)

    Note over H,G: After triage: label route:bug, status In Progress, branch fix/n-slug
    W->>W: Write work/issue-n-slug/bug-report.md from the issue (G3)
    W->>H: Proposed tests: Gherkin if visible, plus a code test if relevant
    H->>W: Confirms
    alt Reproducible with the fake operator
        W->>A: Test writer: bug report, reads the code, writes tests only (G5)
        A-->>W: Failing tests
        W->>T: make: run the new tests
        W->>H: Failure message next to the reported symptom (G2)
        H->>W: Right reason: yes or no
    else Only against real Scaleway (G6)
        W->>A: Test writer: @manual scenario
        H->>H: Plays it on the manual cluster, confirms the failure
        opt Cause in the operator
            W->>H: Move the issue to scaleway-operator?
        end
    end
    W->>T: make freeze, before any builder (G1)
    loop Until the bug tests pass, frozen tests intact
        W->>A: Builder: bug-report.md + frozen tests
        A-->>W: Fix, or failure
    end
    Note over W,A: Two failures: enrich bug-report.md, new builder
    opt New builder fails too
        W->>H: Switch to the long route?
        H->>W: Decides
    end
    W->>T: make: checks, coverage, mutation, fingerprints, behavior diff
    loop One dry pass, at most 2 rounds (G4)
        W->>K: bmad-code-review (lenses + verifier)
        K->>A: Security, correctness, coherence, verifier
        A-->>K: Verified findings
        W->>W: Fix findings
    end
    W->>G: Comment on the issue: what was learned
    W->>W: Write docs/migrations/next/n.md
    W->>T: make evidence
    W->>G: Open PR, post evidence and behavior diff, status Review

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Gate
    H->>H: Plays the bug scenario
    H->>G: Approves and merges the PR
    G->>G: Issue closed, Project workflows set Done
    end
```
