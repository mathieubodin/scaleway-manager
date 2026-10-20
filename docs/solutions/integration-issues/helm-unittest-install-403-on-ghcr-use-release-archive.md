---
title: helm-unittest install fails with 403 on ghcr.io; install the release archive with a committed key
date: 2026-10-20
category: integration-issues
module: charts/scaleway-manager tooling (make lint-chart, CI job lint-chart)
problem_type: integration_issue
component: tooling
severity: medium
symptoms:
  - "helm plugin install oci://ghcr.io/helm-unittest/helm-unittest/unittest:1.2.1 fails with 403 denied at https://ghcr.io/token"
  - "The same 403 with and without --keyring, although the 1.2.1 tag exists"
root_cause: config_error
resolution_type: environment_setup
tags: [helm, helm-unittest, helm-4, ghcr, oci, plugin-verification, gpg]
---

# helm-unittest install fails with 403 on ghcr.io; install the release archive with a committed key

## Problem

`make lint-chart` needs the helm-unittest plugin at exactly 1.2.1 on Helm 4.3.0. Installing it from its OCI artifact failed on the author's machine.

## Symptoms

- `helm plugin install oci://ghcr.io/helm-unittest/helm-unittest/unittest:1.2.1` stops at `Verifying plugin signature...` with `response status code 403: denied` on `https://ghcr.io/token?scope=repository:helm-unittest/helm-unittest/unittest:pull&service=ghcr.io`.
- Adding `--keyring` changes nothing: the request fails before any signature check.
- The tag exists: an anonymous `curl` on the same token URL returns a token, and `/v2/helm-unittest/helm-unittest/unittest/tags/list` lists `1.2.1`.

## What Didn't Work

- The OCI install with and without `--keyring`: same 403.
- The git install: the helm-unittest README says it needs `--verify=false` on Helm 4, so nothing is verified. Rejected.

## Solution

Install from the GitHub release archive and verify its provenance (`.prov`, published next to it) against the project's key, kept in the repository.

- Key: `.github/keys/helm-unittest.asc`, armored, from `https://raw.githubusercontent.com/helm-unittest/helm-unittest/main/public-key.asc`; fingerprint `853008CD0B0A5A2DA04FF8A5616AD47F65B54AC7`, expires 2028-05-06. Helm 4.3.0 reads the armored file directly.
- The Makefile builds the command from `HELM_UNITTEST_VERSION` (`HELM_UNITTEST_INSTALL`, shown in the `check-helm` hint); the `lint-chart` CI job runs the same command:

```bash
helm plugin install https://github.com/helm-unittest/helm-unittest/releases/download/v1.2.1/unittest-1.2.1.tgz \
  --keyring .github/keys/helm-unittest.asc
```

`helm plugin list` then shows `unittest 1.2.1` with PROVENANCE `signed`. With any other key the install fails with `openpgp: signature made by unknown entity`.

## Why This Works

The 403 comes from the registry token request, not from the plugin. `~/.docker/config.json` held a `ghcr.io` auth entry, and Helm's registry client reads Docker credentials; anonymous access works for the same scope. The most likely cause is that Helm sent those stale credentials and ghcr.io refused them instead of falling back to anonymous access. This was not confirmed by removing the entry (`docker logout ghcr.io`), to leave the Docker setup untouched. The release archive comes from GitHub releases, so no registry credentials are involved, and `--keyring` keeps the signature check the OCI route would have done.

## Prevention

- When ghcr.io answers 403 for a public artifact, look for a `ghcr.io` entry in `~/.docker/config.json` before suspecting the tag.
- Prefer the release archive with `--keyring` for Helm plugins in CI: no registry login, and the signature is still checked.
- The CI job writes `1.2.1` literally while the Makefile uses `HELM_UNITTEST_VERSION`: a version bump edits both; `tests/workflows/lint-chart.test.js` fails when the two commands differ.
- Renew `.github/keys/helm-unittest.asc` before 2028-05-06.

## Related

- [map-each-frozen-test-to-its-runner-after-rails.md](../workflow-issues/map-each-frozen-test-to-its-runner-after-rails.md): the other lesson of the same work.
- Issue #41, story 3 (#45).
