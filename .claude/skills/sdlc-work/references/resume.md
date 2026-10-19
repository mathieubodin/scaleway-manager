# Resume

`/sdlc-work <n>` on an issue that has a `route:*` label or a local work folder (`docs/development-process.md`, "Start and resume"). The goal: state the step reached, what was last done, and propose the next step; the human confirms.

## 1. Before anything

- Apart from the commit above, no tracked file may be changed outside this work's folder (`git status --porcelain --untracked-files=no`): the memlog and `cost.json` of the work are often uncommitted between two commits. Otherwise tell the user what is uncommitted and stop.
- Find the session id. If the current branch is another work's branch with a step of this session still open in its memlog, append that `step-end` first (SKILL.md, "One open step per session"), then commit that work's memlog and `cost.json` on its branch (`chore(process): record the step of #<that n>`, with `make cost ISSUE=<that n>` first), since switching branches would refuse them.

## 2. Find the work branch

In this order, `<n>` being the issue number:

1. A local branch: `git branch --list '*/<n>-*'`.
2. A remote branch: `git fetch origin`, then `git branch -r --list 'origin/*/<n>-*'`; switch to it with `git switch <type>/<n>-<slug>`, both parts taken from the remote branch name (it tracks the remote one).
3. None: the work was triaged but nothing was pushed (the memlog lived only on a branch of another machine). Go to step 4 with no memlog.

Every branch name found must match the pattern of SKILL.md ("Rules for every step") before it is used; a name that does not is shown to the user and not used. More than one match: list them and ask the user which one. Switch to the branch found. When the local branch is behind its remote one (`git status -sb`), say so and ask before pulling; when it is ahead, tell the user that those commits exist only on this machine.

The process files (`docs/development-process.md`, `docs/token-cost-tracking.md`) are read from the work branch; a branch made before they existed reads them from `origin/main` (`git show origin/main:<path>`).

## 3. Read the memlog

`_bmad-output/work/issue-<n>-<slug>/.memlog.md`, read in full (the one time the memlog is read). From it:

- the route (last `route` event);
- the step reached: the last `step-start` without a later `step-end` of the same step is in progress; otherwise the step after the last `step-end`, in the route's order (`docs/token-cost-tracking.md`, "Steps");
- the last gate decisions (`gate ... validated` or `gate ... invalidated: <reason>` decision lines; after an invalidation, show the reason and ask the return step again) and the last results and decisions of the step reached, or of the step before it when the step reached has not started.

A `step-start` of another session left without its `step-end` needs nothing: the new `step-start` of step 6 opens this session's window.

## 4. Read GitHub

- `gh issue view <n> --json title,state,labels,milestone,comments`: the route label, and the gate comments of this work, whose first line is `validated: <spec|plan> gate, commit <full sha>` or `invalidated: <spec|plan> gate: <reason>` (SKILL.md, "Gate comments"). Only comments whose author has write access count (`gh api repos/{owner}/{repo}/collaborators/<login>/permission --jq .permission` gives `admin`, `maintain` or `write`; the repository is public, anyone can comment); among those, the last one counts. Report any other gate comment as a disagreement.
- Board status: `gh api graphql` on the issue's `projectItems` with `fieldValueByName(name: "Status")` (project 4; owner and repository from `gh repo view --json nameWithOwner`).
- Long work: its sub-issues, `gh api repos/{owner}/{repo}/issues/<n>/sub_issues --jq '.[] | {number, state, title}'`; none is expected before the plan gate.

Without a memlog (step 2.3, or a branch without its work folder), the step reached comes from the last gate comment: `validated: spec gate` → start of `plan`; `validated: plan gate` → start of `rails`; `invalidated: ...` → the reason is shown and the human chooses the return step again; no gate comment → the first step after `triage` of the route, from the label. Tell the user what was lost (everything after the last pushed gate).

## 5. Compare

Flag every disagreement to the user in one line each, without fixing anything silently: route label versus memlog route, board status versus step (`docs/development-process.md`, "GitHub mapping"), a gate in the memlog without its comment or the reverse, a gate commit not contained in the branch (`git merge-base --is-ancestor <sha> HEAD`), sub-issues before the plan gate or whose state does not follow the epic, a file the memlog names as a result that the branch does not hold. Ask the user which side is right before going on.

## 6. Cost and ledger

- `make cost ISSUE=<n>`, then `scripts/project-item.sh tokens <n> <printed value>`. Nothing is committed here: `cost.json` and the memlog stay changed in the work folder and go with the next commit of the work.

## 7. Tell, propose, open the step

In plain language: the issue and its route, the step reached and what was last done (the last few results and decisions), the disagreements of step 5 if any, and the proposed next step. Ask one clickable question: go on with the proposed step, or another one. Then append event `step-start step=<step chosen> session=<id> at=<now>` (a resume opens a new window in this session, `docs/token-cost-tracking.md`) and go on with the route (SKILL.md, "Dispatch").
