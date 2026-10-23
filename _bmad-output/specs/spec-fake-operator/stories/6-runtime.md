---
title: 'Runtime of the fake operator'
type: 'feature'
created: '2026-10-23'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-fake-operator/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-fake-operator/fake-operator-contract.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The engine (story 3) runs one pass when called, but the binary does nothing on its own: no loop, no health endpoint, no guard against running beside the real operator.

**Approach:** Fill `run.go` of `cmd/fake-operator` (CAP-8). It runs the polling loop at `FAKE_OPERATOR_STEP_DELAY`, rereads the token at each pass, refuses to start without an API server or beside the real operator, serves `/healthz`, and stops cleanly on SIGTERM. The contract is `fake-operator-contract.md`, section "Runtime".

## Boundaries & Constraints

**Always:**

- Keep story 3's signature: `run(getenv func(string) string, stdout io.Writer, sigs <-chan os.Signal, tokenPath, caPath string, clock Clock, rnd *rand.Rand) int`.
- **Start, in order:**
  1. Logger `slog` JSON on `stdout`.
  2. `KUBERNETES_SERVICE_HOST` and `KUBERNETES_SERVICE_PORT` through `kube.NewInCluster(getenv, <caPath bytes>)`. On `ErrNoAPIServer` (unset or empty), or when the CA file is unreadable, log one ERROR and return 1.
  3. `FAKE_OPERATOR_STEP_DELAY` is a Go duration (`time.ParseDuration`), default `2s` when unset or empty. An invalid or non-positive value logs one ERROR and returns 1.
  4. Guard: `List(Deployments, "scaleway-system")`. If any item has the label `app.kubernetes.io/name: scaleway-operator`, log one ERROR naming it and return 1. A list error logs one ERROR and returns 1. The guard runs at start only.
  5. `/healthz` listens on `HEALTH_ADDR` (default `:8081`), on a `net/http` server with `ReadHeaderTimeout`. A bind failure logs one ERROR and returns 1.
- **Loop.**
  - Each pass calls `operator.pass`, which reads the token file at `tokenPath` at its start.
  - The next pass starts one delay after the previous one ends.
  - A pass error is logged and the loop goes on.
  - A failed `List` of one kind still counts as an ended pass.
- **`GET /healthz`.** It answers 503 until the first pass ends. After that, it answers 200 with body `ok` when the last pass ended less than 3 × the delay ago by the injected clock, and 503 otherwise.
- **SIGTERM** on `sigs`: cancel the context, let the in-flight request finish, shut the health server down, log INFO `stopped`, return 0.
- **Logs.** Never a token, a body or an `Authorization` header.

**Never:**

- reading the environment other than through `getenv`;
- a `time.Sleep` that tests cannot drive (the loop waits on a timer created from the injected clock's duration, and tests use a short delay);
- editing files other than `run.go`, `main.go` (only to pass `HEALTH_ADDR` through, if needed) and their tests.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| No host | `KUBERNETES_SERVICE_HOST` unset, then `""` | one ERROR line | return 1 |
| No port | `KUBERNETES_SERVICE_PORT` unset, then `""` | one ERROR line | return 1 |
| Bad delay | `FAKE_OPERATOR_STEP_DELAY=x`, `0s`, `-1s` | one ERROR line | return 1 |
| Real operator | kubetest has a Deployment labelled `app.kubernetes.io/name: scaleway-operator` in `scaleway-system` | one ERROR line naming it | return 1 |
| Other deployment | a Deployment with another label | the loop starts | N/A |
| Token rotation | the test rewrites the token file between two passes, then calls `kubetest.ExpectToken(new)` | the second pass sends the new token | N/A |
| Health before | no pass ended | `/healthz` 503 | N/A |
| Health fresh | last pass ended less than 3 × the delay ago (fixed clock) | 200, `ok` | N/A |
| Health stale | clock advanced to 3 × the delay | 503 | N/A |
| Failed list | kubetest `Fail` on instances | other kinds patched; `/healthz` 200 after that pass | logged |
| SIGTERM | signal sent during the loop | INFO `stopped` | return 0 |

</frozen-after-approval>

## Code Map

- `cmd/scaleway-manager/main.go` and `main_test.go`: the `run`, signal and `slog` patterns, and the `syncBuffer` stdout helper to copy.
- `cmd/fake-operator/operator.go` (story 3): `operator`, `pass`, `Clock`.
- `internal/kube` (story 1): `NewInCluster`, `ErrNoAPIServer`, `List(Deployments…)`.
- `cmd/fake-operator/deletion.go`, `checks.go`, `causes.go` (stories 4 and 5, built in parallel): do not edit.

## Tasks & Acceptance

**Execution:**

- [ ] `cmd/fake-operator/run.go`: start sequence, guard, loop, `/healthz`, shutdown.
- [ ] `cmd/fake-operator/run_test.go`: every matrix row. Tests start with `TestRun`, use `kubetest`, a temporary token file and CA file, `HEALTH_ADDR=127.0.0.1:0` (the bound address is logged on start, as `listening` with `addr`), a fixed or settable clock, and a delay of a few milliseconds.

**Acceptance Criteria:**

- Given the story merged, when `make lint-go test-go check-go-mod` runs, then all pass.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

`HEALTH_ADDR` exists so tests bind `127.0.0.1:0`. The Deployment of story 7 keeps the default `:8081` for its liveness probe.

## Verification

**Commands:**

- `go test ./cmd/fake-operator/...`: expected: pass.
- `make lint-go test-go check-go-mod`: expected: exit 0.
