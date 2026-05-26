---
spec: 022
title: Rename variable `cap` so it doesn't shadow the `cap` builtin
author: code-review
date: 2026-05-25
draft-status: draft
impl-status: not-started
prerequisites: []
---

# IMP: Rename variable `cap` so it doesn't shadow the `cap` builtin

## Problem

`internal/analyzer/bgtasks.go`:

```go
cap := bgtasks.CalculateCapacityDemand(nonIssueSync, dr.EarliestSubmission, dr.LatestCompletion)
...
CapacityDemand: cap,
```

`internal/analyzer/bgtasks/report.go`:

```go
func buildCapacitySection(cap CapacityDemandResults) *rptgen.Section { ... }
func buildCapacityHTML(cap CapacityDemandResults) string { ... }
func computeRecommendation(cap CapacityDemandResults) *capacityRecommendation { ... }
... (many more)
```

`cap` is a Go builtin (`cap(slice)`, `cap(channel)`). Shadowing it inside
a function silently disables it for the rest of that scope — if anyone
later writes `cap(someSlice)`, it'll get a type error referring to
`CapacityDemandResults`.

Go style guides (Google, Uber) explicitly call out shadowing builtins as
an anti-pattern. `gopls` flags it; `golangci-lint`'s `predeclared` checker
catches it.

## Why it matters

- Pure readability — `cd`, `cdr`, `capacity` are all better names.
- Lints today (silently — not run in CI as a hard gate). When the lint
  is enforced, every PR touching these files will trip.
- Subtle bug surface — any future code that wants to compute slice cap
  inside one of these functions will fail to compile in a way that's
  confusing to a reader.

## Proposed fix

Rename to `cd` (matches existing `dr` for `DateRange`) or to `capacity` if
verbosity is preferred:

```go
cd := bgtasks.CalculateCapacityDemand(...)
...
CapacityDemand: cd,
```

```go
func buildCapacitySection(cd CapacityDemandResults) *rptgen.Section { ... }
```

`replace_all` on each file does it mechanically.

## Validation

- `go build ./...` and `go test ./...` continue to pass.
- Add `predeclared` to `.golangci.yml` so this regression can't slip back in.

## Prerequisites

None.
