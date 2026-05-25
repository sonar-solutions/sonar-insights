---
spec: 010
title: `AnalyzeDateRange.LatestCompletion` falls back to `SubmittedAt`, deviating from spec
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# BUG: `AnalyzeDateRange.LatestCompletion` falls back to `SubmittedAt`, deviating from spec

## Problem

Spec 002 line 147–148:

> ### 1. Date range
> - Earliest `submittedAt` across all tasks
> - **Latest `executedAt` across all tasks**

In `internal/analyzer/bgtasks/metrics.go`, the implementation diverges:

```go
func executedOrSubmitted(t BgTask) time.Time {
	if t.ExecutedAt.IsZero() {
		return t.SubmittedAt
	}
	return t.ExecutedAt
}
```

`AnalyzeDateRange` uses this helper, so a task without an `executedAt` (for
example a FAILED task that never started, or a task in flight at collection
time) contributes its `submittedAt` to `LatestCompletion`. The downstream
effects:

- `chartRange` in `metrics.go` uses `LatestCompletion` as the zero-fill end
  for the per-day chart. A queued-but-never-executed task on day N+5
  artificially extends the chart with five days of empty bars.
- `CapacityDemand` uses `dr.LatestCompletion` as its window end. The window
  inflates beyond the actual execution timeline, creating empty buckets and
  diluting the percentile calculations.

There is a test confirming the deviation
(`TestAnalyzeDateRange_ZeroExecutedAt`) — but the test asserts the *current*
behaviour, not the spec. It memorialises the bug.

## Why it matters

- Direct spec violation: the contract says "Latest `executedAt`", not "max of
  executedAt or submittedAt".
- Numerically wrong: percentiles, recommendations, and chart widths all
  inflate when there are queued / failed tasks at the tail of the dataset.
- The test that codifies the behaviour will block the fix unless it is
  updated at the same time.

## Proposed fix

Decide intentionally between two readings of the spec:

**Option A — strictly follow the spec.** `LatestCompletion` is the max
non-zero `executedAt`. If no task has executed yet, return zero.

```go
func AnalyzeDateRange(tasks []BgTask) DateRange {
    ...
    var latest time.Time
    for _, t := range tasks {
        if !t.ExecutedAt.IsZero() && t.ExecutedAt.After(latest) {
            latest = t.ExecutedAt
        }
        ...
    }
    ...
}
```

**Option B — update the spec.** If the intended behaviour really is "max of
either timestamp" (e.g. because the C# reference does this too), amend spec
002 to say so explicitly. Then this isn't a bug, just a documentation gap.

Either way, the test must reflect the spec, not be authoritative on its own.

## Validation

- Decide A or B (probably A — the spec is recent and explicit).
- Update `TestAnalyzeDateRange_ZeroExecutedAt` to match.
- Add a test covering a mixed dataset: one task with `ExecutedAt = day 5`,
  one task with `SubmittedAt = day 10, ExecutedAt = zero` → `LatestCompletion
  = day 5`, not day 10.

## Prerequisites

None.
