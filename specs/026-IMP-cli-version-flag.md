---
spec: 026
title: Add `--version` flag and inject build metadata at link time
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Add `--version` flag and inject build metadata at link time

## Problem

`sonar-insights --version` does not work. The root command has no version
declared, so Cobra synthesizes a "command not found"-style response.

Symptoms:

- Users can't tell which version they're running. Bug reports come in
  without that critical piece of information.
- The `.goreleaser.yaml` is set up to tag versioned releases, but the
  resulting binary has no way to tell you what tag it is.

## Why it matters

- Operational hygiene — minimum bar for a release-tagged CLI.
- Trivial to add.
- Unlocks correlation between bug reports and source state.

## Proposed fix

1. Declare version in `cmd/root.go`:

   ```go
   var (
       // Set via -ldflags at build time.
       Version = "dev"
       Commit  = "none"
       Date    = "unknown"
   )

   var rootCmd = &cobra.Command{
       Use:     "sonar-insights",
       Short:   "Provides insights into SonarQube usage",
       Version: Version,
   }
   ```

2. Customize the version template if commit + date are wanted:

   ```go
   rootCmd.SetVersionTemplate(`sonar-insights {{.Version}}
   commit: ` + Commit + `
   built : ` + Date + "\n")
   ```

3. Update `.goreleaser.yaml`'s `builds.ldflags` to inject these vars at
   build time:

   ```yaml
   ldflags:
     - -s -w
     - -X github.com/sonar-solutions/sonar-insights/cmd.Version={{.Version}}
     - -X github.com/sonar-solutions/sonar-insights/cmd.Commit={{.Commit}}
     - -X github.com/sonar-solutions/sonar-insights/cmd.Date={{.Date}}
   ```

## Validation

- `go run . --version` prints `sonar-insights dev` (or with build flags,
  the actual version).
- The next tagged release's binary prints the tag.

## Prerequisites

None.
