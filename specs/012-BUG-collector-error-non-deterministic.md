---
spec: 012
title: Collector and loader return non-deterministic "first error" under concurrent failures
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# BUG: Collector and loader return non-deterministic "first error" under concurrent failures

## Problem

Both `internal/collector/bgtasks.go` (`writePageResults`) and
`internal/analyzer/bgtasks/loader.go` (`loadFilesParallel`) follow the same
pattern:

```go
var firstErr error
for r := range results {
    if r.err != nil {
        if firstErr == nil {
            firstErr = fmt.Errorf("... page %d: %w", r.page, r.err)
        }
        continue
    }
    ...
}
return firstErr
```

`results` is a buffered channel populated by N goroutines that all run to
completion before `close(results)` is called. The order they push to the
channel depends on goroutine scheduling. So:

- Two pages fail with different errors → the user sees one of them, and
  which one they see changes across runs.
- All goroutines wait for `wg.Done` before any error is surfaced, even
  though we already know we're going to abort. In practice that means a
  full page sweep / file walk completes before the user gets the message.
- Goroutines do not observe the `firstErr` and so cannot short-circuit
  their work. For loader, all files are read and parsed even when the
  first one already errored.

## Why it matters

- **Debugging:** Bug reports that say "I ran it and got error X" become
  irreproducible because the next run reports error Y.
- **Latency on failure:** A user who points at the wrong directory waits
  for all files to be read before being told.
- **Operational waste:** The collector keeps hitting the API after a 401
  is observed, increasing the chance of triggering server-side rate
  limiting or audit alerts.

## Why a fix is worth it

The pattern is wrong in the same way in two places, so future copies will
follow it. Fixing once, idiomatically, eliminates a class of bug.

## Proposed fix

Use `golang.org/x/sync/errgroup` — it is an approved dependency that directly
addresses this pattern, replacing the `sync.WaitGroup` + `sync.Once` +
manual `context.WithCancel` boilerplate with a single purpose-built primitive.

**Collector (`fetchAndWriteRemainingPages`):**

```go
g, ctx := errgroup.WithContext(ctx)

pages := make(chan int)
g.Go(func() error {
    defer close(pages)
    for p := 2; p <= totalPages; p++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case pages <- p:
        }
    }
    return nil
})

for i := 0; i < parallel; i++ {
    g.Go(func() error {
        for page := range pages {
            r, err := fetchPage(ctx, instance, maxExecutedAtEncoded, page)
            if err != nil {
                return fmt.Errorf("page %d: %w", page, err)
            }
            if err := writePage(r.body, targetDir, page); err != nil {
                return fmt.Errorf("page %d: %w", page, err)
            }
        }
        return nil
    })
}
return g.Wait()
```

`errgroup.WithContext` cancels `ctx` the moment any callback returns a non-nil
error, stopping the page producer and all remaining workers. `g.Wait()` returns
the first non-nil error automatically — `sync.Once` and manual `cancel()` calls
are no longer needed.

This is a fixed worker-pool pattern (exactly `parallel` goroutines) that
also resolves [[013-BUG-collector-semaphore-acquired-inside-goroutine]].

**Loader (`loadFilesParallel`):** The loader already exits immediately on
the first error, so the goroutine spawning pattern is the main issue.
Apply the same worker-pool + `errgroup` approach for consistency:
goroutines pull file paths from a channel; the first error cancels the
context and is returned via `g.Wait()`.

## Validation

- A test that makes the test server return 500 for page 3 and asserts
  `CollectBgTasks` returns a non-nil error wrapping "page 3". The exact
  page number returned is the first one encountered by goroutine scheduling,
  so do not assert a specific page number — only assert a non-nil error
  containing "page".
- A test that counts HTTP requests after a failure and asserts the collector
  issues significantly fewer requests than `totalPages` (fail-fast working).

## Prerequisites

None.
