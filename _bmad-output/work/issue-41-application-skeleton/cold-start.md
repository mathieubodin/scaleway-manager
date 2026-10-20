# Cold start: #41

Round 1, run `wf_4df14257-2b4`: red, one blocking item, fixed (story 1: the `addr` field is the bound address). Useful items left for the builders, per story:

## Story 1

- What `run` does when `Serve` returns an error other than `http.ErrServerClosed` before any signal: not specified by the spec.
- Whether this story updates the process files: no, story 4 does.

## Story 2

- Who writes `tests/image/check-dockerfile.test.sh` and its fixtures: the test writer at Rails, against the interface `scripts/check-dockerfile.sh <file>`, exit 0 or 1.
- Whether this story updates the process files: no, story 4 does.

## Story 3

None left after the fixes.

## Story 4

None left after the fixes.
