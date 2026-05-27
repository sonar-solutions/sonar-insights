---
spec: 024
title: Replace substring label matching in capacity analysis with named constants
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Replace substring label matching in capacity analysis with named constants

## Problem

`internal/analyzer/bgtasks/report.go`:

```go
func findPercentileAnalyses(analyses []PercentileAnalysis) (*PercentileAnalysis, *PercentileAnalysis) {
    var allBuckets, weekday *PercentileAnalysis
    for i := range analyses {
        a := &analyses[i]
        if strings.Contains(a.Label, "All Buckets") {
            allBuckets = a
        } else if strings.Contains(a.Label, "Weekday") {
            weekday = a
        }
    }
    return allBuckets, weekday
}
```

The `Label` field is set in `capacity.go`:

```go
calcPercentiles(&results, func(k BucketKey) bool { return true }, "All Buckets (24/7 Coverage)")
calcPercentiles(&results, func(...) bool { ... }, "Weekday Buckets Only (Monday-Friday)")
```

The label is doing two jobs:

1. Human-readable string in the rendered HTML.
2. Machine identifier the report-builder uses to find the right analysis.

Today it works because the strings happen to contain `"All Buckets"` and
`"Weekday"`. If anyone polishes the label ("Weekday-only Buckets",
"All Buckets, 24/7") the report builder silently picks neither and the
recommendation degrades to "no data available".

## Why it matters

- The coupling between two files is invisible: a label change in
  `capacity.go` breaks downstream behaviour with no compile-time signal.
- This is an example of a more general principle: never overload a
  display string with a routing key. The fix is small and the prevention
  value is real.

## Proposed fix

Use a typed enum-like key alongside the label. `PercentileSlice` and its
constants are exported (uppercase) since `PercentileAnalysis` is exported:

```go
// capacity.go

type PercentileSlice int

const (
    SliceAllBuckets PercentileSlice = iota
    SliceWeekday
)

type PercentileAnalysis struct {
    Slice       PercentileSlice
    Label       string  // human-readable; free to change without affecting routing
    BucketCount int
    Percentiles map[float64]PercentileResult
}
```

Update `calcPercentiles` to accept the slice identifier so it can set the
field when building the result:

```go
// Before:
func calcPercentiles(results *CapacityDemandResults, filter func(BucketKey) bool, label string)

// After:
func calcPercentiles(results *CapacityDemandResults, slice PercentileSlice, filter func(BucketKey) bool, label string)
```

Update both call sites in `CalculateCapacityDemand`:

```go
calcPercentiles(&results, SliceAllBuckets, func(k BucketKey) bool { return true }, "All Buckets (24/7 Coverage)")
calcPercentiles(&results, SliceWeekday, func(k BucketKey) bool { ... }, "Weekday Buckets Only (Monday-Friday)")
```

Inside `calcPercentiles`, set the Slice field when appending:

```go
results.PercentileAnalyses = append(results.PercentileAnalyses, PercentileAnalysis{
    Slice:       slice,
    Label:       label,
    BucketCount: len(demands),
    Percentiles: percentiles,
})
```

Update the `makeAnalysis()` test helper in `report_test.go` to accept a
`PercentileSlice` parameter so existing test calls compile:

```go
func makeAnalysis(slice PercentileSlice, label string, ...) PercentileAnalysis {
    return PercentileAnalysis{Slice: slice, Label: label, ...}
}
```

Then `findPercentileAnalyses` becomes:

```go
for i := range analyses {
    switch analyses[i].Slice {
    case SliceAllBuckets: allBuckets = &analyses[i]
    case SliceWeekday:    weekday = &analyses[i]
    }
}
```

## Validation

- Existing tests in `report_test.go` continue to pass.
- New test that changes `Label` to something else and asserts
  `findPercentileAnalyses` still finds the slice via `Slice`.

## Prerequisites

None.
