---
spec: 015
title: Plumb `context.Context` through CLI, collector, and HTTP calls
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Plumb `context.Context` through CLI, collector, and HTTP calls

## Problem

Nothing in the codebase takes a `context.Context`. Concretely:

- `cobra.Command.RunE` is called as `func(cmd *cobra.Command, args []string) error`,
  but `cmd.Context()` is never used.
- `collector.CollectBgTasks` takes no context.
- `fetchPage` builds requests with `http.NewRequest`, not
  `http.NewRequestWithContext`.
- `analyzer.AnalyzeBgTasks` takes no context.
- `bgtasks.Load` takes no context.

Practical consequences:

1. **Ctrl-C does not actually cancel work.** A user hitting SIGINT during a
   1,000-page collection waits for all in-flight HTTP requests to complete
   their 30-second timeout each. Cobra's signal handling is not wired.
2. **No deadlines.** A misconfigured global timeout in the future would
   require touching every call site.
3. **No fail-fast on the parallel collector.** Even if we fix the
   non-deterministic error issue ([[012-BUG-collector-error-non-deterministic]]),
   without a cancellable context the sibling goroutines can't be told to stop.

## Why it matters

- Operationally: a user trying to abort a runaway collection cannot.
- Architecturally: every other Go codebase that touches I/O is built around
  `context.Context`. Adding it later, across a much bigger surface, is
  costly. Doing it now while the surface is small is cheap.

## Proposed approach

Standard Go pattern: pass `ctx` as the first parameter on every I/O-touching
function.

1. In `cmd/root.go`, import `"os/signal"` and wire signal handling so that
   SIGINT cancels the root context:

   ```go
   import (
       "os/signal"
       "syscall"
   )

   func Execute() error {
       ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
       defer stop()
       return rootCmd.ExecuteContext(ctx)
   }
   ```

   In each `RunE`, capture: `ctx := cmd.Context()`.

2. Add `ctx context.Context` as the first parameter to:
   - `CollectBgTasks` in `internal/collector/bgtasks.go`
   - `fetchPage` in `internal/collector/bgtasks.go`
   - `AnalyzeBgTasks` in `internal/analyzer/bgtasks.go`
   - `Load` in `internal/analyzer/bgtasks/loader.go`
   - `Detect` in `internal/sonarqube/detect.go`

   Each HTTP call must use `http.NewRequestWithContext(ctx, ...)` instead of
   `http.NewRequest(...)`.

3. The two parallel sections that need `ctx.Done()` select guards are:
   - `fetchAndWriteRemainingPages` goroutines in `internal/collector/bgtasks.go`
   - `loadFilesParallel` goroutines in `internal/analyzer/bgtasks/loader.go`

   In each, check `ctx.Done()` before starting work:
   ```go
   select {
   case <-ctx.Done():
       return ctx.Err()
   default:
   }
   ```

## Concrete impact

- Provides the cancellation hook that several other findings benefit from
  ([[012-BUG-collector-error-non-deterministic]],
   [[013-BUG-collector-semaphore-acquired-inside-goroutine]]).
- Brings the codebase in line with idiomatic Go.

## Validation

- A test that starts a slow HTTP server (each response takes 500ms), kicks
  off `CollectBgTasks` in a goroutine with a cancellable context, cancels
  after 50ms, and asserts:
  - The function returns within 1 second (well within the server's response
    time, proving cancellation worked).
  - The returned error wraps `context.Canceled`.

## Prerequisites

None — but doing this before [[012-BUG-collector-error-non-deterministic]]
simplifies that fix considerably.
