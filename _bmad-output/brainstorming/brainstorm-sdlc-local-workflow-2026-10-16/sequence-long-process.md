---
topic: Long process, sequence centered on the Claude session
status: draft
created: 2026-10-17
updated: 2026-10-17
---

# Long process: sequence diagram

The human's Claude session runs the `/work` orchestrator. `/work` calls existing BMad skills (some with overrides in `_bmad/custom/`) and new skills of the custom module; skills start fresh subagents; `make` runs the deterministic checks; `gh` writes to GitHub. Skill and override assignments are a base to confirm at implementation.

## Building blocks

| Block | What it is | Talks with the human |
| --- | --- | --- |
| Session | The Claude conversation the human starts | Yes |
| Skill | Instructions loaded by the session, which then plays that role | Yes (it is the session) |
| Subagent | An agent started with a fresh context; works alone and returns a report | No |
| Command | A tool without AI: `make`, `git`, `gh` | No |

## Start and resume

```mermaid
sequenceDiagram
    actor H as Human
    participant W as Session running /work
    participant M as Process memlog (local, committed with normal commits)
    participant G as GitHub (gh)

    alt Start: /work
        H->>W: Idea
        W->>H: Proposed route and milestone
        H->>W: Confirms route, chooses milestone
        W->>G: Create issue or epic, route label, status
        W->>W: git: create work branch
        W->>M: Init memlog (route, step)
    else Resume: /work 42
        H->>W: /work 42
        W->>M: Read memlog (same machine)
        W->>G: Read status, gate comments, sub-issues
        W->>H: Step reached and proposed next step
    end
```

On another machine the memlog is available only once committed; otherwise `/work` resumes from the last gate recorded on GitHub.

## Long process

```mermaid
sequenceDiagram
    autonumber
    actor H as Human
    participant W as Session running /work
    participant K as Skills (BMad + new)
    participant A as Subagents (fresh)
    participant T as make
    participant G as GitHub (gh)

    rect rgba(120, 160, 255, 0.15)
    Note over H,G: Define (status Define)
    W->>K: bmad-forge-idea, then questions
    K->>H: Questions, one at a time
    H->>K: Answers
    W->>K: parallax (new skill)
    K->>A: 3 to 5 agents write the spec they would execute
    A-->>K: Divergences
    W->>K: bmad-advanced-elicitation (pre-mortem)
    K->>H: Questions from divergences and risks
    H->>K: Answers
    W->>K: bmad-deep-recon (if research needed)
    W->>K: bmad-spec
    W->>K: bmad-review with spec-lint lens (override)
    W->>K: gate page (new skill): spec.html
    end

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Spec gate
    W->>H: spec.html + SPEC.md
    H->>W: Validates or invalidates
    W->>W: git: commit spec files
    W->>G: Comment on epic with commit link, status In Progress
    end

    rect rgba(120, 160, 255, 0.15)
    Note over H,G: Plan
    W->>K: bmad-architecture (approaches, trade-offs)
    K->>A: Explorations: code, practices, frameworks
    A-->>K: Findings
    W->>K: bmad-create-epics-and-stories (tasks, contracts, trunk and leaves)
    W->>K: cold-start (new skill)
    K->>A: One fresh agent per task
    A-->>K: Missing items per task
    W->>K: gate page (new skill): plan.html
    end

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Plan gate
    W->>H: plan.html + plan
    H->>W: Validates or invalidates
    W->>W: git: commit plan files
    W->>G: Comment on epic, create sub-issues
    end

    rect rgba(120, 200, 140, 0.15)
    Note over H,G: Rails, build, verification, review
    W->>K: bmad-build (implementation_handoff override)
    K->>A: Test writer (does not see the code)
    A-->>K: Tests
    K->>T: make: record test fingerprints
    loop One builder per task, in its own worktree
        K->>A: Builder (ticket + frozen tests)
        A-->>K: Code, or failure
    end
    Note over K,A: Two failures: improve ticket, new builder. Fails again: back to Plan
    K->>T: make: checks, coverage, mutation, fingerprints
    W->>K: bmad-code-review (review_layers override)
    loop Until two dry passes with 3 lenses, at most 5 rounds
        K->>A: Reviewers (security, correctness, coherence) + verifier
        A-->>K: Verified findings
    end
    W->>K: /ce-compound
    W->>W: write docs/migrations/next/ISSUE.md
    W->>T: make: evidence record, behavior diff
    W->>G: Open PR, post evidence, behavior diff, lessons, status Review
    end

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Delivery gate
    H->>H: Plays the Gherkin scenarios (bmad-walkthrough as aid)
    H->>G: Approves and merges the PR
    W->>G: Status Done
    end
```

## Open questions

- Whether each override is enough, or a new skill is needed instead.
- Exact names of the new skills and of the custom module.
