---
spec: 028
title: Add CLI-level integration tests that exercise Cobra wiring end-to-end
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Add CLI-level integration tests that exercise Cobra wiring end-to-end

## Problem

The `cmd/` package today is tested only at the unit-helper level
(`parseOptionalDate`, `prepareOutputDir`, `writeCollectMetadata`). None of
the tests exercise:

- Flag parsing and binding.
- Subcommand dispatch (does `analyze bgtasks` reach the right handler?).
- Inheritance of parent flags by subcommands (does
  `run bgtasks --report-name X` actually plumb through to the analyzer?).
- Default value handling and environment-variable fallback (`SONAR_TOKEN`).
- Error formatting and exit codes (does `main` exit non-zero on error?).

This is why a bug like [[003-BUG-analyze-parent-hardcodes-target]] could
hide undetected — there is no test that says "running `analyze` with no
subcommand should produce X behaviour".

## Why it matters

- The CLI is the product. Wrong flag handling is more user-visible than
  any internal logic bug.
- The current unit tests give a false sense of coverage. Coverage numbers
  in `cmd/` look reasonable while large behaviours are unverified.
- As more subcommands are added (per [[014-IMP-target-registry]]), the
  combinatorics of flag inheritance will grow. Tests are the only way
  to keep this honest.

## Proposed approach

**DI seam — function variables.** Replace the hardcoded
`analyzer.AnalyzeBgTasks(...)` calls in `cmd/analyze.go` and `cmd/run.go`
with package-level function variables that tests can override:

```go
// cmd/analyze.go
var analyzeBgtasksFn = analyzer.AnalyzeBgTasks // production default

func runAnalyzeBgtasksCmd(...) error {
    ...
    return analyzeBgtasksFn(ctx, opts)
}
```

Tests override the variable with a fake and restore the original via
`t.Cleanup`:

```go
func withFakeAnalyzer(t *testing.T, fn func(context.Context, analyzer.Options) error) {
    t.Helper()
    orig := analyzeBgtasksFn
    analyzeBgtasksFn = fn
    t.Cleanup(func() { analyzeBgtasksFn = orig })
}
```

**Test isolation — fresh command tree per test.** The package-level
`rootCmd` is mutable (cobra stores flag state). Tests must NOT share it.
Extract a constructor:

```go
// cmd/root.go
func newRootCmd() *cobra.Command { ... } // creates a fresh tree

var rootCmd = newRootCmd() // production singleton

func Execute() error { return rootCmd.ExecuteContext(...) }
```

Tests call `newRootCmd()` directly and call `.Execute()` on the fresh
instance:

```go
func TestAnalyzeBgtasks_PassesDateFilters(t *testing.T) {
    var gotOpts analyzer.Options
    withFakeAnalyzer(t, func(_ context.Context, opts analyzer.Options) error {
        gotOpts = opts
        return nil
    })

    cmd := newRootCmd()
    cmd.SetArgs([]string{"analyze", "bgtasks", "--from", "2026-01-01", "--to", "2026-03-31"})
    if err := cmd.Execute(); err != nil {
        t.Fatalf("execute: %v", err)
    }
    if gotOpts.From == nil || gotOpts.From.Format("2006-01-02") != "2026-01-01" {
        t.Errorf("From = %v, want 2026-01-01", gotOpts.From)
    }
}
```

**Minimum viable test list for the first pass:**

1. `TestAnalyzeBgtasks_PassesDateFilters` — `--from`/`--to` reach the handler correctly.
2. `TestAnalyzeBgtasks_DefaultReportName` — default `--report-name` is `"report-bgtasks"`.
3. `TestAnalyzeCmd_RunsAllTargets` — `analyze` (no subcommand) invokes all registered targets.
4. `TestCollectBgtasks_MissingURL` — missing `--url` returns a non-zero exit code with a message.
5. `TestSonarToken_EnvFallback` — `SONAR_TOKEN` env var is read when `--token` flag is absent.

## Validation

- Adding the failing test for [[003-BUG-analyze-parent-hardcodes-target]]
  is the recommended first case — write the test, watch it fail, ship the fix.

## Prerequisites

Requires [[027-IMP-runanalyze-options-struct]] for the `analyzer.Options`
type used in the fake function signature. Easier after
[[014-IMP-target-registry]] + [[017-IMP-cmd-package-globals]], but tests 1
and 4 above are achievable with only the function-variable seam and the
`newRootCmd()` constructor.
