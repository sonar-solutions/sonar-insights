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

Add a unit test that retains a reference to the original slice and asserts
its contents are unchanged after `filterByDate`:

```go
tasks := []BgTask{
    {ID: "a", SubmittedAt: ...},
    {ID: "b", SubmittedAt: ...},
    {ID: "c", SubmittedAt: ...},
}
original := slices.Clone(tasks)
_ = filterByDate(tasks, fromExcludingA, nil)
if !slices.EqualFunc(tasks, original, bgTaskEqual) {
    t.Errorf("filterByDate mutated input")
}
```

## Prerequisites

None.
