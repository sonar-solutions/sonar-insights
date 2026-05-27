---
spec: 018
title: Use structured slog key/value attributes instead of `fmt.Sprintf`-built log messages
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Use structured slog key/value attributes instead of `fmt.Sprintf`-built log messages

## Problem

Logging across the codebase looks like this:

```go
logger.Debug(fmt.Sprintf("detected SonarQube Server version: %s", instance.Version))
logger.Info(fmt.Sprintf("loaded %d unique background tasks", len(tasks)))
logger.Info(fmt.Sprintf("[timing] %s finished in %.2fs", cmd.Name(), elapsed.Seconds()))
logger.Error(fmt.Sprintf("failed to fetch page %d: %v", r.page, r.err))
```

This is the pre-slog idiom. It throws away every advantage of `slog`:

- Log fields cannot be filtered or queried by key.
- Structured-log handlers (`slog.NewJSONHandler`) get a single opaque
  `msg` field; the page numbers and version strings are not indexable.
- Errors lose their wrap chain — `%v` produces a flat string, not the
  underlying error.

## Why it matters

- The project's growth path includes ingestion of large amounts of data.
  When somebody needs to track down "which page failed", they will want to
  grep for `page=42`, not parse free-form English.
- Cost is trivial — find/replace plus a handler swap once.

## Proposed fix

Replace all `fmt.Sprintf` calls inside logger calls with slog key/value
attributes. The full list of calls to update (11 total; `internal/reporter`
is deleted by [[016-IMP-remove-dead-reporter-package]]):

| File | Current | New |
|---|---|---|
| `cmd/collect.go` | `logger.Debug(fmt.Sprintf("detected SonarQube Server version: %s", ...))` | `logger.Debug("detected SonarQube Server", "version", instance.Version)` |
| `cmd/root.go` | `logger.Info(fmt.Sprintf("[timing] %s finished in %.2fs", ...))` | `logger.Info("timing", "command", cmd.Name(), "elapsed_s", elapsed.Seconds())` |
| `internal/collector/bgtasks.go` | `logger.Debug(fmt.Sprintf("collected page 1 of %d", ...))` | `logger.Debug("collected page", "page", 1, "total", totalPages)` |
| `internal/collector/bgtasks.go` | `logger.Error(fmt.Sprintf("failed to fetch page %d: %v", r.page, r.err))` | `logger.Error("page fetch failed", "page", r.page, "err", r.err)` |
| `internal/collector/bgtasks.go` | `logger.Debug(fmt.Sprintf("collected page %d of %d", ...))` | `logger.Debug("collected page", "page", page, "total", totalPages)` |
| `internal/analyzer/bgtasks.go` | `logger.Info(fmt.Sprintf("loaded %d unique background tasks", ...))` | `logger.Info("loaded background tasks", "count", len(tasks))` |
| `internal/analyzer/bgtasks.go` | `logger.Info(fmt.Sprintf("%d tasks remain after date filtering", ...))` | `logger.Info("tasks after date filter", "count", len(filtered))` |
| `internal/analyzer/bgtasks.go` | `logger.Info(fmt.Sprintf("report written to %s", ...))` | `logger.Info("report written", "path", reportPath)` |
| `internal/analyzer/bgtasks/loader.go` | `logger.Debug(fmt.Sprintf("found %d bgtasks files to load", ...))` | `logger.Debug("found bgtasks files", "count", len(files))` |
| `internal/analyzer/bgtasks/loader.go` | `logger.Debug(fmt.Sprintf("removed %d duplicate tasks (kept %d unique)", ...))` | `logger.Debug("deduplicated tasks", "removed", removed, "kept", kept)` |
| `internal/analyzer/bgtasks/loader.go` | `logger.Debug(fmt.Sprintf("loaded %d tasks from %s", ...))` | `logger.Debug("loaded tasks from file", "count", n, "file", path)` |

For errors pass the `error` value directly — slog preserves the type:
`logger.Error("...", "err", err)` (not `"err", err.Error()`).

The `--log-format=text|json` flag is out of scope for this spec. If desired,
add it as a separate improvement after this change lands.

## Validation

Run `go test ./...`. Existing tests assert on error values returned by
functions (via `fmt.Errorf`), not on log output — so logging changes do
not affect any existing test assertions. No test updates are needed.

## Prerequisites

None.
