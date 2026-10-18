---
topic: Long process, sequence centered on the Claude session
status: draft
created: 2026-10-17
updated: 2026-10-18
---

# Long process: sequence diagram

The human's Claude session runs the `sdlc-work` orchestrator. `sdlc-work` calls existing BMad skills (some with overrides in `_bmad/custom/`) and the skills of the custom module for every step that talks with the human; between two such steps it launches saved dynamic workflows (`.claude/workflows/`), whose scripts start fresh subagents and return one result; `make` runs the deterministic checks; `gh` writes to GitHub.

Corrected on 2026-10-18 with the simulation [`simulation-long.md`](../../planning-artifacts/plans/plan-sdlc-process-2026-10-18/simulation-long.md) (gap IDs long G*n*) and [`simulation-short.md`](../../planning-artifacts/plans/plan-sdlc-process-2026-10-18/simulation-short.md) (short G*n*). The version simulated is in commit `081611a`. Stages without human interaction then moved to saved dynamic workflows (participant "Workflows"; see the plan decisions). Every skill called by `sdlc-work` writes its result to the process memlog and hands control back through its `on_complete` override (long G4).

## Building blocks

| Block | What it is | Talks with the human |
| --- | --- | --- |
| Session | The Claude conversation the human starts | Yes |
| Skill | Instructions loaded by the session, which then plays that role | Yes (it is the session) |
| Workflow | A saved script (`agent()`, `parallel()`, `pipeline()`) run in the background; its loops and counters are code; returns one result | No (no input mid-run) |
| Subagent | An agent started with a fresh context, here by a workflow; works alone and returns a report | No |
| Command | A tool without AI: `make`, `git`, `gh` | No |

## Start and resume

```mermaid
sequenceDiagram
    actor H as Human
    participant W as Session running sdlc-work
    participant F as Workflows
    participant M as Process memlog (local, committed at each gate)
    participant G as GitHub (gh)

    alt Start: /sdlc-work
        H->>W: Idea
        W->>H: Questions, one at a time, code checked first
        W->>F: sdlc-triage (idea, gathered facts)
        F-->>W: Proposed route, answers, vote agreement
        W->>H: Route with answers and agreement, slug and milestone proposal (G1, G3)
        H->>W: Confirms or changes the route, chooses the milestone
        W->>M: Record triage answers and the human's decision
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
    participant F as Workflows
    participant A as Subagents (fresh)
    participant G as GitHub (gh)

    rect rgba(120, 160, 255, 0.15)
    Note over H,G: Define (status Define)
    W->>K: bmad-forge-idea (override, G5)
    K->>H: Questions, code checked before each
    H->>K: Answers
    K->>H: Each recorded assumption: yes or no?
    H->>K: Validates
    W->>F: sdlc-parallax (forged-idea.md)
    F->>A: 3 to 5 agents write the spec they would execute
    A-->>F: Specs
    F-->>W: Divergences as questions
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
        W->>F: sdlc-spec-lint (SPEC.md)
        F->>A: 3 votes per atomic question per acceptance criterion
        A-->>F: Votes
        F-->>W: Failing criteria with votes
        W->>H: One question per failing criterion
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
    participant F as Workflows
    participant A as Subagents (fresh)
    participant G as GitHub (gh)

    W->>W: Setup guard: architecture spine exists and names the stack (G13)
    rect rgba(120, 160, 255, 0.15)
    Note over H,G: Plan (status In Progress)
    W->>W: Reread spine, docs/solutions, previous specs, MemPalace (G14)
    W->>F: sdlc-explore (spec)
    F->>A: Readers: code, practices, frameworks
    A-->>F: Findings
    F-->>W: Structured map
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
        W->>F: sdlc-cold-start (story specs)
        F->>A: One fresh agent per story spec
        A-->>F: Missing items
        F-->>W: Missing items per story
        W->>H: Questions when a fix needs a decision
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
    participant F as Workflows
    participant A as Subagents (fresh)
    participant T as make
    participant G as GitHub (gh)

    rect rgba(120, 200, 140, 0.15)
    Note over H,G: Rails, build, verification, review
    W->>F: sdlc-rails, mode long (story specs)
    F->>A: One test writer per story: story spec only, no code (G20)
    A-->>F: Gherkin features, code tests
    F->>A: Run make freeze, no human check (G19)
    F-->>W: Test files written
    W->>W: Commit tests and fingerprint manifest
    W->>G: Sub-issues In Progress
    loop Trunk run, then leaves run (G21)
        W->>F: sdlc-build, mode long (story specs, work branch SHA)
        F->>A: One builder per story, own worktree: story spec + frozen tests
        A-->>F: Branch, or failure
        Note over F,A: Two failures in script: improve the story spec, new builder. Fails again: back to Plan
        F-->>W: Finished branches, failed stories
        W->>T: Merge one by one, make check-frozen after each
    end
    W->>T: make: checks, coverage, mutation, fingerprints
    W->>F: sdlc-review (dryPasses 2, maxRounds 5)
    loop In script: until two dry passes with 3 lenses, at most 5 rounds
        F->>A: Security, correctness, coherence, verifier
        A-->>F: Verified findings
        F->>A: Fixer: apply verified findings, run make
    end
    F-->>W: Converged, or recurring findings at the cap
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
