# Deliver

The last step of every route, `deliver`: migration file, evidence record, PR (`docs/development-process.md`, "Migration files", "Evidence record", "GitHub mapping"). Open it with event `step-start step=deliver session=<id> at=<now>`. Long work adds its own steps before this one (`references/long.md`).

## 1. Migration file

`docs/migrations/next/<n>.md`, `<n>` the issue (the epic for long work). Propose its content to the user with the reason: exactly `Migration: no impact` when nothing changes for an installed version (process, docs, internal code with the same behavior); otherwise `# Migration for #<n>` with `## Upgrade` and `## Rollback` sections, each with text. It goes in the confirmation of step 4.

## 2. Bug only: what was learned

Draft a short comment for the issue: the cause, the fix, and what would have caught it earlier. It goes in the confirmation of step 4.

## 3. Evidence

1. Write the migration file and commit it: `docs: add the migration note of #<n>` (`make cost` first, as for every commit).
2. `make check-migration ISSUE=<n>` must pass.
3. `make evidence ISSUE=<n>`; it fails: show the failing checks and fix them with the user before going on. Commit the record: `docs: add the evidence record of #<n>`.

## 4. PR, after the user's confirmation

Prepare, each in a temporary file outside the repository:

- the PR title, in Conventional Commits form (`<type>(<scope>): <summary>`, the CI check `conventional-title`);
- the PR body: what changes and why, in two or three sentences; `Closes #<n>` (long work: the epic first, then every sub-issue); the cost summary from `cost.json` (`Tokens`, then per step); the evidence record's summary and its path; the review result, with every unfixed finding (short route) or "none"; "Behavior diff: not run (U13, stack not chosen)" until U13 exists; the migration entry;
- for a bug, the learned comment of step 2.

Decided with the author: show the title, the body summary and, for a bug, the comment, and wait for the user's confirmation before anything leaves the machine (the repository is public). Then:

1. `git push -u origin <branch>`;
2. `gh pr create --base main --title "$(cat '<title file>')" --body-file '<body file>'`;
3. for a bug, `gh issue comment <n> --body-file '<comment file>'`;
4. `scripts/project-item.sh status <n> 'Review'`;
5. memlog result `pr <url>`, then `make cost ISSUE=<n>` and `scripts/project-item.sh tokens <n> <value>`.

Give the user the PR link and the state of its checks (`gh pr checks <url>`). The merge is theirs; Done is set by the Project workflows.

## 5. After the PR

Fixes after the PR stay in the `deliver` step: `make cost` before each commit, push, and `make evidence` again when a check changed. When this session stops working on the PR, append the `step-end` of `deliver`.
