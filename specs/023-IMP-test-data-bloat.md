---
spec: 023
title: Move 180-file golden-master dataset out of the source tree
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Move 180-file golden-master dataset out of the source tree

## Problem

`test-data/` contains 18 JSON files (one per page) totalling several MB of
captured production-like SonarQube `/api/ce/activity` responses. They are
the input for the golden-master tests in:

- `internal/analyzer/bgtasks/loader_test.go` (`TestLoad_GoldenMasterTaskCount`)
- `internal/analyzer/bgtasks/metrics_test.go` (`TestAnalyzeDateRange_GoldenMaster`,
  `TestAnalyzeOverall_GoldenMaster`, `TestAnalyzeProjectAnalysis_GoldenMaster`,
  `TestAnalyzeCharts_GoldenMaster`)
- `internal/analyzer/bgtasks/capacity_test.go` (`TestCalculateCapacityDemand_GoldenMaster`)

These tests verify high-level invariants ("dataset has 3815 unique tasks";
"capacity calculation produces 14138 buckets") which are real and worth
keeping. The cost:

- Every clone of the repo downloads the dataset.
- Every CI run reads all 18 files even when the tested code is unrelated.
- Any new contributor sees a lot of opaque JSON in the project root.

## Why it matters

- The dataset isn't documentation, it's a test fixture. Mixing it with
  source code obscures the actual codebase.
- Some files (e.g. the `sonar-data/` directory still tracked in repo) are
  similar data dumps left from local runs.
- Tests that depend on the dataset are also the *slowest* tests
  (`Load` alone walks and parses 18 files in parallel). They are
  effectively integration tests, but they run with `go test ./...`
  unconditionally.

## Proposed fix

Two options, in order of effort:

**Option A — keep in repo, mark as integration tests.** Wrap each
golden-master test in `testing.Short()`. Implement Option A. Apply the guard
to all six golden-master test functions:

| File | Function |
|---|---|
| `internal/analyzer/bgtasks/loader_test.go` | `TestLoad_GoldenMasterTaskCount` |
| `internal/analyzer/bgtasks/metrics_test.go` | `TestAnalyzeDateRange_GoldenMaster` |
| `internal/analyzer/bgtasks/metrics_test.go` | `TestAnalyzeOverall_GoldenMaster` |
| `internal/analyzer/bgtasks/metrics_test.go` | `TestAnalyzeProjectAnalysis_GoldenMaster` |
| `internal/analyzer/bgtasks/metrics_test.go` | `TestAnalyzeCharts_GoldenMaster` |
| `internal/analyzer/bgtasks/capacity_test.go` | `TestCalculateCapacityDemand_GoldenMaster` |

```go
func TestAnalyzeOverall_GoldenMaster(t *testing.T) {
    if testing.Short() {
        t.Skip("golden master test: skipped with -short")
    }
    ...
}
```

CI runs `go test ./...` (full suite); developers run `go test -short ./...`
for a fast local loop. Zero changes to repo layout.

**Option B — extract.** Move `test-data/` to a sibling repo or a
git-submodule. Tests fetch on demand or skip if missing.

Option A is the right first step; Option B can come later if the data
grows.

## Validation

- `go test -short ./...` completes without errors but skips the golden
  master tests.
- `go test ./...` continues to exercise everything.

## Prerequisites

None.
