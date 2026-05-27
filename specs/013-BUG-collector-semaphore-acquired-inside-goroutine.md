---
spec: 013
title: Collector spawns all goroutines up front; semaphore only throttles the HTTP call
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# BUG: Collector spawns all goroutines up front; semaphore only throttles the HTTP call

## Problem

`internal/collector/bgtasks.go`:

```go
for p := 2; p <= totalPages; p++ {
    wg.Add(1)
    go func(page int) {
        defer wg.Done()
        sem <- struct{}{}                  // acquire AFTER goroutine starts
        defer func() { <-sem }()
        r, err := fetchPage(...)
        ...
    }(p)
}
```

The semaphore is acquired *inside* the goroutine, so for a dataset of, say,
1,000 pages, the loop creates 999 goroutines immediately. Only the HTTP
call is gated by `parallel`. Each waiting goroutine sits with its own stack
(~2 KB) plus the deferred cleanup; for a small collector this is harmless
but it defeats the apparent intent of "fetch at most N pages at once" — the
guarantee is "fetch at most N pages, but pay the goroutine cost of all of
them up front".

A second, separate concern: there is no upstream signal from a failed
goroutine to skip remaining goroutines that haven't yet acquired the
semaphore (covered by 012-BUG-collector-error-non-deterministic).

## Why it matters

- The cost is small for current data sizes (~180 pages), but the pattern
  scales linearly with `Total`. A large SonarQube instance with hundreds of
  thousands of background tasks would spawn thousands of stalled goroutines.
- The current pattern obscures intent — anyone reading it would assume the
  semaphore controls how many goroutines exist, not just how many are doing
  HTTP work at any moment.

## Proposed fix

Acquire the semaphore *before* spawning the goroutine. Or simpler, use a
fixed worker pool consuming page numbers from a channel:

```go
pages := make(chan int)
go func() {
    defer close(pages)
    for p := 2; p <= totalPages; p++ {
        pages <- p
    }
}()

var wg sync.WaitGroup
for i := 0; i < parallel; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for page := range pages {
            ...
        }
    }()
}
wg.Wait()
```

This is the standard Go worker-pool idiom: exactly `parallel` goroutines,
each pulling work. Workers call `writePage` (or `fetchPage` + write) directly
inside the loop — there is no separate results channel. This is consistent
with the `errgroup` approach in [[012-BUG-collector-error-non-deterministic]],
which already adopts this worker-pool pattern.

## Validation

- This bug is resolved as a side-effect of implementing
  [[012-BUG-collector-error-non-deterministic]]. If 012 is implemented
  first using the worker-pool + `errgroup` pattern, no additional change
  is required here.
- If implementing this spec independently, use an `atomic.Int64` counter
  that increments when a goroutine starts and decrements when it finishes.
  Assert the peak value never exceeds `parallel`. Do not use
  `runtime.NumGoroutine()` — it is unreliable in tests.

## Prerequisites

Implement after or together with [[012-BUG-collector-error-non-deterministic]].
The 012 fix already resolves this issue when using the worker-pool approach.
