---
title: Map each frozen test to its runner and story after Rails
date: 2026-10-20
category: workflow-issues
module: SDLC process (long route, sdlc-rails and sdlc-build, make check)
problem_type: workflow_issue
component: development_workflow
severity: medium
applies_when:
  - "Long route: Rails freezes the tests of every story before any story is built"
  - "Merging trunk stories one at a time with make check after each merge"
  - "A make target or CI job globs a test directory shared by several stories"
  - "A test writer adds a new kind of test file (shell, image, chart) with no existing runner"
symptoms:
  - "make check red after story 1 with 20 failures, none caused by story 1"
  - "Five frozen tests/image/build-image-*.test.sh reached by no make target, CI job or make check"
root_cause: missing_workflow_step
resolution_type: workflow_improvement
related_components: [testing_framework, tooling]
tags: [frozen-tests, rails, trunk-merge, make-check, test-runner, ci, long-route, sdlc]
---

# Map each frozen test to its runner and story after Rails

## Context

On the long route, `sdlc-rails` writes and freezes the tests of every story before any code exists. Nothing then checks how those files relate to the runners: make targets and CI jobs. In #41 the mismatch showed both ways.

- **A runner reaches tests of stories not built yet.** `make check` runs `test-workflows`, which is `c8 node --test 'tests/workflows/**/*.test.js'`. That glob picked up `tests/workflows/lint-chart.test.js` (story 3) and `tests/workflows/process-files-new-checks.test.js` (story 4). After story 1, `make check` had 20 failures, none from story 1. `docs/development-process.md` and `.claude/skills/sdlc-work/references/long.md` (Build, trunk) merge a trunk story only when `make check` passes: read literally, the trunk never moves.
- **No runner reaches a frozen test.** For the image story, the test writer, who gets only the story spec, wrote five shell tests `tests/image/build-image-*.test.sh`. No make target, no CI job and not `make check` ran them. Rails' red-before-code check runs each test once by hand, so it did not reveal the gap. One of them runs `make build-image` itself, so the `build-image` target cannot call it without recursion.

## Guidance

Right after Rails, before the build, write one map: each file of `frozen-tests.sha256`, the runner that reaches it, and the story that makes it pass. It serves two checks.

1. **Orphans.** Every frozen file has a runner. When a story introduces a new kind of test, its spec names the runner (make target or CI job), or the test writer is told the runners that exist: `make test-workflows` for `tests/workflows/**/*.test.js`, `make test-go`, `helm unittest` through `make lint-chart`, the `build-image` CI job for `tests/image/*.test.sh`.
2. **Merges.** A trunk story or a leaf merges when `make check-frozen` passes and every `make -k check` failure is a frozen test of a story not built yet, according to the map (a leaf not merged yet counts as not built yet; `-k`, so that a failing target does not hide the targets after it).
   - List those expected failures in the memlog result line of the merge.
   - Name them in the builder prompt, so the builder neither stops nor touches them.
   - Go: a frozen `*_test.go` that refers to symbols not written yet fails `go vet` and `go test` for its whole package with compile errors. That package failure is expected when every compile error points at a frozen test file of a story not built yet; the `go vet` failure is checked against the file's `make test-go` row of the map. List every compile error with `go test -gcflags=-e <package>`: `go vet` prints only the first and `go test` stops at 10, which can hide an error in a built story's file. List the masked package in the memlog line, since the tests of built stories in it run again only once every story whose frozen tests break its compilation is merged (#50).
   - A failure outside the list blocks the merge, as before.
   - After the last story (the last leaf, or the last trunk story without leaves), `make check` is fully green.

## Why This Matters

- Without the merge rule, an agent stops the trunk, or "fixes" a frozen test of a later story, which breaks `make check-frozen` and the freeze contract.
- Listing the expected failures keeps `make check` meaningful: an unexpected failure still stands out, and the count never goes up from one merge to the next.
- A frozen test that no runner reaches is a false guarantee: it counts as a rail and never fails. The gap shows only when someone asks "what runs this file?".

## When to Apply

- Long route, any trunk of two or more stories.
- Any runner that selects tests by glob rather than by story.
- Any new test type or directory (shell, chart, image tests).

## Examples

`make check` failures at each trunk merge of #41, judged against the map:

| After merge of | Failures | Source |
| --- | --- | --- |
| Story 1 | 20 | frozen tests of stories 3 and 4 |
| Story 2 | 20 | same |
| Story 3 | 6 | frozen tests of story 4 |
| Story 4 | 0 | 102 of 102 pass |

Runner added for the orphaned image tests, decided with the author: the `build-image` job of `.github/workflows/pr-checks.yml` runs `make build-image`, then every image test, and fails when one fails. Story 2's spec was amended to name it; `make check` does not run these tests.

```yaml
      - run: make build-image
      - name: Run the image tests
        run: |
          failed=0
          for t in tests/image/*.test.sh; do
            "$t" || { echo "FAILED: $t"; failed=1; }
          done
          exit "$failed"
```

## Related

- [real-runs-catch-what-harness-tests-and-review-miss.md](real-runs-catch-what-harness-tests-and-review-miss.md): same family, a process rule written on paper that only a real run tested.
- Issue #41 (epic), sub-issues #43 to #46.
