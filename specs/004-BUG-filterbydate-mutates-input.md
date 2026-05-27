---
spec: 004
title: `filterByDate` aliases the caller's backing array and mutates input
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# BUG: `filterByDate` aliases the caller's backing array and mutates input

## Problem

In `internal/analyzer/bgtasks.go`:

```go
func filterByDate(tasks []bgtasks.BgTask, from, to *time.Time) []bgtasks.BgTask {
	out := tasks[:0] // aliases the caller's backing array
	for _, t := range tasks {
		...
		out = append(out, t)
	}
	return out
}
```

`out := tasks[:0]` reuses the input slice's backing array. As elements are
appended (with `append` writing in place because there is unused capacity),
the original `tasks` slice is overwritten in place. After the call, anyone
holding the original `tasks` value sees corrupted data:

```go
tasks := []BgTask{a, b, c, d}        // a..d in backing array
filtered := filterByDate(tasks, ...) // returns [a, d], backing array now [a, d, c, d]
// tasks[0..3] is now [a, d, c, d] — silently mutated
```

The caller in `AnalyzeBgTasks` happens not to use `tasks` again after
filtering, so the bug is latent. The next refactor that reuses the slice will
introduce hard-to-trace corruption.

## Why it matters

- Hidden landmine: works today because of how the single caller is written,
  but any future use (parallel pipelines, multiple filter passes, retained
  references for logging) silently corrupts data.
- Go idiom is to return a new slice when filtering unless the in-place
  contract is explicit and documented. This function does neither.
- The bug is invisible to tests that only check the returned slice.

## Proposed fix

Allocate a new slice:

```go
func filterByDate(tasks []bgtasks.BgTask, from, to *time.Time) []bgtasks.BgTask {
	out := make([]bgtasks.BgTask, 0, len(tasks))
	for _, t := range tasks {
		submitted := t.SubmittedAt.UTC()
		if from != nil && submitted.Before(from.UTC()) {
			continue
		}
		if to != nil && submitted.After(toEndOfDay(*to)) {
			continue
		}
		out = append(out, t)
	}
	return out
}
```

If memory pressure is later a concern, the explicit in-place version can be
re-introduced under a clearly named function (`filterByDateInPlace`).

## Validation

Add a unit test `TestFilterByDate_DoesNotMutateInput` in `bgtasks_test.go`.
Use the existing `date()` and `ptr()` helpers already in that file. `BgTask`
is a plain struct with comparable fields, so `slices.Equal` works directly
without a custom comparator.

```go
func TestFilterByDate_DoesNotMutateInput(t *testing.T) {
    from := ptr(date(2026, 2, 1))
    tasks := []BgTask{
        task("a", date(2026, 1, 15), time.Time{}), // before from — filtered out
        task("b", date(2026, 2, 10), time.Time{}), // in range
        task("c", date(2026, 3, 5), time.Time{}),  // in range
    }
    original := slices.Clone(tasks)
    _ = filterByDate(tasks, from, nil)
    if !slices.Equal(tasks, original) {
        t.Errorf("filterByDate mutated the input slice: got %v, want %v", tasks, original)
    }
}
```

Add `"slices"` to the import block in `bgtasks_test.go`.

## Prerequisites

None.
