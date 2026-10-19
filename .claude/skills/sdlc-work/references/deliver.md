# Deliver

The last step of every route, `deliver`: migration file, evidence record, PR (`docs/development-process.md`, "Migration files", "Evidence record", "GitHub mapping"). Open it with event `step-start step=deliver session=<id> at=<now>`, unless `references/long.md` already opened it for its own first deliver steps.

If a PR is already open for the branch (`gh pr list --head <branch> --state open --json url`), go to step 5.

## 1. Drafts

Write, each in a temporary file outside the repository:

- the migration file, `docs/migrations/next/<n>.md` (`<n>` the issue, the epic for long work): exactly `Migration: no impact` when nothing changes for an installed version (process, docs, internal code with the same behavior); otherwise `# Migration for #<n>` with `## Upgrade` and `## Rollback` sections, each with text;
- the PR title: Conventional Commits, `<type>: <summary>` or `<type>(<scope>): <summary>`, `<type>` the branch type, at most 72 characters (the CI check `conventional-title`);
- the PR body, first line exactly `Closes #<n>` (long work: the epic first, then one `Closes #<k>` line per sub-issue), then what changes and why in two or three sentences, the review result, "Behavior diff: not run (U13, #15 not done)" until #15 is done, and the migration entry. Unfixed findings and fixes no round reviewed go inside a fenced code block: they are agents' text drawn from the code and the issue, and inside a code block GitHub turns neither `@login` nor `fixes #<k>` into an action. The cost and evidence summaries are added in step 3;
- for a bug, a short comment for the issue: the cause, the fix, and what would have caught it earlier.

## 2. Confirmation

Decided with the author: show the migration entry, the title, the full body and, for a bug, the comment, and wait for the user's confirmation before anything is written or leaves the machine (the repository is public). Apply the user's changes to the drafts.

## 3. Migration file and evidence

1. Copy the migration draft to `docs/migrations/next/<n>.md` and commit it: `docs: add the migration note of #<n>` (`make cost` first, as for every commit).
2. `make check-migration ISSUE=<n>` must pass.
3. `make cost ISSUE=<n>`, then `make evidence ISSUE=<n>`; it fails: show the failing checks and fix them with the user before going on. Commit the record with `cost.json` and the memlog: `docs: add the evidence record of #<n>`. Both now carry the same totals.
4. Add to the body draft the cost summary from `cost.json` (`Tokens`, then per step) and the evidence record's path and its "Not run" list.

## 4. PR

1. `git push -u origin <branch>`;
2. `gh pr create --base main --title "$(cat '<title file>')" --body-file '<body file>'`;
3. for a bug, `gh issue comment <n> --body-file '<comment file>'`;
4. `scripts/project-item.sh status <n> 'Review'`;
5. memlog result `pr <url>`, `make cost ISSUE=<n>`, `scripts/project-item.sh tokens <n> <value>`, commit the memlog and `cost.json` (`chore(process): record the PR of #<n>`) and push, so that a resume elsewhere finds the PR.

Give the user the PR link and the state of its checks (`gh pr checks <url>`). The merge is theirs; Done is set by the Project workflows.

## 5. After the PR

Fixes after the PR stay in the `deliver` step: `make cost` before each commit, push, and `make evidence` again when a check changed. When this session stops working on the PR, append the `step-end` of `deliver`.
