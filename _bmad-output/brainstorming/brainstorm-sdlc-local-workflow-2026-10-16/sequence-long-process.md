---
topic: Long process, sequence centered on the Claude session
status: draft
created: 2026-10-17
updated: 2026-10-18
---

# Long process: sequence diagram

The human's Claude session runs the `sdlc-work` orchestrator. `sdlc-work` calls existing BMad skills (some with overrides in `_bmad/custom/`) and new skills of the custom module; skills and `sdlc-work` start fresh subagents; `make` runs the deterministic checks; `gh` writes to GitHub.

Corrected on 2026-10-18 with the simulation [`simulation-long.md`](../../planning-artifacts/plans/plan-sdlc-process-2026-10-18/simulation-long.md) (gap IDs long G*n*) and [`simulation-short.md`](../../planning-artifacts/plans/plan-sdlc-process-2026-10-18/simulation-short.md) (short G*n*). The version simulated is in commit `081611a`. Every skill called by `sdlc-work` writes its result to the process memlog and hands control back through its `on_complete` override (long G4).

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
    participant W as Session running sdlc-work
    participant M as Process memlog (local, committed at each gate)
    participant G as GitHub (gh)

    alt Start: /sdlc-work
        H->>W: Idea
        W->>H: Questions, one at a time, code checked first
        W->>H: Route, slug and milestone proposal (G1, G3)
        H->>W: Confirms, chooses the milestone
        opt Milestone missing (G2)
            W->>H: Theme of the milestone?
            W->>G: Create milestone, title = semver
        end
        W->>G: Create issue or epic, route label, status
        W->>W: git: branch type/n-slug
        W->>M: Init work/issue-n-slug/.memlog.md (route, step)
    else Resume: /sdlc-work 42
        H->>W: /sdlc-work 42
        W->>M: Read memlog (same machine)
        W->>G: Read status, gate comments, sub-issues
        W->>H: Step reached and proposed next step
    end
```

On another machine the memlog is available from the last gate commit; otherwise `sdlc-work` resumes from the last gate recorded on GitHub.

## Define and spec gate

```mermaid
sequenceDiagram
    autonumber
    actor H as Human
    participant W as Session running sdlc-work
    participant K as Skills (BMad + new)
    participant A as Subagents (fresh)
    participant G as GitHub (gh)

    rect rgba(120, 160, 255, 0.15)
    Note over H,G: Define (status Define)
    W->>K: bmad-forge-idea (override, G5)
    K->>H: Questions, code checked before each
    H->>K: Answers
    K->>H: Each recorded assumption: yes or no?
    H->>K: Validates
    W->>K: sdlc-parallax on forged-idea.md
    K->>A: 3 to 5 agents write the spec they would execute
    A-->>K: Divergences
    W->>K: bmad-advanced-elicitation, pre-mortem
    W->>H: Questions from divergences and risks
    H->>W: Answers, written to define-notes.md (G7)
    opt External factual question left open (G8)
        W->>H: Run research?
        H->>W: Yes or no
        W->>K: bmad-deep-recon
    end
    W->>K: bmad-spec (forged-idea.md + define-notes.md, G6)
    loop Until spec-lint is green, alert after 3 rounds (G9)
        W->>K: bmad-review, spec-lint lens
        K-->>W: Findings
        W->>H: One question per finding
        H->>W: Answers to define-notes.md, bmad-spec updates SPEC.md
    end
    W->>K: sdlc-gate-page: spec.html
    end

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Spec gate
    W->>H: xdg-open spec.html, reference SPEC.md (G12)
    H->>W: Validates, or invalidates with a reason
    alt Validated (G11)
        W->>W: Commit work folder, forge and spec folders
        W->>G: Push branch
        W->>G: Comment validated + commit link, status In Progress
    else Invalidated (G10)
        W->>G: Comment invalidated + reason
        W->>H: Proposed return step
        H->>W: Chooses the step
    end
    end
```

## Plan and plan gate

```mermaid
sequenceDiagram
    autonumber
    actor H as Human
    participant W as Session running sdlc-work
    participant K as Skills (BMad + new)
    participant A as Subagents (fresh)
    participant G as GitHub (gh)

    W->>W: Setup guard: architecture spine exists and names the stack (G13)
    rect rgba(120, 160, 255, 0.15)
    Note over H,G: Plan (status In Progress)
    W->>W: Reread spine, docs/solutions, previous specs, MemPalace (G14)
    W->>A: Explorations: code, practices, frameworks
    A-->>W: Findings
    W->>H: 2 or 3 approaches with trade-offs
    H->>W: Chooses
    opt The choice changes the spine
        W->>K: bmad-architecture
    end
    W->>W: Write work/issue-n-slug/plan.md (G15)
    W->>K: bmad-spec Story Breakdown: stories.yaml (G16)
    K->>H: One story per reviewable slice, checkpoints
    loop For each story (G17)
        W->>K: bmad-build, steps 01 and 02 only
        K-->>W: stories/id-slug.md, status ready-for-dev
    end
    loop Until cold-start is green, alert after 3 rounds (G18)
        W->>K: sdlc-cold-start
        K->>A: One fresh agent per story spec
        A-->>K: Missing items
        W->>W: Fix plan.md, stories.yaml or story specs
    end
    W->>K: sdlc-gate-page: plan.html
    end

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Plan gate
    W->>H: xdg-open plan.html, reference plan.md + story specs
    H->>W: Validates, or invalidates with a reason
    alt Validated
        W->>W: Commit, push
        W->>G: Comment validated + commit link
        W->>G: One sub-issue per story spec
    else Invalidated
        W->>G: Comment invalidated + reason
        W->>H: Proposed return step
        H->>W: Chooses the step
    end
    end
```

## Rails, build, review, delivery gate

```mermaid
sequenceDiagram
    autonumber
    actor H as Human
    participant W as Session running sdlc-work
    participant K as Skills (BMad + new)
    participant A as Subagents (fresh)
    participant T as make
    participant G as GitHub (gh)

    rect rgba(120, 200, 140, 0.15)
    Note over H,G: Rails, build, verification, review
    loop For each story (G20)
        W->>A: Test writer: story spec only, no code
        A-->>W: Gherkin features, code tests
    end
    W->>T: make freeze, no human check (G19)
    W->>W: Commit tests and fingerprint manifest
    W->>G: Sub-issues In Progress
    par Leaves in parallel, one worktree each (G21)
        W->>A: Builder: story spec + frozen tests
        A-->>W: Code, or failure
    end
    Note over W,A: Merged one by one into the work branch. Two failures: improve the story spec, new builder. Fails again: back to Plan
    W->>T: make: checks, coverage, mutation, fingerprints
    loop Until two dry passes with 3 lenses, at most 5 rounds
        W->>K: bmad-code-review (override: lenses + verifier)
        K->>A: Security, correctness, coherence, verifier
        A-->>K: Verified findings
        W->>W: Fix findings
    end
    opt Not converged after 5 rounds (G23)
        W->>H: Recurring findings: back to Plan, back to Define, or accept and list in PR
        H->>W: Chooses
    end
    W->>K: /ce-compound
    W->>W: Update MemPalace
    W->>G: Issues simplify after n and distill lessons, Backlog (short G1)
    W->>W: Write docs/migrations/next/n.md
    W->>T: make evidence, behavior diff
    W->>G: Open PR, Closes epic + all sub-issues (G22)
    W->>G: Post evidence, behavior diff, lessons, status Review
    end

    rect rgba(255, 180, 80, 0.2)
    Note over H,G: Delivery gate
    H->>H: Plays the Gherkin scenarios (bmad-walkthrough as aid)
    H->>G: Approves and merges the PR
    G->>G: Issues closed, Project workflows set Done (G24)
    end
```
