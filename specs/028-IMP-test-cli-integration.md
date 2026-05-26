---
spec: 028
title: Add CLI-level integration tests that exercise Cobra wiring end-to-end
author: code-review
date: 2026-05-25
draft-status: draft
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

Cobra is designed to be test-driven. Use `rootCmd.SetArgs([]string{...})` +
`rootCmd.SetOut(io.Discard)` + `rootCmd.Execute()` from a test harness, and
substitute the action functions with test doubles (or use a fake collector
/ fake analyzer through dependency injection).

Sketch:

```go
func TestAnalyze_DefaultsToAllTargets(t *testing.T) {
    invoked := map[string]int{}
    withFakeAnalyzers(t, map[string]func(...) error{
        "bgtasks": func(...) error { invoked["bgtasks"]++; return nil },
        "metrics": func(...) error { invoked["metrics"]++; return nil },
    })

    rootCmd.SetArgs([]string{"analyze"})
    if err := rootCmd.Execute(); err != nil {
        t.Fatalf("execute: %v", err)
    }
    if invoked["bgtasks"] != 1 || invoked["metrics"] != 1 {
        t.Errorf("expected both targets invoked once, got %v", invoked)
    }
}

func TestAnalyzeBgtasks_PassesDateFilters(t *testing.T) {
    var gotFrom, gotTo string
    withFakeAnalyzer(t, "bgtasks", func(opts Options) error {
        gotFrom, gotTo = opts.From.String(), opts.To.String()
        return nil
    })

    rootCmd.SetArgs([]string{"analyze", "bgtasks",
        "--from", "2026-01-01", "--to", "2026-03-31"})
    if err := rootCmd.Execute(); err != nil { ... }
    // assert gotFrom / gotTo
}
```

This depends on having a clean dependency-injection seam, which is why
[[014-IMP-target-registry]] / [[019-IMP-sonarclient-abstraction]] are
prerequisites in spirit (you need a way to substitute the real handler).

## Validation

- Adding the failing test for [[003-BUG-analyze-parent-hardcodes-target]]
  is a useful first case — write the test, watch it fail, then ship the
  bug fix.

## Prerequisites

Easier after [[014-IMP-target-registry]] + [[017-IMP-cmd-package-globals]]
because both make the cmd package testable. But the first test for
[[003-BUG-...]] is achievable today with a small refactor that makes the
analyzer dependency injectable.
