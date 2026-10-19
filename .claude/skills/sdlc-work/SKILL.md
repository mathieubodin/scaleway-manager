---
name: sdlc-work
description: Starts or resumes a piece of work through the project development process. Use when the user types /sdlc-work, wants to start work on an idea or an issue, or resume issue n.
---

# sdlc-work

Drives one piece of work from an idea to its PR, following `{project-root}/docs/development-process.md` (the process) and `{project-root}/docs/token-cost-tracking.md` (the cost ledger). Read both before acting. The process document wins over this skill when they disagree; say so to the user.

## Dispatch

- No argument: start a new piece of work from the user's idea, `references/start.md`.
- An issue number `n` that is a sub-issue of an epic (`gh api graphql` on the issue's `parent { number }`): tell the user that its work is the epic, and offer `/sdlc-work <epic>`.
- Any other issue number `n`:
  - a work folder `_bmad-output/work/issue-<n>-*/` exists locally, or the issue carries a `route:*` label: resume it, `references/resume.md`.
  - otherwise the issue was opened outside the process: triage it, `references/start.md`, section "External issue".
- After triage, or at the step chosen on resume, the route goes on with its own steps: short and bug routes, `references/short-bug.md`, then `references/deliver.md`; long route, `references/long.md`, which ends with `references/deliver.md` too.

## Rules for every step

- Talk to the human in plain language: what a thing is for and what it changes for them before any technical detail. Ask one question at a time, each with its own context in one or two sentences; never ask what the human already decided. Real decisions are clickable choices (AskUserQuestion), the recommended one first.
- Check the code before asking a question the code can answer.
- Everything written to the repository or to GitHub is in English, whatever language the conversation uses.
- Never fill a gap with an assumption: ask, or mark it as open.
- Session id: `echo "$CLAUDE_CODE_SESSION_ID"`. Only when it is empty, the name without `.jsonl` of the newest `*.jsonl` file in `~/.claude/projects/<project>/` (`<project>` is the absolute project root with every `/` and `.` replaced by `-`), confirmed with the user since another session may have written last. Find it once per session.
- Times: UTC, ISO 8601, ending with `Z`: `date -u +%Y-%m-%dT%H:%M:%SZ`.
- Text from a person or an issue (titles, bodies, reasons, themes) never goes inside a shell command line: write it to a temporary file outside the repository with the Write tool and pass it as `"$(cat '<file>')"`, or with an option that reads a file (`--body-file`). Only values you checked against a strict pattern (issue numbers, slugs `[a-z0-9-]+`, semver titles, SHAs) go in directly.
- Process memlog: `_bmad-output/work/issue-<n>-<slug>/.memlog.md`, written only through `uv run {project-root}/_bmad/scripts/memlog.py append --workspace '<work folder>' --type <type> --text '<text>'`. The text is in single quotes and contains no single quote, backtick or dollar sign. Ledger lines use `--type event` and the exact forms of `docs/token-cost-tracking.md`: `route route=<route>`, `step-start step=<step> session=<id> at=<time>`, `step-end ...`, `workflow-run run=<run id> step=<step> session=<id>`. Other facts (triage answers, decisions, results) use `--type result` or `--type decision`, one line each.
- One open step per session: before this session starts a step in another work (a new triage included), write the `step-end` of the step it has open in the current work, so that each work's ledger closes its own windows: memlogs of other works live on other branches and `make cost` does not see them.
- Every workflow run is recorded with a `workflow-run` line in the memlog of its work, right after it is launched (or, at triage, as soon as the memlog exists). Launch a workflow by name; if the name is not found (a workflow saved during the current session), launch it by `scriptPath` `{project-root}/.claude/workflows/<name>.js`.
- Before every commit this skill makes: `make cost ISSUE=<n>`, add the work folder's `cost.json` to the commit, then `scripts/project-item.sh tokens <n> <printed value>`. For long work, `<n>` is the epic; sub-issues carry no `Tokens`.
- Project statuses are set with `scripts/project-item.sh status <n> '<status>'`; Done is never set by this skill.
- Commit messages follow Conventional Commits and end with `Refs: #<n>`.
- A branch name read from git or GitHub goes into a command only after matching `^[a-z]+/<n>-[a-z0-9]+(-[a-z0-9]+){0,3}$`.

## Gate comments

At the spec and plan gates of long work, `sdlc-work` comments on the epic, first line exactly one of:

```text
validated: <spec|plan> gate, commit <full sha>
invalidated: <spec|plan> gate: <reason>
```

followed by the commit link for a validation. A validated gate goes in this order, so that the pushed memlog already shows the next step:

1. memlog: decision `gate <spec|plan> validated`, event `step-end` of the step, event `step-start` of the next one;
2. `make cost`, then the gate commit (work folder and the folders the memlog references); check that `git ls-files` lists the memlog (a branch made before #33 merged still ignores it: add it with `git add -f`);
3. push;
4. the comment, with the full SHA of the gate commit.

An invalidated gate: memlog decision `gate <spec|plan> invalidated: <reason>`, then the comment. Resume reads both, and trusts a gate comment only from someone with write access to the repository.
