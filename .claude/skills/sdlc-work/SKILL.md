---
name: sdlc-work
description: Starts or resumes a piece of work through the project development process. Use when the user types /sdlc-work, wants to start work on an idea or an issue, or resume issue n.
---

# sdlc-work

Drives one piece of work from an idea to its PR, following `{project-root}/docs/development-process.md` (the process) and `{project-root}/docs/token-cost-tracking.md` (the cost ledger). Read both before acting. The process document wins over this skill when they disagree; say so to the user.

## Dispatch

- No argument: start a new piece of work from the user's idea, `references/start.md`.
- An issue number `n`:
  - a work folder `_bmad-output/work/issue-<n>-*/` exists locally, or the issue carries a `route:*` label: resume it. Resume is not available yet (epic #2, unit U18): tell the user so, and stop.
  - otherwise the issue was opened outside the process: triage it, `references/start.md`, section "External issue".
- After triage, the route goes on with its own steps: short and bug routes (unit U19), long route (unit U20). They are not available yet: tell the user which route was chosen, that its next steps are not implemented, and stop.

## Rules for every step

- Talk to the human in plain language: what a thing is for and what it changes for them before any technical detail. Ask one question at a time, each with its own context in one or two sentences; never ask what the human already decided. Real decisions are clickable choices (AskUserQuestion), the recommended one first.
- Check the code before asking a question the code can answer.
- Everything written to the repository or to GitHub is in English, whatever language the conversation uses.
- Never fill a gap with an assumption: ask, or mark it as open.
- Session id: the name, without `.jsonl`, of the newest `*.jsonl` file in `~/.claude/projects/<project>/`, where `<project>` is the absolute project root with every `/` and `.` replaced by `-`. Find it once per session.
- Times: UTC, ISO 8601, ending with `Z`: `date -u +%Y-%m-%dT%H:%M:%SZ`.
- Process memlog: `_bmad-output/work/issue-<n>-<slug>/.memlog.md`, written only through `uv run {project-root}/_bmad/scripts/memlog.py append --workspace '<work folder>' --type <type> --text '<text>'`. The text is in single quotes and contains no single quote, backtick or dollar sign. Ledger lines use `--type event` and the exact forms of `docs/token-cost-tracking.md`: `route route=<route>`, `step-start step=<step> session=<id> at=<time>`, `step-end ...`, `workflow-run run=<run id> step=<step> session=<id>`. Other facts (triage answers, decisions, results) use `--type result` or `--type decision`, one line each.
- Every workflow run is recorded with a `workflow-run` line in the memlog of its work, right after it is launched (or, at triage, as soon as the memlog exists). Launch a workflow by name; if the name is not found (a workflow saved during the current session), launch it by `scriptPath` `{project-root}/.claude/workflows/<name>.js`.
- Before every commit this skill makes: `make cost ISSUE=<n>`, add the work folder's `cost.json` to the commit, then `scripts/project-item.sh tokens <n> <printed value>`. For long work, `<n>` is the epic; sub-issues carry no `Tokens`.
- Project statuses are set with `scripts/project-item.sh status <n> '<status>'`; Done is never set by this skill.
- Commit messages follow Conventional Commits and end with `Refs: #<n>`.
