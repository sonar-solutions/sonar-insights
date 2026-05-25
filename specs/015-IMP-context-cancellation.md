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

1. In `cmd/`, capture `ctx := cmd.Context()` at the top of each `RunE`. In
   `cmd/root.go`, wire signal handling so that SIGINT cancels the root
   context:

   ```go
   func Execute() error {
       ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
       defer stop()
       return rootCmd.ExecuteContext(ctx)
   }
   ```

2. Add `ctx context.Context` to `CollectBgTasks`, `Load`, `AnalyzeBgTasks`,
   `Detect`, `fetchPage`. Each constructs `http.NewRequestWithContext(ctx, ...)`.

3. In parallel sections, `select { case <-ctx.Done(): return ctx.Err(); ... }`
   to fail fast.

## Concrete impact

- Provides the cancellation hook that several other findings benefit from
  ([[012-BUG-collector-error-non-deterministic]],
   [[013-BUG-collector-semaphore-acquired-inside-goroutine]]).
- Brings the codebase in line with idiomatic Go.

## Validation

- A test that starts a slow HTTP server, kicks off `CollectBgTasks` in a
  goroutine with a cancellable context, cancels mid-flight, and asserts:
  - The function returns within ~100ms.
  - The returned error is `context.Canceled` (or wraps it).

## Prerequisites

None — but doing this before [[012-BUG-collector-error-non-deterministic]]
simplifies that fix considerably.
