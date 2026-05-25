---
spec: 006
title: `maxExecutedAt` is formatted using local timezone, breaking determinism
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
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

Compute the cutoff in UTC and use an explicit UTC format:

```go
maxExecutedAt := time.Now().UTC().Add(-5 * time.Minute).Format("2006-01-02T15:04:05+0000")
```

Or, since SonarQube accepts RFC 3339, use `time.RFC3339` — but verify
SonarQube's actual parser tolerates the `Z` suffix first.

## Validation

Unit test that calls a small helper (`buildMaxExecutedAt(now time.Time) string`)
and asserts the returned string ends with `+0000` regardless of the input's
location.

## Prerequisites

None.
