---
spec: 014
title: Introduce a target registry to make adding a new collection target a one-file change
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Introduce a target registry to make adding a new collection target a one-file change

## Problem

The string `"bgtasks"` appears throughout the CLI layer:

- `cmd/root.go` — `knownTargets() []string{"bgtasks"}`
- `cmd/collect.go` — `collectBgtasksCmd`, `runCollectBgtasksCmd`, `case "bgtasks":`
- `cmd/analyze.go` — `analyzeBgtasksCmd`, `runAnalyzeBgtasksCmd`, `case "bgtasks":`
- `cmd/run.go` — `runBgtasksCmd`, `runRunBgtasksCmd`, two `[]string{"bgtasks"}`
- `internal/collector/bgtasks.go` — the actual collector
- `internal/analyzer/bgtasks.go` — the actual analyzer

The product roadmap mentions adding more targets (and likely splitting
Server vs. Cloud variants — see [[022-IMP-sonarclient-abstraction]] and
[[007-BUG-cloud-detected-but-bgtasks-unsupported]]). Each new target
currently means editing at minimum three files in `cmd/`, plus the
collector and analyzer packages — and remembering to update every
`switch target` statement and every `[]string{...}` literal. The bug at
[[003-BUG-analyze-parent-hardcodes-target]] is exactly this kind of
omission.

## Why it matters

- The CLI is the user-facing surface; mistakes here are immediately
  visible.
- The current "add a target" path is invisible — there's no checklist or
  type that forces all the right hooks to be wired.
- All five sibling target-aware files are functionally identical with
  the strings swapped. Eliminating this duplication is the central
  prerequisite for making the codebase actually extensible.

## Proposed approach

Introduce a `Target` interface and a registry inside a new internal
package, e.g. `internal/targets`:

```go
package targets

import (
    "github.com/spf13/cobra"
    ...
)

type Target interface {
    Name() string                              // e.g. "bgtasks"
    SupportsProduct(p sonarqube.Product) bool  // for [[007-BUG-...]]
    CollectFlags(*cobra.Command)               // register subcommand-local flags
    AnalyzeFlags(*cobra.Command)
    Collect(ctx context.Context, inst sonarqube.SonarInstance, opts CollectOptions) error
    Analyze(ctx context.Context, opts AnalyzeOptions) error
    DefaultReportName() string
}

var registry = map[string]Target{}

func Register(t Target) { registry[t.Name()] = t }
func All() []Target     { ... }
func Get(name string) (Target, bool) { ... }
```

A `bgtasks` package then registers itself in `init()`:

```go
package bgtargets

func init() {
    targets.Register(&Bgtasks{})
}
```

`cmd/collect.go` becomes:

```go
for _, t := range targets.All() {
    sub := &cobra.Command{Use: t.Name(), RunE: func(...) error { ... }}
    t.CollectFlags(sub)
    collectCmd.AddCommand(sub)
}
```

The same loop in `analyze` and `run`. Adding a new target is now a single
new package with an `init()` import.

## Concrete impact

- [[003-BUG-analyze-parent-hardcodes-target]] becomes impossible.
- [[007-BUG-cloud-detected-but-bgtasks-unsupported]] gets a natural home
  via `SupportsProduct`.
- [[022-IMP-sonarclient-abstraction]] can land cleanly because the target
  interface controls which client it asks for.

## Validation

- The existing tests must continue to pass with the new wiring.
- A new test that registers a fake target and asserts it shows up under
  `collect`, `analyze`, and `run` commands.

## Prerequisites

None — this is the prerequisite for several other findings, not a
dependent of them.
