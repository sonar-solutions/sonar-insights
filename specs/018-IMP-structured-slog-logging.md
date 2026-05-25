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

Use the slog field idiom:

```go
logger.Debug("detected SonarQube Server", "version", instance.Version)
logger.Info("loaded background tasks", "count", len(tasks), "unique", len(unique))
logger.Info("timing", "command", cmd.Name(), "elapsed", elapsed)
logger.Error("page fetch failed", "page", r.page, "err", r.err)
```

For errors, use `slog.Any("err", err)` or just pass the `error` value —
slog preserves the underlying type.

For human readability of the current TextHandler output, slog already
formats key/value pairs nicely; switching to `slog.NewJSONHandler` later
becomes a one-line change.

Consider exposing a `--log-format=text|json` flag.

## Validation

The existing tests assert error messages contain certain substrings; those
assertions should continue to pass because the substrings are in the
error chain, not the log line. Run `go test ./...`.

## Prerequisites

None.
