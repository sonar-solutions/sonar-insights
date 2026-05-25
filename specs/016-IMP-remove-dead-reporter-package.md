---
spec: 016
title: Remove dead `internal/reporter` package
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Remove dead `internal/reporter` package

## Problem

`internal/reporter/reporter.go`:

```go
package reporter

import (
    "fmt"
    "log/slog"
)

// TODO: import github.com/lfrystak/rptgen once report implementation begins

// Generate builds and writes the HTML report to disk.
func Generate(reportName string, logger *slog.Logger) error {
    logger.Info(fmt.Sprintf("[reporter] would generate report: %s.html", reportName))
    // TODO: use rptgen to assemble sections and write the HTML report file
    return nil
}
```

The package is never imported. Reporting has been implemented inside the
`bgtasks` analyzer package (`internal/analyzer/bgtasks/report.go`), which is
the correct location once you accept that Server vs. Cloud and different
targets will generate different reports. `internal/reporter` is leftover
scaffolding.

## Why it matters

- Dead code signals confusion to a reader: "is this the canonical reporter,
  or the new one in `bgtasks/`?"
- `Generate` is exported, so anything importing it would get a no-op report
  with no warning.
- Stale TODOs train readers to ignore TODOs.

## Proposed fix

Delete `internal/reporter/`. Verify no imports first:

```sh
grep -r 'sonar-insights/internal/reporter' .
```

Should return nothing (verified at review time — the package is unused).

## Validation

`go build ./...` and `go test ./...` continue to pass after deletion.

## Prerequisites

None.
