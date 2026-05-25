---
spec: 027
title: Replace positional-argument soup in `runAnalyze` / `AnalyzeBgTasks` with options structs
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Replace positional-argument soup in `runAnalyze` / `AnalyzeBgTasks` with options structs

## Problem

`internal/analyzer/bgtasks.go`:

```go
func AnalyzeBgTasks(dir, reportDir, reportName string, from, to *time.Time, logger *slog.Logger) error
```

`cmd/analyze.go`:

```go
func runAnalyze(targets []string, dir, reportDir, from, to string, opts ...analyzeOption) error
```

Concerns:

1. Six positional parameters of which four are strings — type system gives
   you no protection against transposing them.
2. The function uses a half-implemented "options" pattern
   (`withReportName`) layered on top of positional args. The pattern is
   inconsistent: `from`/`to`/`dir`/`reportDir` are positional, but
   `reportName` is an option. Why one and not the others?
3. `runAnalyze` is `cmd`-package internal but is called from both
   `analyze.go` (via `runAnalyzeCmd`) and `run.go` (via `runRunCmd` /
   `runRunBgtasksCmd`). Each call site has to remember the order.

## Why it matters

- More targets mean more variations of "what does analyze need". An
  options struct accommodates per-target divergence without breaking
  the shared signature.
- Tests are harder to write because every call has to set every argument
  in the right order, even when the test only cares about one.
- The half-options/half-positional split signals that the design is in
  flux — better to commit before it grows further.

## Proposed fix

Define a per-target options struct passed from `cmd` to the analyzer:

```go
// analyzer/bgtasks.go
type Options struct {
    DataDir    string
    ReportDir  string
    ReportName string
    From       *time.Time
    To         *time.Time
    Logger     *slog.Logger
}

func AnalyzeBgTasks(ctx context.Context, opts Options) error { ... }
```

`cmd/analyze.go` builds the struct from flags. The `analyzeOption`
functional-options pattern can go away — there's no caller that wants to
partially configure analysis from outside `cmd`.

If [[014-IMP-target-registry]] lands, each target package exposes its own
`Options` type and `Analyze(ctx, opts)` method; the registry plumbs flags
into it.

## Validation

- All existing tests rebuilt against the new signature.
- A new test that constructs `Options` with only some fields set
  (zero-value `Logger`, zero-value `From`/`To`) and confirms the
  analyzer handles defaults sensibly.

## Prerequisites

Cleanest after [[014-IMP-target-registry]] but can be done independently.
