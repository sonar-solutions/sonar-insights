---
spec: 029
title: Reconcile `--out-dir` vs `--dir` and other CLI flag-name inconsistencies
author: code-review
date: 2026-05-25
draft-status: draft
impl-status: not-started
prerequisites: []
---

# IMP: Reconcile `--out-dir` vs `--dir` and other CLI flag-name inconsistencies

## Problem

The CLI uses different names for the same conceptual thing across
sibling commands:

| Command            | Flag name      | What it points at          |
|--------------------|----------------|----------------------------|
| `collect`          | `--out-dir`    | dir to write collected data |
| `analyze`          | `--dir`        | dir to read collected data  |
| `run`              | `--out-dir`    | dir to write AND read collected data |
| `run --report-dir` | `--report-dir` | where reports are written  |
| `analyze --report-dir` | `--report-dir` | where reports are written |

`analyze --dir` and `collect --out-dir` and `run --out-dir` are all the
same directory. A user invoking `run` then `analyze` separately discovers
they need to learn two names for the same thing.

Spec 002 (Flags table) introduces the inconsistency explicitly: the spec
itself uses `--dir` for analyze and `--out-dir` for collect. So this is
also a spec issue to resolve.

Other small inconsistencies:

- `collect` uses `--token` / `--url` (good) — no other command needs
  these, but `run` rightly inherits them.
- `--report-name` exists on `analyze bgtasks` but not on `analyze` or
  `run`. The intent is "only on subcommands where it makes sense", which
  is defensible but worth re-checking once a second target exists.

## Why it matters

- User-facing surface area; mistakes here are immediate cost to every
  user every day.
- The current naming hides the fact that `collect`'s output is
  `analyze`'s input — they should share a name to make the pipeline
  obvious.
- Once the names are public (people start scripting against them),
  changing them is a breaking change.

## Proposed fix

Pick one of two consistent schemes:

**Scheme A — single name `--data-dir`** (or `--dir`):

- `collect --data-dir` (writes)
- `analyze --data-dir` (reads)
- `run --data-dir`

Pros: emphasizes the shared concept. Cons: requires updating spec 001 and
breaking the current `--out-dir`.

**Scheme B — keep two names, document the relationship.**

- `collect --out-dir` and `analyze --in-dir` (clearer than `--dir`).

Either way, agree once, update the specs, and rename in code.

## Validation

- The existing tests use the current names — updating them is mechanical.
- Add an `analyze --help` and `collect --help` golden test that asserts
  the renamed flag appears.

## Prerequisites

None for the code change; this needs a brief spec amendment first so the
specs and code stay aligned.
