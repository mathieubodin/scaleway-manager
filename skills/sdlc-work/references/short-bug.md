# Short and bug routes

From the confirmed route to the PR (`docs/development-process.md`, "Bug route" and "Short route"). Each step opens with event `step-start step=<step> session=<id> at=<now>` and closes with its `step-end`; every workflow run gets its `workflow-run` line right after launch. Make no commit while a workflow runs.

## Prompts from the overrides

The workflows get their prompts from the BMad overrides, resolved once per session (the fields are under `.workflow`):

- builder and test writers: `uv run {project-root}/_bmad/scripts/resolve_customization.py --skill {project-root}/.claude/skills/bmad-build --project-root {project-root} --key workflow`, fields `sdlc_bug_test_writer_prompt` and `sdlc_builder_prompt`;
- review: the same with `--skill {project-root}/.claude/skills/bmad-code-review`, fields `review_layers` (keep the entries whose `instruction` is not empty, as `{id, name, instruction}`), `sdlc_verifier_prompt`, `sdlc_fixer_prompt`.

Extract them with jq into the workflow `args`; never retype them.

## Bug route

### Rails (step `rails`)

1. Read `bug-report.md`. Propose the tests to the user: a Gherkin scenario when the bug is visible in the UI, plus a code test when relevant; a code test only otherwise. One clickable confirmation.
2. A bug that only shows against real Scaleway: first look for a way to reproduce it with the fake operator. Otherwise the test is a `@manual` scenario, which no runner plays: after `sdlc-rails` writes it, the user plays it on the manual cluster and confirms that it fails for the reported reason (step 4). If the cause is in the operator, propose moving the issue to `scaleway-operator`; after the user agrees, `gh issue transfer` it and stop the route here.
3. Launch `sdlc-rails` with `{ mode: "bug", items: ["<work folder>/bug-report.md"], testWriterPrompt: <sdlc_bug_test_writer_prompt> }`.
4. Show each test with its failure message next to the reported symptom (for a `@manual` scenario, the user's own run), and ask the user whether it fails for the reported reason. A test that does not fail, or fails for another reason: say so, delete the test files of that round (they are new and untracked), add what was wrong to `bug-report.md` ("Open points"), and launch `sdlc-rails` again; after a second wrong round, ask the user how to go on.
5. Once the user confirms: `make freeze FILES='<test files>'` (each name checked against `^\w[\w.-]*(/\w[\w.-]*)*$`), write the memlog result `bug tests confirmed: <files>`, then commit exactly those test files, `frozen-tests.sha256` and the work folder: `test: reproduce #<n> <short title>`.

### Build (step `build`)

1. `baseSha` = `git rev-parse HEAD`. Launch `sdlc-build` with `{ mode: "bug", items: ["<work folder>/bug-report.md"], baseSha, builderPrompt: <sdlc_builder_prompt> }`.
2. Finished: its `branch` must match `^worktree-[A-Za-z0-9_-]+$`; otherwise show it to the user and do not use it. When `ticketImproved` is true, commit `bug-report.md` first. Then `git merge --no-ff --no-commit <branch>`, `make check-frozen` and `make check`; both pass: `make cost`, add `cost.json` and the memlog, commit `fix: merge the fix of #<n>`. A check fails: `git merge --abort` and tell the user. Then remove its worktree (its path is the line of `git worktree list --porcelain` whose branch is `refs/heads/<branch>`) and `git branch -d <branch>`.
3. Failed with outcome `ask the human`: show both reasons and the improvement, and ask whether to switch to the long route (`start.md`, "Rerouting") or stop. Outcome `wrong base`: something was committed during the run; say so and launch again. The worktrees listed are kept for the user to look at.

### Review (step `review`)

1. `base` = `git merge-base origin/main HEAD`; `diffDir` = a new folder from `mktemp -d` (outside the repository). Launch `sdlc-review` with `{ base, diffDir, specPath: "<work folder>/bug-report.md", layers, verifierPrompt, fixerPrompt, dryPasses: 1, maxRounds: 2 }`.
2. Memlog result: `review converged=<true|false> rounds=<r> unfixed=<count>`.
3. The fixer leaves its fixes uncommitted. Run `make check`; it passes: commit them with the memlog and `cost.json`, `fix: apply the review of #<n>`; no fix: commit the memlog and `cost.json`, `chore(process): record the review of #<n>`. `make check` fails: tell the user and fix with them.
4. Not converged: show the recurring and unfixed findings in plain language and propose a way out (fix by hand now, list them in the PR, or switch to the long route); the user chooses. A finding that contradicts `bug-report.md` or a frozen test is never fixed by hand and never changes a frozen test: the user dismisses it, or it is recorded as in `long.md`, "Review", and listed in the PR. The fixes of the last round that no round reviewed (`unreviewedFixes`) go to the PR body.

Then `references/deliver.md`.

## Short route

### Change (step `change`)

1. The session makes the change itself on the work branch, without a subagent.
2. A frozen test that must change: explain why and ask the user; only after a yes, change it and `make freeze FILES='<file>'`.
3. `make check`. The behavior diff (#15) does not exist yet: nothing to compare; the evidence record says it was not run.
4. Commit with the type planned for the PR (the branch type): `<type>: <short title>`.
5. The change turns out bigger than expected (new behavior, adoption keys, exports or Secrets touched): propose rerouting (`start.md`, "Rerouting").

### Review (step `review`)

1. Launch `sdlc-review` as in the bug route, with `specPath` the issue text written to a temporary file outside the repository, and `singlePass: true` instead of `dryPasses` and `maxRounds`.
2. A single pass converges only when it finds nothing: here `converged` false is normal, and there is no way out to propose. Memlog result and commit as in the bug route, steps 2 and 3.
3. The findings left unfixed (`unfixed`) and the fixes no round reviewed (`unreviewedFixes`) go to the PR body. A finding that contradicts the issue text or a frozen test is never fixed by hand and never changes a frozen test: the user dismisses it, or it is recorded as in `long.md`, "Review", and listed in the PR.

Then `references/deliver.md`.
