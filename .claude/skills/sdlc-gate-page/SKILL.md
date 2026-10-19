---
name: sdlc-gate-page
description: Renders the HTML page of a spec or plan gate (spec.html, plan.html) in a work folder, from its reference Markdown. Use when sdlc-work reaches a gate, or the user asks for a gate page.
---

# sdlc-gate-page

Renders the page the human opens at the spec gate or the plan gate of long work (`{project-root}/docs/development-process.md`, "Gates"). The page helps the human understand what they validate; the reference Markdown stays what is validated or invalidated.

## Inputs

- `path`: the work folder, `{project-root}/_bmad-output/work/issue-<n>-<slug>/`. Without it, take the folder of the active process memlog (current branch `<type>/<n>-<slug>` and that folder's `.memlog.md` exists); otherwise ask the user.
- `kind`: `spec` or `plan`. Without it, ask the user.

Read the work folder's `.memlog.md` to find the reference files: the `SPEC.md` folder (`_bmad-output/specs/spec-<slug>/`) and, for the plan gate only, the story specs. For the plan gate, the useful items of the last cold-start round are in the work folder's `cold-start.md`, written by `sdlc-work`. If a reference file of this gate is missing, stop and say which one; never render a page without its reference.

## Rules

- Every statement on the page comes from the reference files, `define-notes.md` and, for the plan gate, `cold-start.md`; add nothing of your own, no assumption, no recommendation. Only short linking words are yours, for example to put a pre-mortem answer next to the risk it answers. Where the reference leaves a point open, show it as `<span class="open">Open:</span>` followed by the point.
- Written in English (project rule), in plain language: say what a thing is for and what it changes for the user before any technical detail.
- Name every reference file by its path from the project root (an absolute path when it lies outside the project), so the human can open it.
- Start from `assets/gate-page.html`: keep its style, replace every placeholder in double curly braces, and add the sections below in that order, skipping a section only when the reference has nothing for it. Placeholders: `Gate` is `Spec` or `Plan`; `n` and the slug come from the work folder name `issue-<n>-<slug>`; `title` is the issue title, from `gh issue view <n> --json title --jq .title`, or the title of `SPEC.md` when that fails; `branch` is the current git branch.
- Escape every text copied into the page, the issue title included: `&` as `&amp;`, `<` as `&lt;`, `>` as `&gt;`, and two opening curly braces as `&#123;&#123;`. Put every table inside `<div class="wide">`.
- The page stays self-contained and inert: no script, no event attribute, and nothing loaded from elsewhere (no `<link>`, `<img>`, `<iframe>`, `src`, `srcset`, CSS `url()` or `@import`). Plain links (`<a href>`) to sources are allowed.
- Never edit a reference file.

## Sections

Spec gate, `spec.html`, from `SPEC.md` and `define-notes.md`:

1. What changes for the user.
2. Scope and non-goals.
3. Acceptance criteria: a table, one row per criterion, with how it will be verified (automated test, `@manual` scenario).
4. Decisions taken during Define: the answers of `define-notes.md` (parallax, pre-mortem, research), one line each.
5. Risks and open points.

Plan gate, `plan.html`, from the work folder's `plan.md` and `cold-start.md`, and from `stories.yaml` and the story specs in the `SPEC.md` folder:

1. Chosen approach, and the approaches set aside with the reason.
2. Build order: the sequential trunk, then the parallel leaves, as a `.flow` of `.step` boxes with their story numbers.
3. Stories: a table, one row per story spec (number, title, what it delivers, its path).
4. Contracts at each boundary between stories.
5. Useful items of the last cold-start round, per story.
6. Risks and open points.

## Output

Write `spec.html` or `plan.html` in the work folder, replacing any previous one. Then check that `grep -niE '<(script|link|img|iframe)|[[:space:]](src|srcset|on[a-z]+)=|url\(|@import' '<page>'` and `grep -n '{{' '<page>'` print nothing; fix the page until they do, without dropping any reference content (escape it instead). Print the absolute path of the page. Do not open it, commit it or comment on GitHub: `sdlc-work` runs `xdg-open` and handles the gate.
