---
spec: 022
title: Rename variable `cap` so it doesn't shadow the `cap` builtin
author: code-review
date: 2026-05-25
draft-status: ready
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

Rename to `cd` (consistent with the existing `dr` abbreviation for `DateRange`).
Apply `replace_all` to the following files:

- `internal/analyzer/bgtasks.go` — local variable `cap`
- `internal/analyzer/bgtasks/report.go` — parameter name in four functions:
  `buildCapacitySection`, `buildCapacityHTML`, `computeRecommendation`,
  `writeDetailedHTML`
- `internal/analyzer/bgtasks/report_test.go` — any local variables named `cap`
- `internal/analyzer/bgtasks/capacity_test.go` — any local variables named `cap`

```go
cd := bgtasks.CalculateCapacityDemand(...)
...
CapacityDemand: cd,
```

```go
func buildCapacitySection(cd CapacityDemandResults) *rptgen.Section { ... }
```

## Validation

- `go build ./...` and `go test ./...` continue to pass.
- Create `.golangci.yml` if it does not exist, and enable the `predeclared`
  linter so this regression cannot slip back in:

```yaml
linters:
  enable:
    - predeclared
```

Then confirm `golangci-lint run` passes.

## Prerequisites

None.
