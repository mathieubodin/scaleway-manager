# Runner map: #42

One row per file of `frozen-tests.sha256`, after Rails (`wf_2286961f-b30`). The rows give the runner that reaches the file and the story whose code makes it pass. Files frozen before #42 are already green; they are listed so that every `make -k check` failure can be judged against this map (`docs/development-process.md`, Build).

## Frozen by #42

| File | Runner | Story |
| --- | --- | --- |
| `internal/kube/token_test.go` | `make test-go` (`make lint-go` vets it) | S1 (#54) |
| `internal/kube/resource_test.go` | `make test-go` | S1 (#54) |
| `internal/kube/client_test.go` | `make test-go` | S1 (#54) |
| `internal/kube/imports_test.go` | `make test-go` | S1 (#54) |
| `internal/kube/kubetest/helpers_test.go` | `make test-go` | S2 (#55) |
| `internal/kube/kubetest/read_test.go` | `make test-go` | S2 (#55) |
| `internal/kube/kubetest/patch_test.go` | `make test-go` | S2 (#55) |
| `internal/kube/kubetest/delete_test.go` | `make test-go` | S2 (#55) |
| `internal/kube/kubetest/auth_test.go` | `make test-go` | S2 (#55) |
| `internal/kube/kubetest/fault_test.go` | `make test-go` | S2 (#55) |
| `cmd/fake-operator/lifecycle_test.go` | `make test-go` | S3 (#56) |
| `cmd/fake-operator/writes_test.go` | `make test-go` | S3 (#56) |
| `cmd/fake-operator/deletion_test.go` | `make test-go` | S4 (#57) |
| `cmd/fake-operator/checks_test.go` | `make test-go` | S4 (#57) |
| `cmd/fake-operator/causes_test.go` | `make test-go` | S5 (#58) |
| `cmd/fake-operator/run_test.go` | `make test-go` | S6 (#59) |
| `tests/make/fake_operator_test.go` | `make test-go` | S7 (#60) |
| `tests/image/build-fake-operator-image.test.sh` | CI job `build-image` of `.github/workflows/pr-checks.yml` (runs every `tests/image/*.test.sh`) | S7 (#60) |
| `cmd/fake-operator/events_test.go` | `make test-go` | Review (e402ae2) |
| `cmd/fake-operator/review_fixes_test.go` | `make test-go` | Review (0ee55fa) |
| `cmd/fake-operator/review_fixes2_test.go` | `make test-go` | Review (2e21c34) |
| `cmd/fake-operator/review_fixes3_test.go` | `make test-go` | Review (9fb166e, then the way out of the second review) |

Every `*_test.go` above is also compiled by `make lint-go` (`go vet ./...`). A `make lint-go` failure of one of these files is judged against its `make test-go` row.

## Frozen before #42 (green)

| Files | Runner |
| --- | --- |
| `charts/scaleway-manager/tests/*_test.yaml` | `make lint-chart` |
| `cmd/check-go-mod/main_test.go` and its `testdata/` | `make test-go` |
| `cmd/scaleway-manager/main_test.go`, `serve_failure_linux_test.go`, `embed_test.go`, `internal/server/server_test.go` | `make test-go` |
| `tests/make/make_test.go`, `tests/make/wiring_test.go` | `make test-go` |
| `tests/image/build-image-*.test.sh` | CI job `build-image` |
| `tests/workflows/*.test.js` | `make test-workflows` |

## Orphans

None: every frozen file has a runner.

## Expected failures per trunk merge

Contract stubs (`internal/kube/{token,resource,client}.go`, `internal/kube/kubetest/server.go`, `cmd/fake-operator/*.go`) are committed with the frozen tests. Every package compiles, `make lint-go` stays green, and no file needs to be set aside. Frozen tests of unbuilt stories fail at run time under `make test-go`.

Six frozen tests already pass against the stubs. Five are invariants or guards that must stay green: `TestDirectImportsStandardLibraryOnly`, `TestPackageHasSourceFiles`, `TestFakeOperatorAbsentFromReleaseArtifacts`, `TestFakeOperatorScanReportsLeak` and `TestCausesAnnotationKeys`. The sixth, `TestDeleteWithoutFinalizers`, was strengthened by the author's decision and re-frozen.

| After the merge of | Expected `make -k check` failures |
| --- | --- |
| S1 | `test-go`: the frozen tests of S2 to S7 (in `internal/kube/kubetest`, `cmd/fake-operator`, `tests/make/fake_operator_test.go`) |
| S2 | as after S1, without S2 |
| S3 | `test-go`: the frozen tests of S4 to S7 |
| Each leaf | the frozen tests of the leaves not merged yet |
| Last leaf | none: `make check` fully green |
