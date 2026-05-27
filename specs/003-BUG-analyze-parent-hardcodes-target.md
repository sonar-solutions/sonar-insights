---
spec: 003
title: `analyze` parent command hardcodes bgtasks instead of running all targets
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: complete
prerequisites: []
---

# BUG: `analyze` parent command hardcodes bgtasks instead of running all targets

## Problem

Spec 002 states: "When invoked without a target subcommand, each command runs
all known targets using their defaults." The `collect` and `run` parents honour
this contract — both call `runCollect(knownTargets(), ...)` /
`runAnalyze(knownTargets(), ...)`. The `analyze` parent does not.

In `cmd/analyze.go`:

```go
func runAnalyzeCmd(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	reportDir, _ := cmd.Flags().GetString(flagReportDir)
	return runAnalyze([]string{"bgtasks"}, dir, reportDir, "", "")
}
```

The literal `[]string{"bgtasks"}` hardcodes the target list, so when a new
target is added (e.g. `metrics`), `sonar-insights analyze` will continue to
process only `bgtasks` and silently ignore everything else.

## Why it matters

- Violates the documented CLI contract in spec 002.
- Diverges from the sibling `collect` and `run` parent commands, creating an
  inconsistency that is easy to overlook.
- Future-hostile: adding a new target requires remembering to also patch this
  call site. The bug will manifest as "I added the target but `analyze` didn't
  pick it up."

## Proposed fix

Replace the hardcoded slice with `knownTargets()`, matching the pattern used
in `collect.go` and `run.go`:

```go
func runAnalyzeCmd(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	reportDir, _ := cmd.Flags().GetString(flagReportDir)
	return runAnalyze(knownTargets(), dir, reportDir, "", "")
}
```

## Prerequisites

None.

## Scope

The fix is intentionally minimal. `runAnalyze` internals — the hardcoded
`reportName` default and the `switch` that dispatches on target names — are
unchanged and out of scope. Those will be addressed when new targets are
introduced (see spec 014).

## Validation

No new test is required for this fix alone. `knownTargets()` currently returns
a single entry, making it impossible to distinguish "runs all targets" from
"runs the hardcoded one" in a test. Coverage of the "all targets run" property
belongs to the tests added in spec 014 when a second target exists.
