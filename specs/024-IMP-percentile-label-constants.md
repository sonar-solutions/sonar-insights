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

Use a typed enum-like key alongside the label:

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

Then `findPercentileAnalyses` becomes:

```go
for i := range analyses {
    switch analyses[i].Slice {
    case SliceAllBuckets: allBuckets = &analyses[i]
    case SliceWeekday:    weekday = &analyses[i]
    }
}
```

A switch on a known enum is exhaustive in style — adding a new slice
forces a compiler-helped audit of every consumer.

## Validation

- Existing tests in `report_test.go` continue to pass.
- New test that changes `Label` to something else and asserts
  `findPercentileAnalyses` still finds the slice via `Slice`.

## Prerequisites

None.
