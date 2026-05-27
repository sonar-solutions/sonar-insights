---
spec: 006
title: `maxExecutedAt` is formatted using local timezone, breaking determinism
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: complete
prerequisites: []
---

# BUG: `maxExecutedAt` is formatted using local timezone, breaking determinism

## Problem

In `internal/collector/bgtasks.go`:

```go
maxExecutedAt := time.Now().Add(-5 * time.Minute).Format("2006-01-02T15:04:05-0700")
```

`time.Now()` returns a `time.Time` in the local timezone of the host running
the collector. The format string includes a timezone offset (`-0700`), so the
serialized value carries whatever the host happens to be set to:

- CI runner in UTC → `2026-05-25T10:00:00+0000`
- Developer laptop in CET → `2026-05-25T12:00:00+0200`
- Developer laptop in PST → `2026-05-25T03:00:00-0800`

All three represent the same instant, and SonarQube *does* accept any of
them, so the query result is correct. The problems are subtler:

1. **Spec mismatch.** The rest of the app is explicit about UTC ("All
   `submittedAt` values are normalised to UTC before comparison", spec 002).
   This is the one place the contract is violated.
2. **Non-reproducible logs.** Reading the debug log to verify the cutoff time
   becomes harder because the displayed offset varies by host.
3. **Snapshot/integration tests.** Any future test asserting on the URL or
   on a recorded HTTP cassette will fail when run in a different TZ.

## Why it matters

The cost of fixing this is one line; the cost of leaving it is recurring
confusion every time someone debugs a collection issue from a host with a
non-UTC clock. It also signals that the codebase is inconsistent about
timezone handling, which is exactly the kind of inconsistency that produces
real bugs in the next change.

## Proposed fix

Extract the cutoff into an unexported helper and compute it in UTC with a
hardcoded `+0000` offset (not `time.RFC3339`, which uses the `Z` suffix whose
acceptance by SonarQube's parser is unverified):

```go
func buildMaxExecutedAt(now time.Time) string {
    return now.UTC().Add(-5 * time.Minute).Format("2006-01-02T15:04:05+0000")
}
```

Call from `CollectBgTasks`:
```go
maxExecutedAt := buildMaxExecutedAt(time.Now())
```

`buildMaxExecutedAt` lives in `internal/collector/bgtasks.go` alongside the
rest of the collector logic.

## Validation

Add a test in `bgtasks_test.go` for `buildMaxExecutedAt`:

```go
func TestBuildMaxExecutedAt_AlwaysUTC(t *testing.T) {
    loc, _ := time.LoadLocation("America/New_York")
    now := time.Date(2026, 5, 25, 12, 0, 0, 0, loc)
    got := buildMaxExecutedAt(now)
    if !strings.HasSuffix(got, "+0000") {
        t.Errorf("expected +0000 suffix, got %q", got)
    }
}
```

## Prerequisites

None.
