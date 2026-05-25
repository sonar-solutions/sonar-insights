---
spec: 003
title: `analyze` parent command hardcodes bgtasks instead of running all targets
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
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

## Validation

Add a CLI-level test that invokes `analyze` (no subcommand) against a temp
directory containing only one known target's data, and assert that the
target's analyzer runs (or the run is dispatched). Cleaner: introduce target
registration first (see 017-IMP-target-registry) — then this bug becomes
mechanically impossible.
