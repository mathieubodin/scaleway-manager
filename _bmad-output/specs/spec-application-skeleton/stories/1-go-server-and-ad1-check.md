---
title: 'Go server and AD-1 check'
type: 'feature'
created: '2026-10-20'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-application-skeleton/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-application-skeleton/skeleton-contract.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The repository has no Go code: there is no server for the image (story 2) or the chart (story 3), and nothing enforces AD-1 (standard library only).

**Approach:** Add the module, the server (CAP-1, CAP-2) in the spine layout, the AD-1 check program, and the make targets, CI jobs and prek hook that run them (CAP-5, Go part).

## Boundaries & Constraints

**Always:**

- Go 1.27.2, standard library only: `go.mod` is `module github.com/mathieubodin/scaleway-manager` plus `go 1.27.2`, nothing else; no `go.sum`.
- Every response, 404 and 405 included, carries `Content-Security-Policy: default-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'` and `X-Content-Type-Options: nosniff`: one middleware wraps the whole mux.
- Every line the server writes is one `log/slog` JSON object on stdout; `http.Server.ErrorLog` is `slog.NewLogLogger(<that handler>, slog.LevelWarn)`.
- `ReadHeaderTimeout` 10 s; no `WriteTimeout`.
- Makefile: each public target has a `##` comment and is in `.PHONY`; tool guard `check-go` (no `##`) prints `Error: go not found. Install Go 1.27.2: https://go.dev/dl/` and exits 1; `check` gains `lint-go test-go check-go-mod`.
- CI jobs named exactly `lint-go`, `test-go`, `check-go-mod`, following the existing jobs (checkout `3d3c42e5aac5ba805825da76410c181273ba90b1` with `persist-credentials: false`), plus `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0` with `go-version-file: go.mod` and `cache: false`.

**Never:** a dependency or `require`; `http.FileServer` or `ServeFileFS` for the page; `ListenAndServe`; calling the stop function of `signal.NotifyContext` before exit; a per-request log line; any Kubernetes call; a Go file in `web/`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Health | `GET /healthz` | 200, body `ok`, no newline | N/A |
| Health HEAD | `HEAD /healthz` | 200, empty body | N/A |
| Page | `GET /` | 200, bytes of `web/index.html` | N/A |
| Unknown | `GET /nope` | 404 with both headers | N/A |
| Address | `LISTEN_ADDR` unset or `""` | listens on `:8080` | N/A |
| Start | listen succeeds | one INFO line, `msg` `listening`, field `addr` = the bound address `ln.Addr().String()` (so `127.0.0.1:0` logs the real port) | N/A |
| Bind failure | address in use | one ERROR line, `msg` `listen failed`, field `error`; no `listening` line | exit 1 |
| Serve failure | `Serve` returns an error other than `http.ErrServerClosed` before any signal | one ERROR line, `msg` `serve failed`, field `error`; no `stopped` line | exit 1 |
| Stop | first SIGTERM or SIGINT, a request in flight | the request completes; INFO `stopped` | exit 0 |
| Slow stop | `Shutdown` not done after the delay | one ERROR line, `msg` `shutdown timed out`; then `srv.Close()`; no `stopped` line | exit 1 |
| Second signal | signal during shutdown | no line, no change | same exit code |
| AD-1 | fixture with a `require` line, a `require` block, a non-empty `go.sum`, a clean module | fails naming `go.mod`, `go.mod`, `go.sum`; passes | exit 1 on failure |

</frozen-after-approval>

## Code Map

- `Makefile` -- add `check-go`, `lint-go`, `test-go`, `check-go-mod`; extend `check` and `.PHONY`; follow `check-node` for the guard.
- `prek.toml` -- add local hook `lint-go` (`name = "Go lint"`, `entry = "make lint-go"`, `files = "(\\.go|^go\\.mod)$"`, `language = "system"`, `pass_filenames = false`), like `fix-workflows`.
- `.github/workflows/pr-checks.yml` -- add three jobs, one per target, like `check-frozen`.
- `.gitignore` -- add the binary name `/scaleway-manager` if `go build` writes it at the root.

## Tasks & Acceptance

**Execution:**

- [ ] `go.mod` -- module and `go 1.27.2` only.
- [ ] `web/index.html` -- `<html lang="en">`, title `scaleway-manager`, `<meta name="viewport" content="width=device-width, initial-scale=1">`, the sentence `scaleway-manager is running; no feature is available yet.`, no script, no inline style.
- [ ] `embed.go` (module root, package `scalewaymanager`) -- `//go:embed web/index.html` into `var IndexHTML []byte`.
- [ ] `internal/server/server.go` -- `New(index []byte) http.Handler`: mux with `GET /healthz` and `GET /{$}` (bytes written directly), wrapped by the header middleware.
- [ ] `internal/server/server_test.go` -- `httptest` cases of the matrix rows Health to Unknown, headers on each, and on a 405 (`POST /healthz`).
- [ ] `cmd/scaleway-manager/main.go` -- `main` calls `os.Exit(run(os.Getenv, os.Stdout, sigs, server.New(scalewaymanager.IndexHTML), shutdownTimeout))` with `const shutdownTimeout = 10 * time.Second` and `sigs` from `signal.Notify(SIGTERM, SIGINT)`, never stopped; `run` does `net.Listen`, the `listening` line, `Serve`, and on the first signal `Shutdown` with a fresh `context.WithTimeout(context.Background(), delay)`; later signals are drained and ignored.
- [ ] `cmd/scaleway-manager/main_test.go` -- drives `run` with a signal channel, a slow test handler and a short delay: matrix rows Address to Second signal (Serve failure: the test closes the listener `run` uses, through an injected `net.Listener` factory or by passing a listener), every stdout line parsed as JSON, ERROR count per case, and `shutdownTimeout == 10*time.Second`.
- [ ] `cmd/check-go-mod/main.go` -- `check(dir string) error` runs `go mod edit -json <dir>/go.mod` (`os/exec`), fails on any `Require` with `go.mod: require found`, fails when `<dir>/go.sum` exists with size > 0 with `go.sum: not empty`; `main` checks `.`, prints the error, exits 1.
- [ ] `cmd/check-go-mod/main_test.go` and `cmd/check-go-mod/testdata/{require-line,require-block,gosum,clean}/` -- the AD-1 row; each fixture `go.mod` has `module example.com/fixture` and `go 1.27.2`.
- [ ] `Makefile`, `prek.toml`, `.github/workflows/pr-checks.yml` -- per Code Map; `lint-go` = `test -z "$$(gofmt -l $$(git ls-files '*.go'))"` (tracked files only, so other worktrees and `node_modules` are skipped; print the list on failure) then `go vet ./...`; `test-go` = `go test ./...`; `check-go-mod` = `go run ./cmd/check-go-mod`.

**Acceptance Criteria:**

- Given this story merged, when `make lint-go test-go check-go-mod` and `make check` run, then all pass.
- Given a scratch `require` added to `go.mod`, when `make check-go-mod` runs, then it exits non-zero naming `go.mod` (real run, then reverted).
- Given an unformatted `.go` file, when `make lint-go` runs, then it exits non-zero listing the file (real run, then reverted).
- Given `go` absent from `PATH`, when any of the three targets runs, then it prints the install hint and exits 1.
- Given `make help`, then `lint-go`, `test-go`, `check-go-mod` are listed.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

`run(getenv func(string) string, stdout io.Writer, sigs <-chan os.Signal, h http.Handler, delay time.Duration) int` keeps the lifecycle testable in-process: tests inject the handler (a slow one for the in-flight and slow-stop rows) and a short delay instead of 10 s. The bind-failure test first listens on `127.0.0.1:0` and passes that address. The default-address row is checked through `listenAddr(getenv func(string) string) string`, which `run` uses: unset or `""` gives `:8080`, any other value is returned as is.

## Verification

**Commands:**

- `make lint-go test-go check-go-mod` -- expected: exit 0.
- `make check` -- expected: exit 0.
- `go build ./cmd/scaleway-manager && LISTEN_ADDR=127.0.0.1:18080 ./scaleway-manager & curl -si 127.0.0.1:18080/healthz` -- expected: 200, both headers, body `ok`.
