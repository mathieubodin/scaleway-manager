# Cold start: #41

Round 3, run `wf_4ef327c6-39d`: green, no blocking item. Rounds 1 (`wf_4df14257-2b4`) and 2 (`wf_3017d28b-290`) each had one blocking item, fixed in the story specs. Useful items of round 3, per story:

## Story 1

- What `run` does when `Serve` returns an error other than `http.ErrServerClosed` before any signal: closed after the round, with the author (one ERROR line `serve failed`, exit 1).
- How `cmd/check-go-mod` reports a module with both a `require` and a non-empty `go.sum`, and whether it prints to stdout or stderr.
- The unformatted file of the `lint-go` criterion must be tracked by git, since `lint-go` checks `git ls-files '*.go'`; with no tracked `.go` file, `gofmt -l` with no argument reads stdin.
- The process files and `docs/migrations/next/41.md` belong to story 4.

## Story 2

- What `healthz: status=<code> body=<body>` prints when no response ever arrived (for example `status=000` from curl).
- The success line of `scripts/build-image.sh`.
- `tests/image/check-dockerfile.test.sh` and its fixtures are written and frozen at Rails; the builder follows them.
- A per-attempt curl timeout, so polling stays within 30 s.
- `.dockerignore`: `*.md` or `**/*.md`; the build works with either.

## Story 3

- The exact wording of the missing-plugin hint of `check-helm`.
- Whether the ClusterRoles render `rules: []` or no `rules` key.
- Story 1's tool guard and prek hook are the pattern to copy once story 1 is merged.

## Story 4

- The job names added by stories 1 to 3 are the five target names; the builder checks `pr-checks.yml` once the trunk is merged.
