# Cold start: #41

Round 2, run `wf_3017d28b-290`: red, one blocking item, fixed (story 2: the docker and curl install URLs). Round 1 (`wf_4df14257-2b4`) had one blocking item, fixed (story 1: the `addr` field). Useful items left for the builders, per story:

## Story 1

- What `run` does when `Serve` returns an error other than `http.ErrServerClosed` before any signal: not specified by the spec.
- `docs/migrations/next/41.md` is written by story 4.

## Story 2

- `scripts/check-dockerfile.sh` and its test fixtures: the test writer writes `tests/image/` at Rails, against the interface `scripts/check-dockerfile.sh <file>`, exit 0 or 1.
- The process files and the migration file are updated by story 4.

## Story 3

None left after the fixes.

## Story 4

None left after the fixes.
