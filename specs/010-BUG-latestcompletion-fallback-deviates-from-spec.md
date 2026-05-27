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

Implement Option A: `LatestCompletion` is the max non-zero `executedAt`.
Only the `latest` variable computation changes. The `latestSubmission`
tracking (used for `DateRangeInDays`) stays unchanged.

In `AnalyzeDateRange`, replace the `executedOrSubmitted` call:

```go
// Before:
executed := executedOrSubmitted(t)
if executed.After(latest) {
    latest = executed
}

// After:
if !t.ExecutedAt.IsZero() {
    if executed := t.ExecutedAt.UTC(); executed.After(latest) {
        latest = executed
    }
}
```

Delete `executedOrSubmitted` once it has no callers.

**Zero LatestCompletion:** If all tasks have a zero `ExecutedAt` (e.g.
the entire dataset is queued or failed tasks), `LatestCompletion` will be
the zero `time.Time`. Downstream:

- `chartRange` will produce an empty per-day map (the date loop does not
  iterate when end is zero). This is correct — there is nothing to chart.
- `CalculateCapacityDemand` will produce no buckets. The report should
  render an empty capacity section rather than panic. Verify that both
  callers handle a zero `LatestCompletion` without crashing before shipping.

## Validation

- Update `TestAnalyzeDateRange_ZeroExecutedAt`: the existing assertion
  `LatestCompletion.Equal(submitted)` (the old fallback) must change to
  `LatestCompletion.IsZero()`:
  ```go
  if !dr.LatestCompletion.IsZero() {
      t.Errorf("LatestCompletion = %v, want zero when no task has ExecutedAt", dr.LatestCompletion)
  }
  ```

- Add `TestAnalyzeDateRange_MixedDataset` — mixed executed/unexecuted tasks:
  ```go
  // one task executed on day 5, one queued task submitted on day 10
  tasks := []BgTask{
      {SubmittedAt: day(1), ExecutedAt: day(5)},
      {SubmittedAt: day(10), ExecutedAt: time.Time{}},
  }
  dr := AnalyzeDateRange(tasks)
  if !dr.LatestCompletion.Equal(day(5)) {
      t.Errorf("LatestCompletion = %v, want day 5", dr.LatestCompletion)
  }
  ```

## Prerequisites

None.
