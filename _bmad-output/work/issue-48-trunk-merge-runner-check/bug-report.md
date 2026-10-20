# Bug report: #48 Process: trunk merge rule and frozen-test runner check after Rails

Source: <https://github.com/mathieubodin/scaleway-manager/issues/48>, opened by mathieubodin.

## Symptom

On the long route, the trunk cannot move as written, and a frozen test can be reached by no runner without anyone noticing. Both showed during #41.

## Steps to reproduce

1. Long route with a trunk of two or more stories, where a later story's frozen tests match the glob of a runner in `make check` (in #41: `tests/workflows/**/*.test.js`).
2. Rails freezes the tests of every story, then the first trunk story is built.
3. Merge it as `docs/development-process.md` and `skills/sdlc-work/references/long.md` (Build, trunk) say: `make check-frozen`, then `make check`.
4. Separately: a test writer adds a frozen test of a new kind (in #41: `tests/image/build-image-*.test.sh`) that no make target or CI job runs.

## Expected

- A trunk story merges when `make check-frozen` passes and every `make check` failure is a frozen test of a story not built yet; any other failure blocks; `make check` is fully green after the last story.
- Right after Rails, `sdlc-work` writes a map of each file of `frozen-tests.sha256` to the runner that reaches it and the story that makes it pass; a file with no runner is reported before the build.

## Actual

- `make check` fails after story 1 (20 failures in #41, none from story 1): read literally, the merge rule stops the trunk, or invites the builder to touch frozen tests of later stories.
- Nothing after Rails maps frozen tests to runners: five frozen tests in #41 were run by nothing.

## Where

- `docs/development-process.md`, section "Rails, build, review, delivery gate" and its sequence diagram.
- `skills/sdlc-work/references/long.md` and its installed copy `.claude/skills/sdlc-work/references/long.md`, steps Rails and Build.
- Rule and map as decided for #41: `docs/solutions/workflow-issues/map-each-frozen-test-to-its-runner-after-rails.md`.
- Not visible in the UI.

## Tests agreed

Not visible in the UI: no Gherkin scenario. One Node test file under `tests/workflows/` (run by `make test-workflows`, hence `make check`), in the style of `tests/workflows/process-files-new-checks.test.js`, checking:

1. `docs/development-process.md`, `skills/sdlc-work/references/long.md` and `.claude/skills/sdlc-work/references/long.md` describe, right after Rails, the map of each file of `frozen-tests.sha256` to its runner and its story, with files without a runner reported before the build.
2. The same files describe the trunk merge rule: `make check-frozen` passes; `make check` failures accepted only when each is a frozen test of a story not built yet according to the map; those failures listed in the memlog merge line and in the builder prompt; any other failure blocks; `make check` fully green after the last story.
3. The installed `long.md` is identical to its module source.

## Open points

None. Decided with the author: the runner check is a written step of `sdlc-work` (map in the work folder), not a script or make target.
