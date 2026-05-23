---
spec: 002
title: Analyze Background Tasks (bgtasks)
author: lfrystak
date: 2026-05-23
draft-status: draft
impl-status: ready-started
prerequisites: [001]
---

# Spec: Analyze Background Tasks (bgtasks)

## Purpose

Read the collected SonarQube Compute Engine background task data and produce a statistical
analysis report covering overall task throughput, project analysis performance, and capacity
demand. The report gives SonarQube administrators the data they need to right-size their
Compute Engine worker configuration.

## Goal

Port the analysis functionality implemented in the C# reference project at
`/Users/lukas/repos/sonar-insights-cs` to this Go application, with an improved internal
architecture. The C# implementation is the authoritative source for analysis logic, formulas,
and report structure. Any deviation from it must be called out explicitly in this spec or
agreed upon during implementation.

## CLI restructuring

This spec introduces a structural change to the `collect`, `analyze`, and `run` commands.
Targets currently passed as positional arguments become Cobra subcommands, enabling
per-target flags without polluting the shared flag set.

### New command structure

```
sonar-insights collect              # run all targets with defaults
sonar-insights collect bgtasks      # run bgtasks target only

sonar-insights analyze              # run all targets with defaults
sonar-insights analyze bgtasks [flags]

sonar-insights run                  # collect + analyze, all targets
sonar-insights run bgtasks [flags]  # collect + analyze bgtasks only
```

When invoked without a target subcommand, each command runs all known targets using their
defaults. Shared flags (`--dir`, `--report-dir`, `--out-dir`, `--parallel`) remain on the
parent command. Per-target flags live only on the target subcommand.

The existing `--report-name` flag is removed from `collect`, `analyze`, and `run` parent
commands. Each target subcommand produces a default report filename; `analyze bgtasks`
exposes its own `--report-name` flag so users generating only the bgtasks report can
override the output filename.

### Flags

**`collect` (parent command)**

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | (env: `SONAR_HOST_URL`, fallback: `https://sonarcloud.io`) | SonarQube base URL |
| `--token` | (env: `SONAR_TOKEN`) | SonarQube authentication token |
| `--out-dir` | `./sonar-data/` | Directory to write collected data |
| `--parallel` | `5` | Number of pages to fetch concurrently |

**`collect bgtasks` (subcommand)**

No flags specific to this target. All configuration is inherited from the parent.

**`analyze` (parent command)**

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `./sonar-data/` | Directory containing collected data |
| `--report-dir` | `./sonar-reports/` | Directory where reports are written |

**`analyze bgtasks` (subcommand)**

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | (none) | Include tasks submitted on or after this date (`YYYY-MM-DD`, UTC) |
| `--to` | (none) | Include tasks submitted on or before this date (`YYYY-MM-DD`, UTC) |
| `--report-name` | `report-bgtasks` | Output report filename (without `.html` extension) |

**`run` (parent command)**

Carries all flags from both `collect` and `analyze` parent commands.

**`run bgtasks` (subcommand)**

Inherits all flags from both `collect bgtasks` and `analyze bgtasks` subcommands.

## Data source

Input: all `*.json` files found recursively under `<dir>/bgtasks/`, written by the
`collect bgtasks` command. Files are the verbatim `GET /api/ce/activity` responses
described in spec 001.

Files are loaded in parallel using a fixed pool of 8 goroutines. This limit is intentionally
not exposed as a flag — it controls local disk I/O concurrency, not network concurrency, and
8 is sufficient for all expected data sizes. After loading, tasks are deduplicated by `id` —
duplicate IDs can appear across pages in real SonarQube data.

## Date filtering

When `--from` and/or `--to` are provided, tasks are filtered on the `submittedAt` field
after loading and deduplication, before any analysis runs. Both flags are optional and
independent.

- `--from 2026-01-01` → `submittedAt >= 2026-01-01T00:00:00Z`
- `--to 2026-03-31` → `submittedAt <= 2026-03-31T23:59:59Z`

All `submittedAt` values are normalised to UTC before comparison.

## Internal architecture

```
internal/
  mathutil/
    percentile.go          ← generic linear-interpolation percentile (reusable)
  analyzer/
    bgtasks.go             ← public entry point: AnalyzeBgTasks(dir, reportDir, from, to, logger)
    bgtasks/
      models.go            ← BgTask, TasksRoot structs; timestamp parsing to UTC
      loader.go            ← parallel file loading, JSON deserialisation, deduplication
      metrics.go           ← all analysis functions
      capacity.go          ← capacity demand calculator
      report.go            ← transforms BgTaskAnalysisResults → rptgen report
```

`internal/mathutil.CalculatePercentile[T]` is a generic function using linear interpolation.
It must work on any ordered numeric type and be independently testable. It is the Go
equivalent of `Shared/CalculationExtensionMethods.CalculatePercentile<T>` in the C#
reference.

All analysis functions in `metrics.go` and `capacity.go` are pure: they take a task slice
and return results structs with no I/O or global state.

## Analysis pipeline

The pipeline mirrors `CollectedBgTasksAnalyzer.Analyze()` in the C# reference exactly.

### 1. Date range
- Earliest `submittedAt` across all tasks
- Latest `executedAt` across all tasks
- Inclusive date range in days (UTC date-only comparison)

### 2. Overall metrics
- Raw task count and unique task count (after deduplication)
- Average tasks per day
- Busiest single day (by submission date, UTC)
- Percentage of failed tasks

### 3. Project analysis metrics
Filters to `type == "REPORT"` tasks only.

- Total REPORT tasks, distinct projects analysed, average analyses per day
- Split into PR tasks (`pullRequest != ""`) vs. branch tasks
- Average and 80th-percentile execution time (seconds) computed separately for PR and branch tasks
- Top 10 projects by count (ties broken alphabetically), with count and percentage of total
- Execution time series for the top 3 most-analysed projects: for each, select the
  branch with the highest task count (`branchType == "BRANCH"`; ties broken alphabetically
  by branch name), return ordered `startedAt → executionTimeSec` pairs

### 4. Chart datasets
- Tasks per day (all types) — zero-filled for every day with no tasks. The fill range
  is `--from` to `--to` when those flags are set; otherwise the earliest `submittedAt`
  to the latest `executedAt` across all tasks. Days outside the active range are not included.
- Project analyses per day (REPORT only) — zero-filled using the same range rules
- Tasks per day broken down by type (stacked bar chart data)
- Time category histograms for pending time (`startedAt − submittedAt`) and execution
  time (`executionTimeMs / 1000`), bucketed into 8 fixed categories:
  `0–1s`, `1–3s`, `3–5s`, `5–10s`, `10–30s`, `30–60s`, `60–180s`, `>180s`.
  All buckets are inclusive on the upper bound and exclusive on the lower bound, except
  `0–1s` which is inclusive on both bounds. Concretely: a value of 1s falls into `0–1s`;
  a value of 3s falls into `1–3s`; a value of 5s falls into `3–5s`; and so on.
- Summary breakdowns by type, status, submitter, and warning count

### 5. Capacity demand
Implemented in `capacity.go`, mirroring `CapacityDemandCalculator` in the C# reference.

- Excludes `ISSUE_SYNC` tasks (reindexing periods skew results)
- Divides the full timeline into 5-minute UTC-aligned buckets
- For each task, uses `submittedAt` as the theoretical execution start (not `startedAt`)
  and distributes `executionTimeMs` proportionally across all overlapping buckets
- Single-worker utilisation = `bucketDemandMs / (bucketLengthMin × 60 × 1000) × 100`
- Computes percentiles at 85th, 90th, 95th, 99th, 99.5th, 99.9th, and 100th for two
  slices: all buckets (24/7) and weekday-only (Monday–Friday)
- Detects clusters of consecutive busy buckets: a cluster is a contiguous run of buckets
  whose utilisation exceeds the 99.9th-percentile value. The output is a list of clusters,
  each described by its start time, end time, and bucket count. The goal is to identify
  sustained busy periods rather than isolated spikes.
- **Worker recommendation**: uses the 99th-percentile utilisation from whichever slice
  (24/7 or weekday) is higher; workers needed = `ceil(utilisation / 100)`

## Output

Report file: `<report-dir>/report-bgtasks.html`

The `<report-dir>` is created if it does not exist. Existing reports in the directory
are not deleted — reports accumulate across runs. If `report-bgtasks.html` already exists
it is overwritten.

The report structure mirrors the C# reference output exactly (four sections: introduction,
overall summary, project analysis summary, capacity demand analysis). Use
`github.com/lfrystak/rptgen` for rendering.

## Further requirements

### Logging
- INFO: analysis started, number of tasks loaded and after filtering, report path written
- DEBUG: number of files found, tasks per file, tasks removed by deduplication

### Error handling
- Missing `<dir>/bgtasks/` directory: log error and abort
- Malformed JSON file: log the filename and error, then abort
- Empty task list after loading and deduplication (before any filtering): log a clear
  error (e.g. "no tasks found in `<dir>/bgtasks/`") and abort
- Empty task list after date filtering: log a clear error (e.g. "no tasks matched the
  specified date range") and abort

### Testing
- `internal/mathutil` must have unit tests covering percentile correctness: single element,
  all-equal input, unsorted input, multiple percentile levels
- `capacity.go` must have unit tests covering bucket generation, proportional demand
  distribution, percentile output, and cluster detection
- `metrics.go` must have unit tests for each analysis function
- Use the C# golden master data at `/Users/lukas/repos/sonar-insights-cs/golden-masters/`
  as a reference for expected output values

### Reference implementation
Before implementing, read the C# source at `/Users/lukas/repos/sonar-insights-cs`. Analysis
logic, formulas, and bucket boundaries must match the reference exactly. Deviations require
explicit justification.
