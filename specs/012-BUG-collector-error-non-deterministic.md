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

Use `errgroup.Group` with `errgroup.WithContext`:

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(parallel)
for p := 2; p <= totalPages; p++ {
    page := p
    g.Go(func() error {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        r, err := fetchPage(ctx, instance, maxExecutedAtEncoded, page)
        if err != nil {
            return fmt.Errorf("page %d: %w", page, err)
        }
        if err := writePage(r.body, targetDir, page); err != nil {
            return fmt.Errorf("page %d: %w", page, err)
        }
        return nil
    })
}
return g.Wait()
```

`errgroup.Group.Wait` returns the first error in the order it was *reported*
(not in the order goroutines were started, but it cancels the shared
`Context` on first error so the rest stop quickly). For both the collector
and the loader this is the right semantics: fail fast, report deterministic
context.

`errgroup` is in the Go x/sync repo (`golang.org/x/sync/errgroup`). It is
not yet an approved external dependency per CLAUDE.md; if approval is not
desired, build the same fail-fast pattern with stdlib (`context` +
`sync.Once` to capture the first error).

## Validation

- A new test that injects errors on two different pages and asserts the
  returned error mentions a deterministic page (e.g. the lowest-numbered
  one). With `errgroup`, you have to choose what "first" means — picking
  "lowest page index" gives a stable contract.
- A test that asserts the collector stops issuing requests after the first
  failure (count requests; expect ≤ N).

## Prerequisites

If choosing `errgroup`, ask the user for approval to add
`golang.org/x/sync` per CLAUDE.md's external-dependency rule.
