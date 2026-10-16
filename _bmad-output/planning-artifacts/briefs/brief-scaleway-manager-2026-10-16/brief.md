---
title: "Product Brief: scaleway-manager"
status: draft
created: 2026-10-16
updated: 2026-10-16
---

<!-- markdownlint-disable MD025 -->

# Product Brief: scaleway-manager

## Executive Summary

`scaleway-manager` is a web service that runs inside a Kubernetes cluster, typically a throwaway kind cluster started when needed and destroyed afterwards. Its backend creates, reads, updates and deletes Kubernetes custom resources. `scaleway-operator`, and later KRO, reconcile those resources into real Scaleway infrastructure.

The point is control. With the credentials it is given, the interface takes charge of what exists in a Scaleway project: it adds what is missing and deletes what it is allowed to delete, without confirmation. It can break things, and that is deliberate.

It is built in three successive stages: first to exercise `scaleway-operator`, then to test quick deployment of objects into Scaleway, then to test compositions of infrastructure objects with KRO, which could lead to a landing zone. The project is intended to be published, for any user.

## The Problem

There is no quick way to test `scaleway-operator`, then to deploy objects quickly into Scaleway, from a small kind cluster.

## The Solution

A web interface, deployed in the cluster, that:

- tells whether the cluster can talk to Scaleway and what is missing, and lets the user fill the gaps (create, edit, update) instead of only reporting them;
- lists the namespaces that manage Scaleway projects and the Scaleway custom resources in them, with their sync status;
- creates, edits (only when `Synced`) and deletes those resources;
- exports resources as YAML and re-imports them later, because the kind cluster is disposable and durable state lives outside it.

Every managed Scaleway resource is a custom resource. Existing infrastructure is taken over by writing matching resources, which the operator adopts by name or tags. The interface never shows Scaleway objects that have no resource in the cluster.

The backend is a client of the Kubernetes API and nothing else. How it works is left to the spec.

## What Makes This Different

Research found no existing tool that combines these traits for Scaleway: write access with delete, YAML export and re-import, a cluster readiness check, and filling of gaps, all aimed at a disposable cluster (see addendum for sources and what could not be verified).

Closest alternatives and why they fall short for this need:

- **kro-ui** shows KRO resources and generates instance forms from the schema, but is read-only.
- **Headlamp** is extensible and could host a plugin; it has no schema-generated forms for custom resources. Whether a Headlamp plugin would make sense is deferred.
- **Backstage with the TeraSky plugins** covers forms from CRDs, but through a heavy portal and GitOps pull requests, the opposite of a disposable kind cluster.
- **Crossplane**, with an official Scaleway provider, has its own UIs (Komoplane, Upbound). This project targets `scaleway-operator` and KRO, not Crossplane.

The edge is honest and narrow: it is tied to the operator's real workflow and to the disposable-cluster use. It is not a technical moat.

## Who This Serves

Any user who wants to manage Scaleway infrastructure through Kubernetes custom resources, starting with the author. No narrower persona has been defined.

## Success Criteria

[ASSUMPTION] Proposed from the forge session, awaiting the author's yes or no:

1. **Operator testing:** every operator state, including `Error` and `TerminationBlocked`, is visible in the interface with its message.
2. **Quick deployment:** from an empty kind cluster, a resource reaches `Synced` in Scaleway without leaving the interface.
3. **Compositions:** a KRO-composed object is created, observed and deleted from the interface.
4. **Disposable cycle:** destroy the kind cluster, start a new one, re-import the YAML, and get the same Scaleway project back under control without duplicates.

Open: no criterion yet for a stranger installing and using it, which publication implies.

## Scope

In for the first version:

- cluster readiness check and filling of gaps;
- namespaces that manage Scaleway projects, and their resources;
- create, delete, and edit-when-`Synced`;
- YAML export and re-import;
- hand-written forms, one resource type at a time, added iteratively.

Out, or deferred:

- confirmation dialogs, a "forget" action, or any safety rail on delete;
- viewing Scaleway objects that have no resource in the cluster;
- generated forms, and KRO-composed types, until stage 3;
- multiple Scaleway accounts (possible, minor);
- screen design and layout (not decided).

Rule: the four default namespaces (`default`, `kube-system`, `kube-public`, `kube-node-lease`) are never valid support for Scaleway resources. At least one other namespace is needed before the cluster can talk to Scaleway.

## Vision

The interface becomes the place from which a Scaleway project is bootstrapped and run from a small disposable cluster: start kind, re-import the saved YAML, and the project is back under control. KRO compositions could grow into a full landing zone.
