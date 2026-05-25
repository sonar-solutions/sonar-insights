---
spec: 017
title: Remove package-global state from `cmd` (logger, timing, startTime)
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Remove package-global state from `cmd` (logger, timing, startTime)

## Problem

`cmd/root.go`:

```go
var logger *slog.Logger
var timing bool
var verbose bool
var startTime time.Time
```

These are mutable package globals. They are written by Cobra's
`PersistentPreRun` hook and read from every `runXxxCmd`. The hooks run
implicitly via Cobra's lifecycle — there is no compile-time check that the
hook actually fires before the read.

Symptoms today:

- Every test in `cmd` package that exercises `runCollect` /
  `runAnalyze` (only `collect_test.go` so far) has to either accept the
  default `logger` or assign one manually before calling.
- The `runCollect` function in `cmd/collect.go` uses `logger.Debug(...)` —
  it implicitly depends on the package global.
- A future test that needs to assert log output must do reflection or
  override the global, which means tests can't run in parallel without
  contaminating each other.

## Why it matters

- Hidden-coupling: anything in `cmd` can read these. There's no map of who
  depends on what.
- Tests get harder as the package grows. Parallel tests become unsafe.
- It is the standard Go anti-pattern. Removing it now is cheap; removing
  it after 10 commands have grown around it is not.

## Proposed fix

Group execution context into a struct constructed once per command run:

```go
type runtime struct {
    logger    *slog.Logger
    startTime time.Time
    timing    bool
}

func newRuntime(verbose, timing bool) *runtime { ... }
```

`PersistentPreRun` constructs a `*runtime` and stashes it on the cobra
context (`cmd.SetContext(context.WithValue(cmd.Context(), ctxKeyRuntime, rt))`).
Each `RunE` reads it back. Internal packages (`collector`, `analyzer`)
already accept `*slog.Logger` parameters — keep that contract; `cmd` is the
only layer that needs to construct and pass it.

If [[015-IMP-context-cancellation]] is adopted, the runtime struct can
live on the context naturally alongside the cancellation context.

## Validation

- The existing tests should continue passing with no behavior change.
- A new test that constructs two `runtime` instances concurrently and runs
  `runCollect` against two different test servers in parallel — no
  cross-contamination of logger output.

## Prerequisites

Cleaner after [[015-IMP-context-cancellation]] introduces a context for the
runtime to live on. Can also be done independently.
