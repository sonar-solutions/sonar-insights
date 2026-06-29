---
spec: 031
title: Capacity Estimate (workers → supportable jobs)
author: lfrystak
date: 2026-06-26
draft-status: ready
impl-status: complete
prerequisites: [002]
---

# Spec: Capacity Estimate (workers → supportable jobs)

## Purpose

Invert the existing capacity-demand analysis. Spec 002 reads real Compute Engine data and
*recommends a worker count*. This feature does the opposite: given a *planned* worker count,
it estimates how many project analyses (by size) that capacity can support per hour. It lets a
SonarQube admin who is sizing a new or resized instance answer "if I provision N workers, how
many analyses can I run?" instead of "how many workers do I need for my current load?".

## Goal

After this exists, the user can run `analyze bgtasks --estimate-workers 4,8,16` against
collected data and see, in the logs, an estimated per-hour project-analysis throughput broken
down by size category (XXS…XXXL) for each requested worker count, with a ±20% error margin.

This is an **experimental** feature. Its only output is log messages. It must **not** change
the generated HTML report in any way — when `--estimate-workers` is omitted, behaviour and
report output are byte-for-byte identical to today.

## CLI design

A new flag on the existing `analyze bgtasks` subcommand:

| Flag | Default | Description |
|------|---------|-------------|
| `--estimate-workers` | (unset) | Comma-separated / repeatable list of worker counts to estimate for, e.g. `4,8,16`. When unset, the estimate does not run. |

```
sonar-insights analyze bgtasks --estimate-workers 4,8,16
sonar-insights analyze bgtasks --estimate-workers 4 --estimate-workers 8   # repeated form
```

- The flag is an integer slice (cobra `IntSlice`). Values must be `>= 1`; any value `< 1`
  is a usage error (`--estimate-workers: worker counts must be >= 1, got %d`).
- Duplicate values are de-duplicated; output order follows the de-duplicated ascending sort.
- The same flag is added to `run bgtasks` for parity (it inherits the analyze surface), with
  identical semantics. This is secondary; `analyze bgtasks` is the primary surface.
- All existing flags (`--from`, `--to`, `--report-name`, inherited `--data-dir`,
  `--report-dir`) are unchanged.

### Expected on-screen output

Nothing is written to disk beyond the usual report. When `--estimate-workers` is set, the
estimate is logged (see *Logging*). Illustrative INFO output for `--estimate-workers 4,8`:

```
INFO  capacity estimate: REPORT (project analysis) accounts for 62.4% of non-ISSUE_SYNC compute time
INFO  capacity estimate (per hour, ±20%) for 4 workers: total ≈ 1240 analyses (992–1488)
INFO    XXS (0-1s): ≈ 610 (488–732)   XS (1-3s): ≈ 240 (192–288)   ...   XXXL (180-540s): ≈ 1 (1–1)
INFO  capacity estimate (per hour, ±20%) for 8 workers: total ≈ 2480 analyses (1984–2976)
INFO    XXS (0-1s): ≈ 1220 (976–1464)  ...
```

## Data source

No new data source. Operates on the same in-memory `[]bgtasks.BgTask` slice already loaded,
deduplicated, and date-filtered by the analyze pipeline (spec 002). No SonarQube connection.

Relevant fields: `Type`, `ExecutionTimeMs`.

## Algorithm

All steps operate on the post-load, post-date-filter task slice `T`.

### Step 0 — Exclude ISSUE_SYNC
`A = { t ∈ T : t.Type != "ISSUE_SYNC" }`. `ISSUE_SYNC` tasks represent reindexing periods and
would skew every ratio below. They are excluded from **all** sums in this feature.

### Step 1 — Isolate project analyses
`R = { t ∈ A : t.Type == "REPORT" }` (`R ⊆ A`).

### Step 2 — Share of compute spent on project analyses
```
reportShare = Σ_{t∈R} t.ExecutionTimeMs  /  Σ_{t∈A} t.ExecutionTimeMs
```
This is the fraction of (non-ISSUE_SYNC) worker time available to project analyses; the
remainder is consumed by other task types. `0 <= reportShare <= 1`.

Guards: if `Σ_{t∈A} ExecutionTimeMs == 0` or `R` is empty, log a clear message and skip the
estimate entirely (no division).

### Step 3 — Size categories and their share of REPORT time
REPORT tasks are bucketed by execution time into eight categories. The bucket boundaries reuse
the eight thresholds already defined in `metrics.go` (`timeThresholds`); the XXS…XXXL names are
labels introduced by this feature and are not present in `metrics.go` (which uses range labels
like `"0-1s"` … `"> 180s"`). The seven lower categories' display labels reuse the corresponding
`timeThresholds` range label (e.g. `XXS (0-1s)`); the XXXL label is constructed as
`180-{maxObservedSec}s` (see below). Lower bound exclusive, upper bound inclusive, except XXS
which is inclusive at 0:

| Category | Range (execution time) |
|----------|------------------------|
| XXS | 0 – 1s |
| XS  | 1 – 3s |
| S   | 3 – 5s |
| M   | 5 – 10s |
| L   | 10 – 30s |
| XL  | 30 – 60s |
| XXL | 60 – 180s |
| XXXL | 180s – **max observed** |

The XXXL upper bound is **not** hardcoded. The top bucket is open-ended (`> 180s`), and its
reported upper bound is the maximum REPORT execution time present in the data (used for the
label only; membership is "anything over 180s"). For each category `c`:
```
categoryShare_c = Σ_{t∈R_c} t.ExecutionTimeMs  /  Σ_{t∈R} t.ExecutionTimeMs
```
The eight `categoryShare_c` values sum to 1.

### Step 4 — Representative per-job cost (mode) per category
Within each category, find the most frequently occurring execution time and use it as the
typical per-job cost for that category:
```
representativeSec_c = mode_{t∈R_c}( round(t.ExecutionTimeMs / 1000, 0.1s) )
```
- Execution times are rounded to the nearest **0.1s** before taking the mode, because raw
  millisecond values are effectively continuous and a raw mode would be degenerate. On ties
  (two rounded values equally frequent) pick the smaller value (deterministic).
- Clamp the result to a floor of **0.1s** so sub-50ms tasks cannot produce a zero cost
  (division guard).
- The 0.1s rounding granularity is a documented heuristic and the most likely tuning knob for
  this experimental feature. It is a named constant in code, not a flag (yet).
- Empty category → no representative cost is needed (its `categoryShare_c` is 0, so it
  contributes 0 jobs).

### Step 5 — Estimate jobs per worker count
For each requested worker count `n`:
```
capacityPerHourSec(n)  = n * 3600                      # n worker-hours of compute per clock-hour
reportCapacitySec(n)   = capacityPerHourSec(n) * reportShare
catCapacitySec_c(n)    = reportCapacitySec(n) * categoryShare_c
jobs_c(n)              = catCapacitySec_c(n) / representativeSec_c    # 0 when category empty
totalJobs(n)           = Σ_c jobs_c(n)
```
Apply the ±20% margin to every reported figure: `low = x * 0.8`, `high = x * 1.2`.

Job counts are fractional internally; log them rounded to the nearest whole analysis (totals
and per-category alike). The ±20% bounds are rounded the same way.

## Internal architecture

```
internal/analyzer/bgtasks/
  estimate.go        ← new: pure estimation functions + result structs (documented algorithm)
  estimate_test.go   ← new: unit tests
internal/analyzer/bgtasks.go
                     ← AnalyzeBgTasks gains an estimateWorkers []int parameter; when non-empty,
                       runs the estimator and logs results AFTER the report is written.
                       The report build is untouched.
cmd/analyze.go       ← --estimate-workers IntSlice flag; threaded via a new
                       withEstimateWorkers([]int) analyzeOption.
cmd/run.go           ← same flag for parity.
```

- `estimate.go` functions are pure: take `[]BgTask` (and worker counts) and return a result
  struct, no I/O, no globals — consistent with `metrics.go` / `capacity.go`.
- The estimate result is **not** added to `AnalysisResults` and is **not** passed to
  `report.go`. It exists only to be logged.
- Suggested shape:
  ```go
  type CategoryEstimate struct {
      Label              string
      UpperBoundSec      int      // observed max for XXXL
      Share              float64  // categoryShare_c
      RepresentativeSec  float64  // mode, clamped
      Jobs, JobsLow, JobsHigh float64
  }
  type WorkerEstimate struct {
      Workers     int
      Categories  []CategoryEstimate
      TotalJobs, TotalLow, TotalHigh float64
  }
  type CapacityEstimate struct {
      ReportShare    float64
      MarginPct      float64        // 0.20
      PerHour        bool           // true
      WorkerEstimates []WorkerEstimate
  }
  ```

## Validation

### Acceptance criteria
- With `--estimate-workers` **omitted**: no estimate log lines appear and the generated
  `report-bgtasks.html` is identical to a run on the same data without this feature.
- With `--estimate-workers 4,8`: exactly two per-worker estimate blocks are logged (ascending),
  each listing all eight size categories plus a total, every figure carrying a ±20% range, and
  the report is unchanged.
- `ISSUE_SYNC` tasks are excluded from `reportShare` and from every category sum.
- `Σ categoryShare_c == 1` (within float tolerance) whenever `R` is non-empty.
- No panics / divide-by-zero on empty categories, sub-second tasks, or all-zero execution times.

### Test scenarios
- Happy path: a hand-built task set with known REPORT/other split and known per-category modes
  → assert `reportShare`, each `categoryShare_c`, each `representativeSec_c`, and `jobs_c(n)`
  for a couple of worker counts.
- No REPORT tasks: `R` empty → estimator returns a "skipped" signal; entry point logs a clear
  message and does not error; report still written.
- All non-ISSUE_SYNC time is REPORT: `reportShare == 1`.
- Mode floor: a category containing only sub-50ms tasks → `representativeSec_c == 0.1`, finite
  job count.
- Duplicate / unsorted / `<1` worker inputs: de-dup + sort; `<1` rejected at the CLI layer.
- XXXL label reflects the maximum observed REPORT execution time.

### Smoke test
`sonar-insights -v analyze bgtasks --estimate-workers 4,8,16`
Expected: three estimate blocks in the log (per-hour, ±20%, by size), a `reportShare` line, the
DEBUG intermediate-arithmetic lines for each step (visible because of `-v`), and
`report-bgtasks.html` written exactly as before.

## Further requirements

### Logging

**Every step of the calculation must be traceable through log messages.** Because the feature
is experimental and intended to be checked against real-world throughput, a reader must be able
to reconstruct each result from the logs alone — every intermediate value that feeds a result
is logged, not just the final job counts. The split is INFO for headline results, DEBUG for the
intermediate arithmetic, mapped to the algorithm steps:

| Algorithm step | Logged value(s) | Level |
|----------------|-----------------|-------|
| 0 — Exclude ISSUE_SYNC | count of tasks excluded; count of non-ISSUE_SYNC tasks (`|A|`) | DEBUG |
| 1 — Isolate REPORT | count of REPORT tasks (`|R|`) | DEBUG |
| 2 — Report share | `Σ REPORT ms`, `Σ non-ISSUE_SYNC ms`, and the resulting `reportShare` % (headline) | DEBUG sums, INFO % |
| 3 — Category shares | per category: task count, `Σ ms`, `categoryShare_c` %, and the observed XXXL upper bound | DEBUG |
| 4 — Representative cost | per category: the mode (`representativeSec_c`), and whether the 0.1s floor was applied | DEBUG |
| 5 — Per-worker estimate | per worker count `n`: `capacityPerHourSec(n)`, `reportCapacitySec(n)`, per-category `catCapacitySec_c(n)` | DEBUG |
| 5 — Results | per worker count `n`: total jobs and per-category jobs, each with its ±20% range | INFO |

- At least one INFO line names the basis explicitly ("per hour, ±20%") so the numbers are never
  ambiguous out of context.
- When the estimate is skipped (no REPORT data / zero total compute): a single clear INFO line
  explaining why; never an error.
- DEBUG lines surface only when the existing root `-v` / `--verbose` flag is set (project
  convention); the headline INFO lines appear at the default level. Full step-by-step
  traceability therefore requires running with `-v`.

### Error handling
- `--estimate-workers` values `< 1` → usage error before analysis runs.
- Empty / non-REPORT data → skip-with-log, not an error (the report path is unaffected).
- The estimate must never abort report generation; it runs after the report is written.

### Constants
- Mode rounding granularity (`0.1s`), cost floor (`0.1s`), margin (`0.20`), and seconds-per-hour
  (`3600`) are named constants in `estimate.go`.

### Open questions / notes (experimental)
- **Mode appropriateness.** Mode on rounded data is the chosen representative cost per the
  feature owner's design. Mean or median would be more stable; if real-world comparison shows
  the estimate is off, revisit the representative-cost choice and/or the 0.1s granularity first.
- The per-hour basis assumes workers run continuously; a per-day figure is just `× 24` and can
  be added later if useful.
