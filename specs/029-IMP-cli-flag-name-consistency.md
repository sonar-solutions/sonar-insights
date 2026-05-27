---
spec: 029
title: Reconcile `--out-dir` vs `--dir` and other CLI flag-name inconsistencies
author: code-review
date: 2026-05-25
draft-status: ready
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

Use `--data-dir` on all three commands:

| Command   | New flag       | Old flag    |
|-----------|----------------|-------------|
| `collect` | `--data-dir`   | `--out-dir` |
| `analyze` | `--data-dir`   | `--dir`     |
| `run`     | `--data-dir`   | `--out-dir` |

The command name already encodes the direction (`collect` writes,
`analyze` reads); the flag just needs to say "where the data lives".
Having one name across all three commands makes the pipeline obvious —
the same path value works for collect, analyze, and run.

Also update specs 001 and 002 to use `--data-dir` in their flag tables.
ho
## Validation

- The existing tests use the current names — updating them is mechanical.
- Add an `analyze --help` and `collect --help` golden test that asserts
  the renamed flag appears.

## Prerequisites

None for the code change; this needs a brief spec amendment first so the
specs and code stay aligned.
