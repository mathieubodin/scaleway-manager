---
title: 'Helm chart'
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

**Problem:** Nothing installs the server in a cluster; #16 and #18 need a chart that runs exactly one pod without API rights, reachable through a Gateway the installer names.

**Approach:** The chart `charts/scaleway-manager` exactly as `skeleton-contract.md`, "Chart" (values, objects, render failures), proved by helm-unittest tests and `helm lint` in `make lint-chart` (CAP-4, chart part of CAP-5).

## Boundaries & Constraints

**Always:**

- Every object, field and value of the contract's "Chart" section; `Chart.yaml`: `apiVersion: v2`, `name: scaleway-manager`, `type: application`, `version: 0.0.0`, `appVersion: "0.0.0"`.
- Render failures through `required` (Gateway keys: absent, `null` and `""` all fail) and `fail` (`listenAddr`), each message starting with the key path without `.Values`.
- Port of `listenAddr`: the text after its last `:`, matched by `^[1-9][0-9]*$` and at most 65535.
- Makefile, following story 1's pattern: `HELM_VERSION := 4.3.0`, `HELM_UNITTEST_VERSION := 1.2.1`; guard `check-helm` fails with `Error: helm 4.3.0 required. Install with: https://helm.sh/docs/intro/install/` when `helm` is missing or `helm version --template '{{.Version}}'` is not exactly `v$(HELM_VERSION)`, and with a hint naming helm-unittest `$(HELM_UNITTEST_VERSION)` and its install command when `helm plugin list` shows no `unittest` plugin at exactly that version; `print-helm-version` (no `##`) prints `$(HELM_VERSION)`; `lint-chart` joins `make check`.
- CI job `lint-chart`: checkout as story 1; `azure/setup-helm@9bc31f4ebc9c6b171d7bfbaa5d006ae7abdb4310 # v5.0.1` with `version: v` + the output of `make -s print-helm-version` (from a previous step); the plugin installed with the same command as the hint; `make lint-chart`.
- prek: builtin `check-yaml` gets `exclude = "^charts/[^/]+/templates/"`; local hook `lint-chart` with `name = "Chart lint"` and `files = "^charts/"`, like story 1's `lint-go`.

**Never:** a Role, RoleBinding or ClusterRoleBinding; a rule in either ClusterRole; a Gateway, CRD or `/api/events` rule; a CPU limit; a probe timing field; a port check rejecting ports below 1024; `values.schema.json`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Defaults + Gateway | `ci/test-values.yaml` (Gateway `name` and `namespace` set), release `scaleway-manager` | 6 objects named per contract; container port `http` = 8080; probes and Service `targetPort` = `http` | N/A |
| Optional set | `sectionName: https`, `hostnames: [a.example]` | `parentRefs[0].sectionName` and `hostnames` rendered | N/A |
| Optional unset | both empty | neither field present (`notExists`) | N/A |
| IPv6 | `listenAddr: "[::]:8080"` | port 8080; env `LISTEN_ADDR` `[::]:8080` | N/A |
| Tag | `image.tag: x` | image `ghcr.io/mathieubodin/scaleway-manager:x` | N/A |
| No tag | `image.tag: ""` | image tag `0.0.0` | N/A |
| Gateway missing | `name` or `namespace` absent, `null`, `""` | render fails | message starts `httpRoute.gateway.name` / `httpRoute.gateway.namespace` |
| Bad port | `8080`, `host:`, `:08080`, `:+80`, `: 8080`, `:0`, `:65536`, `:http` | render fails | message starts `listenAddr` |

</frozen-after-approval>

## Code Map

- `skeleton-contract.md`, "Chart" -- source of every asserted value (values block, Objects table, resources).
- `Makefile`, `prek.toml`, `.github/workflows/pr-checks.yml` -- story 1 and 2 patterns; existing `check-yaml` hook entry in `prek.toml`.
- `.markdownlint-cli2.yaml` -- lints the chart `README.md`; no change.
- `biome.json` -- covers `**/*.json`; the chart adds no JSON.

## Tasks & Acceptance

**Execution:**

- [ ] `charts/scaleway-manager/Chart.yaml`, `values.yaml` -- per Always and the contract values block, with comments: `listenAddr` port ≥ 1024, `image.tag` needs an override until #18 publishes, no CPU limit.
- [ ] `charts/scaleway-manager/templates/_helpers.tpl` -- `fullname` and `name` as `helm create`; labels `helm.sh/chart`, `app.kubernetes.io/name`, `app.kubernetes.io/instance`, `app.kubernetes.io/version`, `app.kubernetes.io/managed-by`, selector labels `name` and `instance`; the `listenAddr` port helper with its `fail`.
- [ ] `charts/scaleway-manager/templates/{serviceaccount,deployment,service,httproute,clusterroles}.yaml` -- one file per object kind of the Objects table; container name `scaleway-manager`; common labels on all six objects, selector labels `app.kubernetes.io/name` and `app.kubernetes.io/instance` on the Deployment selector, its pod template and the Service selector; Service `type: ClusterIP`, port `name: http`, `protocol: TCP`; `parentRefs` carry only `name`, `namespace` and `sectionName` when set; `backendRefs` only `name` and `port: 80`.
- [ ] `charts/scaleway-manager/.helmignore` -- excludes `tests/` and `ci/`.
- [ ] `charts/scaleway-manager/ci/test-values.yaml` -- Gateway `name` and `namespace` only.
- [ ] `charts/scaleway-manager/tests/*_test.yaml` -- one suite per template, `release.name: scaleway-manager`, covering every matrix row and every Objects table row at its exact value (`equal`, `notExists`, `failedTemplate` with `errorPattern`).
- [ ] `charts/scaleway-manager/README.md` -- install command with the Gateway values and `image.tag`; one install per cluster (fixed ClusterRole names); no role is bound.
- [ ] `Makefile` -- `check-helm`, `print-helm-version`, `lint-chart: check-helm ##` running `helm lint charts/scaleway-manager --values charts/scaleway-manager/ci/test-values.yaml` then `helm unittest charts/scaleway-manager`; `check` extended.
- [ ] `prek.toml`, `.github/workflows/pr-checks.yml` -- per Always.

**Acceptance Criteria:**

- Given this story merged, when `make lint-chart` and `make check` run locally and in CI, then they exit 0.
- Given a Helm other than v4.3.0 first in `PATH`, or the plugin removed, when `make lint-chart` runs, then it exits 1 with the matching hint (real runs, then restored).
- Given `helm package charts/scaleway-manager`, then the archive holds no `tests/` or `ci/` file.
- Given a commit touching only `charts/`, then prek runs `lint-chart` and `check-yaml` skips the templates.

## Implementation Notes

- E1 install method: neither OCI nor git. The OCI install failed on the author's machine with a 403 caused by local ghcr.io credentials; the author installed from the release archive, verified with a keyring: `helm plugin install https://github.com/helm-unittest/helm-unittest/releases/download/v1.2.1/unittest-1.2.1.tgz --keyring .github/keys/helm-unittest.asc`. The keyring is the project key from `https://raw.githubusercontent.com/helm-unittest/helm-unittest/main/public-key.asc` (fingerprint `853008CD0B0A5A2DA04FF8A5616AD47F65B54AC7`, expires 2028-05-06), committed armored: Helm 4.3.0 reads it as is. Real runs: the install succeeds with that key and fails with another key (`openpgp: signature made by unknown entity`). The hint and the CI job use this command; CI writes the version literally, and `tests/workflows/lint-chart.test.js` fails if it drifts from the hint.

## Spec Change Log

## Review Triage Log

## Design Notes

E1 of `plan.md`: try the verified OCI install first (`helm plugin install oci://ghcr.io/helm-unittest/helm-unittest/unittest:$(HELM_UNITTEST_VERSION)`); if verification cannot succeed on a real run, use the git URL with `--version v$(HELM_UNITTEST_VERSION) --verify=false`; record which one and why in Implementation Notes. The hint and CI use the same command.

## Verification

**Commands:**

- `make lint-chart` -- expected: exit 0, all helm-unittest suites pass.
- `helm template scaleway-manager charts/scaleway-manager` -- expected: exit non-zero, message starting `httpRoute.gateway.name`.
- `make check` -- expected: exit 0.
