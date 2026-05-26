---
spec: 008
title: `mathutil.toFloat64` silently returns 0 for non-numeric ordered types
author: code-review
date: 2026-05-25
draft-status: draft
impl-status: not-started
prerequisites: []
---

# BUG: `mathutil.toFloat64` silently returns 0 for non-numeric ordered types

## Problem

`CalculatePercentile` is constrained on `cmp.Ordered`, which includes
`string` and all numeric types. `toFloat64` only handles numerics:

```go
func toFloat64[T cmp.Ordered](v T) float64 {
	switch x := any(v).(type) {
	case int:    return float64(x)
	... // all numeric types
	default:
		return 0 // <-- silent fallback
	}
}
```

If a caller (today, or anyone using this package later because it lives in
`internal/mathutil` and is presented as reusable) passes
`CalculatePercentile([]string{...}, 0.5)`, every value is converted to `0`
and the function returns `0` for every percentile. Nothing in the call site
or logs reveals the misuse.

Secondary issue: each call goes through `any()`-boxing and a 12-case type
switch on every element — wasteful when the constraint could be narrowed.

## Why it matters

- The function lives in a "shared utility" package with a permissive generic
  constraint. The first time someone reaches for it with the wrong type, they
  get plausible-looking but completely wrong numbers.
- Silent-zero failures are the worst kind: the dashboard rendered looks fine,
  the percentile column says "0", and nobody notices until a customer reports
  it.

## Proposed fix

Narrow the type constraint to numeric types only. Define a small constraint:

```go
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

func CalculatePercentile[T Number](values []T, p float64) float64 { ... }
```

With this constraint the compiler rejects non-numeric callers at the call
site, and `toFloat64` can be replaced with `float64(v)`. The 12-case type
switch and `any`-boxing both disappear.

## Validation

- Add a compile-time test in `mathutil` that uses `string` and verify the
  file no longer compiles (negative-compile checks aren't standard in Go;
  document the expected error in a comment or use `analysistest`).
- Existing percentile tests continue to pass with the new constraint.

## Prerequisites

None.
