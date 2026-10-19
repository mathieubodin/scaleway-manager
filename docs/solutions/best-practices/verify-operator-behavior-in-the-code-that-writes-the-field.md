---
title: Verify scaleway-operator behavior in the code that writes the field, not in its types
date: 2026-10-19
category: best-practices
module: "architecture spine (Setup 34), scaleway-operator facts"
problem_type: best_practice
component: documentation
severity: medium
applies_when:
  - "Stating what scaleway-operator does with a CR field (status values, initial state, immutability, adoption)"
  - "Writing a decision premise for the author, a spine rule, or fake-operator behavior from operator code"
tags: [scaleway-operator, status, fake-operator, architecture-spine, verification]
---

# Verify scaleway-operator behavior in the code that writes the field, not in its types

## Context

During Setup #34, two operator facts reached the architecture spine from the CR type definitions in `scaleway-operator/src/resources.rs`:

- "The operator marks a NamespaceRole `Validated` or `Invalid`." The field `NamespaceRoleStatus.validation_state` exists and its Rust `Default` is `"Validated"`. Yet no reconciler ever writes it: the operator only reads the NamespaceRole (`src/context.rs`, `Api::all(...).get(namespace)`) to learn `spec.scaleway_role`.
- "LoadBalancer starts at `Pending`." The reconciler writes `Syncing` after creation (`src/reconcilers.rs`). It writes `Pending` only after Scaleway reports the load balancer not found.

The first claim was put to the author as the premise of a decision ("editable only when Validated"). The author chose on that basis. Every NamespaceRole would then have been uneditable against the real operator, and the fake operator would have written a status the real one never writes. The pre-merge correctness review caught it, and the question was asked again.

## Guidance

Before stating an operator behavior, find the line that writes the value at runtime, not the line that declares it:

```bash
# Who writes the field? A declaration or Default impl is not evidence.
grep -n 'validation_state\s*=' src/*.rs        # assignments only
grep -n 'sync_state = "' src/reconcilers.rs    # every state actually written
grep -n 'NamespaceRole' src/*.rs | grep -v '^src/resources.rs'   # usage outside the type file
```

- A `Default` impl, a `#[serde(default)]` or a doc comment ("Validated, Invalid, etc.") describes a shape, not a behavior.
- A status the operator writes appears as an assignment in a reconciler, followed by `update_status`/`patch_status`.
- When a fact becomes a question premise for the author, cite the file and line that write it. If none exists, say so in the question.
- The fake operator (spine AD-9) mimics only what a reconciler writes. A field no reconciler writes stays untouched in the fake too.

## Why This Matters

A wrong premise turns into a wrong product rule, accepted in good faith, and into a fake operator that lets tests pass against behavior production never has. That is exactly the drift AD-9 exists to prevent. The first spine reviewers, reading the type file, took `validation_state` as live status. Only the pre-merge correctness reviewer, who read the reconcilers, caught it.

## When to Apply

- Writing or updating spine rules on CR status, initial states, immutable fields or adoption keys.
- Building or changing `cmd/fake-operator` and its Gherkin steps.
- Any question to the author whose premise is "the operator does X".

## Examples

| Claim | From the type (wrong source) | From the writer (right source) |
| --- | --- | --- |
| NamespaceRole status | `validation_state`, `Default` = `Validated` | never written; only `spec.scaleway_role` is read |
| LoadBalancer initial state | — (assumed `Pending`) | `sync_state = "Syncing"` after create; `"Pending"` only after `LbNotFound` |
| Instance zone change | `spec.zone: String`, no immutability marker | get by id in the new zone → NotFound → recreate; the old instance is orphaned |
