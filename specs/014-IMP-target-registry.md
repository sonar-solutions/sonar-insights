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
Server vs. Cloud variants — see [[019-IMP-sonarclient-abstraction]] and
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

Introduce a `Target` interface and a registry inside a new package
`internal/targets`. Define shared option types there too:

```go
package targets

import (
    "context"
    "log/slog"
    "time"

    "github.com/sonar-solutions/sonar-insights/internal/sonarqube"
    "github.com/spf13/cobra"
)

// CollectOptions holds the parameters passed from cmd to a target's Collect.
type CollectOptions struct {
    OutDir   string
    Parallel int
    Logger   *slog.Logger
}

// AnalyzeOptions holds the parameters passed from cmd to a target's Analyze.
type AnalyzeOptions struct {
    DataDir    string
    ReportDir  string
    ReportName string
    From       *time.Time
    To         *time.Time
    Logger     *slog.Logger
}

type Target interface {
    Name() string                              // e.g. "bgtasks"
    SupportsProduct(p sonarqube.Product) bool  // for [[007-BUG-...]]
    CollectFlags(*cobra.Command)               // register subcommand-local flags
    AnalyzeFlags(*cobra.Command)
    Collect(ctx context.Context, inst sonarqube.SonarInstance, opts CollectOptions) error
    Analyze(ctx context.Context, opts AnalyzeOptions) error
    DefaultReportName() string                 // used as the default for --report-name flag
}

var registry []Target

func Register(t Target)              { registry = append(registry, t) }
func All() []Target                  { return registry }
func Get(name string) (Target, bool) { ... }
```

Create `internal/targets/bgtasks/bgtasks.go` (a new package) that
implements `Target` for the existing bgtasks collector/analyzer and
registers itself:

```go
package bgtasks

import "github.com/sonar-solutions/sonar-insights/internal/targets"

func init() {
    targets.Register(&Bgtasks{})
}

type Bgtasks struct{}

func (b *Bgtasks) Name() string { return "bgtasks" }
func (b *Bgtasks) DefaultReportName() string { return "report-bgtasks" }
func (b *Bgtasks) SupportsProduct(p sonarqube.Product) bool { return p == sonarqube.Server }
// ... CollectFlags, AnalyzeFlags, Collect, Analyze delegate to existing packages
```

**Triggering `init()`:** add a blank import in `cmd/root.go` (or a
dedicated `cmd/targets.go` file) so the registration runs at startup:

```go
import _ "github.com/sonar-solutions/sonar-insights/internal/targets/bgtasks"
```

**`cmd/collect.go` parent command** — preserve the existing behaviour
where `sonar-insights collect` (with no subcommand) runs all targets:

```go
// Parent RunE — iterate all registered targets
collectCmd.RunE = func(cmd *cobra.Command, args []string) error {
    for _, t := range targets.All() {
        if err := runCollectTarget(cmd.Context(), t, ...); err != nil {
            return err
        }
    }
    return nil
}

// Subcommands — one per target
for _, t := range targets.All() {
    t := t
    sub := &cobra.Command{Use: t.Name(), RunE: func(cmd *cobra.Command, _ []string) error {
        return runCollectTarget(cmd.Context(), t, ...)
    }}
    t.CollectFlags(sub)
    collectCmd.AddCommand(sub)
}
```

Apply the same pattern in `cmd/analyze.go` and `cmd/run.go`.

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
