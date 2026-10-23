---
title: 'Packaging and isolation'
type: 'feature'
created: '2026-10-23'
status: 'ready-for-dev'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-fake-operator/go-interfaces.md'
  - '{project-root}/_bmad-output/specs/spec-fake-operator/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-fake-operator/fake-operator-contract.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The fake operator has no image and no manifests. Nothing stops it from leaking into a release artifact, and nothing checks that its RBAC stays minimal.

**Approach:** Add its Dockerfile, a build script and Make target, its JSON manifests with RBAC, and a test in `tests/make` (CAP-9). The test checks the RBAC rule by rule against `fake-operator-contract.md` ("Packaging and RBAC", "RBAC rules"), checks the Dockerfile rules, and checks that no release artifact mentions the fake.

## Boundaries & Constraints

**Always:**

- **`cmd/fake-operator/Dockerfile`.** Same two `FROM` lines and digests as the root `Dockerfile`. Build with `CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /out/fake-operator ./cmd/fake-operator`, then `USER 65532` and `ENTRYPOINT ["/fake-operator"]`. It must pass `scripts/check-dockerfile.sh cmd/fake-operator/Dockerfile`.
- **`scripts/build-fake-operator-image.sh`.** Modeled on `scripts/build-image.sh`, it runs `scripts/check-dockerfile.sh cmd/fake-operator/Dockerfile`, then `DOCKER_BUILDKIT=1 docker build -f cmd/fake-operator/Dockerfile -t scaleway-manager-fake-operator:local .` from the repository root. There is no run or probe: the binary needs an API server.
- **Makefile.** Add `build-fake-operator-image: check-docker ## Check the fake operator Dockerfile and build scaleway-manager-fake-operator:local (CAP-9)` and declare it in `.PHONY`. It stays out of `check` and out of `scripts/evidence.sh`, whose ten checks a frozen test fixes.
- **CI.** The `build-image` job of `.github/workflows/pr-checks.yml` gains one step, `run: make build-fake-operator-image`, after `make build-image`. No new job.
- **`tests/cluster/fake-operator/`.** One JSON object per file:
  - `namespace.json`: Namespace `fake-operator`.
  - `serviceaccount.json`.
  - `clusterrole.json` and `clusterrolebinding.json`.
  - `role.json` and `rolebinding.json`: Role and RoleBinding in `scaleway-system`.
  - `deployment.json`:
    - one replica, image `scaleway-manager-fake-operator:local`, `imagePullPolicy: Never`;
    - `automountServiceAccountToken: false`;
    - a projected volume (`serviceAccountToken` with `path: token` plus ConfigMap `kube-root-ca.crt`, key `ca.crt`) mounted read-only at `/var/run/secrets/kubernetes.io/serviceaccount`;
    - env `FAKE_OPERATOR_STEP_DELAY=2s`;
    - container port 8081;
    - a liveness probe `httpGet /healthz` on port 8081 with `periodSeconds: 10` and `failureThreshold: 3`, which exceeds 3 × the delay;
    - `securityContext`: `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`.
- **RBAC rules.** They are exactly the companion's "RBAC rules" table: the same `apiGroups`, `resources` and `verbs`, with no other rule, no `secrets` and no `*`.
- **`tests/make/fake_operator_test.go`** (package `make_test`; reuse `repoRoot` and `read`). Test names start with `TestFakeOperator`.
  1. Decode every JSON file of `tests/cluster/fake-operator/` with `encoding/json`. Compare the ClusterRole's and the Role's `rules` with the table, as order-insensitive sets of (`apiGroup`, `resource`, `verb`) triples. The Role's `metadata.namespace` must be `scaleway-system`.
  2. No rule names `secrets` or `*`.
  3. The Deployment has `automountServiceAccountToken: false`, the projected volume at the path above, `imagePullPolicy: Never` and the probe on `/healthz`.
  4. `scripts/check-dockerfile.sh cmd/fake-operator/Dockerfile` exits 0, run with `os/exec`.
  5. The string `fake-operator` appears neither in `Dockerfile`, nor in any file under `charts/`, nor in `.github/workflows/deliver.yml`.

**Never:**

- a YAML file or a YAML library;
- pushing the image;
- touching `Dockerfile`, `charts/`, `deliver.yml`, `scripts/evidence.sh` or any frozen file;
- adding the target to `check`;
- a Kubernetes call from the test.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| RBAC equal | manifests as specified | test passes | N/A |
| Extra verb | a scratch copy with `watch` added | the comparison helper reports the difference | test fails |
| Leak | a scratch file under a temporary `charts/` copy mentioning `fake-operator` | the scan helper reports it | test fails |
| Dockerfile | `scripts/check-dockerfile.sh cmd/fake-operator/Dockerfile` | exit 0 | N/A |
| Image | `make build-fake-operator-image` with Docker | builds `scaleway-manager-fake-operator:local` | exit 1 on failure |

</frozen-after-approval>

## Code Map

- `Dockerfile`, `scripts/check-dockerfile.sh`, `scripts/build-image.sh`, `.dockerignore` (an allowlist that already contains `cmd/` and `internal/`): the models. Do not edit the first three.
- `Makefile`: `build-image` and `check-docker` are the pattern. The frozen `tests/make/make_test.go` checks only the Go targets, so adding a target is safe.
- `.github/workflows/pr-checks.yml`: the `build-image` job, which has Docker.
- `tests/make/make_test.go` (frozen): helpers `repoRoot`, `read`; do not edit.

## Tasks & Acceptance

**Execution:**

- [ ] `cmd/fake-operator/Dockerfile`
- [ ] `scripts/build-fake-operator-image.sh` (executable)
- [ ] `Makefile`: target and `.PHONY`
- [ ] `.github/workflows/pr-checks.yml`: one step in `build-image`
- [ ] `tests/cluster/fake-operator/*.json`
- [ ] `tests/make/fake_operator_test.go`: checks 1 to 5, with the comparison and scan helpers tested on the "Extra verb" and "Leak" rows (scratch copies under `t.TempDir()`).

**Acceptance Criteria:**

- Given the story merged, when `make check` runs, then it passes.
- Given Docker, when `make build-fake-operator-image` runs, then it exits 0 and `docker image inspect scaleway-manager-fake-operator:local` succeeds.
- Given `make help`, then `build-fake-operator-image` is listed.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Verification

**Commands:**

- `go test ./tests/make/...`: expected: pass.
- `make build-fake-operator-image`: expected: exit 0.
- `make check`: expected: exit 0.
