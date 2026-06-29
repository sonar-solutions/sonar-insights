---
spec: 032
title: Capacity Estimate Refinement — mean cost + demand profile
author: lfrystak
date: 2026-06-29
draft-status: ready
impl-status: complete
prerequisites: [031]
amends: 031
---

# Spec: Capacity Estimate Refinement — mean cost + demand profile

## Purpose

Refine the experimental capacity estimate from spec [031](031-capacity-estimate.md) so its
numbers are easier to trust and easier to act on. Spec 031 shipped two design choices that this
amendment replaces: the **mode** as the representative per-job cost (which systematically
over-estimates throughput), and a **flat ±20% band** that did not actually reflect anything in
the data. This spec switches the representative cost to the **mean**, and replaces the arbitrary
band with a **demand profile** that contrasts the workers' throughput capacity against how the
operator's analysis load actually arrives over time (average vs. peak hour).

## Goal

After this exists, `analyze bgtasks --estimate-workers 4,8,16` still logs a per-hour, per-size
capacity breakdown for each worker count, but:

1. Each category's per-job cost is the **mean** of that category's execution times, so the total
   reduces to the provably-correct identity `totalJobs = reportCapacity ÷ averageJobCost`.
2. All **eight** size categories are always listed; empty categories are shown explicitly as
   having no tasks (rather than silently omitted).
3. Instead of a ±20% band, the estimate reports a **demand profile** — the operator's average and
   peak REPORT arrival rate per hour — and, for each worker count, a plain-language verdict on
   whether that capacity keeps up with average load and with peak-hour load.

This remains an **experimental, log-only** feature. When `--estimate-workers` is omitted, the
generated `report-bgtasks.html` is byte-for-byte identical to today (unchanged from 031).

## What this changes relative to 031

This is an amendment; the implementer should **modify** the 031 implementation, not add a parallel
one. Concretely:

| Area | 031 (current) | 032 (this spec) |
|------|---------------|-----------------|
| Representative cost (Step 4) | mode of execution times rounded to 0.1s, smaller value on ties | **mean** of execution times in the category |
| Rounding granularity | `modeRoundingSec = 0.1` constant + tie-break rule | **removed** (no rounding, no tie-break) |
| Cost floor | `modeFloorSec = 0.1` | kept, renamed `costFloorSec = 0.1` |
| Uncertainty signal | flat ±20% band on every figure (`estimateMargin`, `JobsLow/High`, `TotalLow/High`, `MarginPct`) | **removed** entirely, replaced by the demand profile + verdict |
| Empty category in INFO output | skipped (`if BucketCount == 0 { continue }`) | **always shown**, marked `no tasks` |
| Demand / timing | not measured | **new**: average & peak hourly REPORT arrival rate + per-worker verdict |
| `computeWorkerEstimate` doc | none | one-line doc comment for the `capacity → reportCapacity → catCapacity → jobs` chain |

Everything else from 031 (Steps 0–3, ISSUE_SYNC exclusion, REPORT isolation, report share, the
eight `timeThresholds` buckets, the XXXL observed-max label, guards, the log-only contract, the
`--estimate-workers` CLI surface) is **unchanged**.

## CLI design

Unchanged from 031. No new flags. `--estimate-workers` (cobra `IntSlice`, values `>= 1`,
de-duplicated and ascending) on `analyze bgtasks` and, for parity, `run bgtasks`.

### Expected on-screen output

Illustrative INFO output for `--estimate-workers 4,8` (numbers illustrative):

```
INFO  capacity estimate: REPORT (project analysis) accounts for 62.4% of non-ISSUE_SYNC compute time
INFO  capacity estimate: REPORT demand averaged ≈ 38 analyses/hour, peaking at ≈ 210/hour (over 31 days)
INFO  capacity estimate (per hour) for 4 workers: capacity ≈ 1240 analyses/hour
INFO    XXS (0-1s): ≈ 610   XS (1-3s): ≈ 240   S (3-5s): ≈ 180   M (5-10s): ≈ 120   L (10-30s): ≈ 70   XL (30-60s): ≈ 18   XXL (60-180s): ≈ 2   XXXL (> 180s): no tasks
INFO    verdict: covers your peak hour (≈ 210/hr) with ≈ 1030/hr to spare
INFO  capacity estimate (per hour) for 8 workers: capacity ≈ 2480 analyses/hour
INFO    XXS (0-1s): ≈ 1220   ...   XXXL (> 180s): no tasks
INFO    verdict: covers your peak hour (≈ 210/hr) with ≈ 2270/hr to spare
```

When capacity sits between average and peak demand, the verdict instead reads, e.g.:

```
INFO    verdict: keeps up with average demand (≈ 38/hr) but during peak hours (≈ 210/hr) ≈ 90/hr would queue and drain in quieter periods
```

When capacity is below average demand:

```
INFO    verdict: below your average demand (≈ 38/hr) — sustained backlog likely; consider more workers
```

## Data source

No new data source. Operates on the same in-memory, deduplicated, date-filtered
`[]bgtasks.BgTask` slice (spec 002). No SonarQube connection.

Relevant fields: `Type`, `ExecutionTimeMs` (as in 031), plus `SubmittedAt` for the demand profile.
`SubmittedAt` is the time a task entered the queue — i.e. true demand arrival — and is independent
of how many workers the source instance ran, so it is a clean demand signal. It is already used by
the existing date filter and `OverallMetrics` busiest-day logic.

## Algorithm

All steps operate on the post-load, post-date-filter task slice `T`, as in 031. Steps 0–3 are
**unchanged from 031** and are summarised here only for context.

### Step 0 — Exclude ISSUE_SYNC (unchanged)
`A = { t ∈ T : t.Type != "ISSUE_SYNC" }`.

### Step 1 — Isolate project analyses (unchanged)
`R = { t ∈ A : t.Type == "REPORT" }`.

### Step 2 — Share of compute spent on project analyses (unchanged)
`reportShare = Σ_{t∈R} ExecutionTimeMs / Σ_{t∈A} ExecutionTimeMs`. Guards (empty `R`, zero total)
unchanged: log a clear INFO message and skip.

### Step 2b — Demand profile (NEW)
Measure how REPORT analyses arrive over time, using `SubmittedAt`:

1. Bucket every task in `R` into clock-hour bins (truncate `SubmittedAt` to the hour, UTC).
2. `observedHours` = number of whole clock-hours spanned by `R`, from the earliest to the latest
   `SubmittedAt` inclusive (≥ 1). Hours with no arrivals count as zero-arrival hours (zero-filled),
   so the average reflects real calendar time including idle nights/weekends.
3. ```
   avgDemandPerHour  = |R| / observedHours
   peakDemandPerHour = p95( per-hour counts, zero-filled across observedHours )
   busiestHourCount  = max( per-hour counts )                 # context / DEBUG only
   ```
   The peak uses the **95th percentile** of hourly counts rather than the single busiest hour, so a
   one-off bulk re-analysis does not distort the headline peak. The absolute busiest hour is logged
   at DEBUG for transparency. Reuse `mathutil.CalculatePercentile` (already used by `metrics.go` /
   `capacity.go`); call it with `0.95`. Note it interpolates linearly between ranks (it is not
   nearest-rank), so test expectations must be computed the same way.
4. **Sufficiency guard.** If `observedHours < minObservedHours` (24), the dataset is too short to
   characterise a peak hour reliably: set `Demand.Available = false`, still report
   `avgDemandPerHour`, and omit the peak figure and the peak comparison from the verdict (see Step
   5). This is a skip-of-part, not an error.

> Scope: the demand profile is measured at the overall REPORT level, **not** per size category.
> The size-category breakdown remains on the capacity side only. Per-category demand timing is out
> of scope (data is too sparse per category-hour to be meaningful).

### Step 3 — Size categories and their share of REPORT time (unchanged)
Eight buckets from `timeThresholds`; `categoryShare_c = Σ_{t∈R_c} ms / Σ_{t∈R} ms`; XXXL upper
bound = observed max (label only). The eight shares sum to 1.

### Step 4 — Representative per-job cost: MEAN (CHANGED)
Within each category, the representative per-job cost is the **mean** execution time:
```
representativeSec_c = ( Σ_{t∈R_c} ExecutionTimeMs / |R_c| ) / 1000      # seconds
```
- Clamp to a floor of `costFloorSec` (0.1s) so a category of only sub-100ms tasks cannot produce a
  near-zero cost and an explosive job count (division guard). Record `FloorApplied` when clamped.
- Empty category → no representative cost needed (`categoryShare_c = 0`, contributes 0 jobs).
- **Removed from 031:** rounding to 0.1s, the mode, and the tie-break rule. There is no rounding
  granularity constant any more.

Rationale (record in code/spec): let `averageReportCost = (Σ_{t∈R} ExecutionTimeMs / |R|) / 1000`
(the mean cost over **all** REPORT tasks, in seconds). With the per-category mean, `totalJobs(n)`
algebraically reduces to `reportCapacitySec(n) ÷ averageReportCost` — the category split introduces
no bias (this holds exactly only when no category is clamped to the cost floor). The mode used
in 031 sits below the mean for right-skewed execution-time distributions and so over-estimated
throughput, worst for the wide buckets (L/XL/XXL/XXXL). This resolves 031's "mode appropriateness"
open question.

### Step 5 — Estimate jobs per worker count, and the demand verdict (CHANGED)
For each requested worker count `n`, capacity is computed exactly as in 031 (now with the mean
cost), but **without** the ±20% band:
```
capacityPerHourSec(n)  = n * 3600
reportCapacitySec(n)   = capacityPerHourSec(n) * reportShare
catCapacitySec_c(n)    = reportCapacitySec(n) * categoryShare_c
jobs_c(n)              = catCapacitySec_c(n) / representativeSec_c     # 0 when category empty
capacityJobsPerHour(n) = totalJobs(n) = Σ_c jobs_c(n)
```
Job counts are fractional internally; log them rounded to the nearest whole analysis. No `low`/
`high` bounds are produced.

**Verdict (NEW)** — compare `capacityJobsPerHour(n)` to the demand profile from Step 2b. Let
`cap = capacityJobsPerHour(n)`, `avg = avgDemandPerHour`, `peak = peakDemandPerHour`:

- `Demand.Available == false`: verdict reports capacity vs. average only —
  *"average demand ≈ {avg}/hr; insufficient time span to estimate peak-hour load."*
- `cap >= peak`: *"covers your peak hour (≈ {peak}/hr) with ≈ {cap−peak}/hr to spare."*
- `avg <= cap < peak`: *"keeps up with average demand (≈ {avg}/hr) but during peak hours
  (≈ {peak}/hr) ≈ {peak−cap}/hr would queue and drain in quieter periods."*
- `cap < avg`: *"below your average demand (≈ {avg}/hr) — sustained backlog likely; consider more
  workers."*

The verdict is the operationally meaningful replacement for the old band: it uses the
non-uniformity of arrivals (avg vs. peak) rather than perturbing the capacity ceiling with an
arbitrary percentage.

> Modelling assumption (note in spec): `reportShare` is an average fraction of worker time; the
> verdict assumes the non-REPORT workload mix holds during peaks. Worker scaling is linear
> (`n × 3600`), i.e. no DB/Elasticsearch contention. Both are acceptable for a rough sizing aid and
> are stated so the reader does not over-trust the figure.

## Internal architecture

Same files as 031 — modify in place:

```
internal/analyzer/bgtasks/estimate.go        ← Step 4 mean; remove mode/rounding/margin; add demand profile + verdict inputs; doc comment on computeWorkerEstimate
internal/analyzer/bgtasks/estimate_test.go   ← replace mode/tie-break/rounding tests with mean + demand-profile + verdict tests
internal/analyzer/bgtasks.go (logCapacityEstimate) ← drop ±20% formatting; always print 8 categories; add demand line + per-worker verdict line
cmd/analyze.go, cmd/run.go                   ← unchanged
```

Struct changes (`estimate.go`):

```go
type CategoryEstimate struct {
    Label             string
    UpperBoundSec     int
    Share             float64
    RepresentativeSec float64 // mean, clamped to costFloorSec
    FloorApplied      bool
    BucketCount       int
    BucketSumMs       int64
    CatCapacitySec    float64
    Jobs              float64 // jobs/hour — JobsLow/JobsHigh REMOVED
}

type WorkerEstimate struct {
    Workers    int
    Categories []CategoryEstimate
    TotalJobs  float64 // capacity jobs/hour — TotalLow/TotalHigh REMOVED
    // verdict text is derived at log time from Demand + TotalJobs
}

type DemandProfile struct { // NEW
    Available        bool    // false when observedHours < minObservedHours
    ObservedHours    int
    AvgPerHour       float64
    PeakPerHour      float64 // p95 of hourly counts
    BusiestHourCount int     // absolute max (context)
}

type CapacityEstimate struct {
    ReportShare     float64
    TotalNonSyncMs  int64
    TotalReportMs   int64
    ExcludedCount   int
    NonSyncCount    int
    ReportCount     int
    MaxObservedMs   int
    PerHour         bool
    Demand          DemandProfile // NEW
    WorkerEstimates []WorkerEstimate
    // MarginPct REMOVED
}
```

`EstimateCapacity` stays pure (no I/O, no globals): it computes and returns `Demand` and the
worker estimates; `logCapacityEstimate` formats the verdict strings.

## Validation

### Acceptance criteria
- With `--estimate-workers` **omitted**: no estimate log lines; `report-bgtasks.html` identical to
  a run without this feature (unchanged contract from 031).
- With `--estimate-workers 4,8`: two per-worker blocks (ascending), **each listing all eight size
  categories** (empty ones shown as `no tasks`), each with a single capacity figure (no ±range),
  plus one verdict line per worker. The report is unchanged.
- The representative cost per category equals the mean of that category's execution times (clamped
  to 0.1s), and `Σ jobs_c == totalJobs` equals `reportCapacitySec ÷ averageReportCost` within float
  tolerance (the mean identity) — exact when no category hit the cost floor.
- A demand line is logged with `avgDemandPerHour` and, when `Demand.Available`, `peakDemandPerHour`.
- `ISSUE_SYNC` excluded from `reportShare`, every category sum, **and** the demand profile.
- No band fields, constants, or ±20% strings remain anywhere in the feature.
- No panics / divide-by-zero on empty categories, sub-second tasks, all-zero execution times, or a
  data window shorter than `minObservedHours`.

### Test scenarios
- **Mean cost:** a category with known execution times → `representativeSec_c` is their mean;
  `jobs_c(n)` matches `catCapacitySec_c ÷ mean`.
- **Mean identity:** hand-built REPORT set with no sub-floor category (so no clamp fires) →
  `totalJobs(n) == reportCapacitySec(n) ÷ avgCost` (within tolerance), confirming the category split
  is unbiased.
- **Mode tests removed:** delete 031's tie-break and 0.1s-rounding tests.
- **Cost floor:** a category of only sub-100ms tasks → `representativeSec_c == 0.1`,
  `FloorApplied == true`, finite job count.
- **All eight categories shown:** an input leaving ≥ 1 category empty → that category appears in
  INFO output marked `no tasks`.
- **Demand profile:** a crafted arrival pattern (e.g. 24 hours, most hours ~5 tasks, one hour 50)
  → assert `avgDemandPerHour`, `peakDemandPerHour` (p95), `busiestHourCount`.
- **Verdict selection:** craft capacity vs. demand to hit each of the three branches (peak-covered,
  average-but-not-peak, below-average) and assert the chosen verdict.
- **Insufficient span:** window < 24 clock-hours → `Demand.Available == false`, average reported,
  no peak, no panic.
- **Unchanged guards:** no REPORT tasks / zero total compute → skip-with-log as in 031.
- Duplicate / unsorted / `<1` worker inputs: de-dup + sort; `<1` rejected at the CLI layer
  (unchanged).

### Smoke test
`sonar-insights -v analyze bgtasks --estimate-workers 4,8,16`
Expected: three capacity blocks (per hour, all eight categories, with verdicts), the `reportShare`
line, a demand line (avg + peak/hour), DEBUG step lines (now including the demand bins and the
absolute busiest hour), and `report-bgtasks.html` written exactly as before. No `±20%` text.

## Further requirements

### Logging
Every step must remain reconstructable from the logs (031's traceability contract). INFO for
headline results, DEBUG for intermediate arithmetic:

| Algorithm step | Logged value(s) | Level |
|----------------|-----------------|-------|
| 0 — Exclude ISSUE_SYNC | excluded count; `|A|` | DEBUG |
| 1 — Isolate REPORT | `|R|` | DEBUG |
| 2 — Report share | `Σ REPORT ms`, `Σ non-ISSUE_SYNC ms`; resulting `reportShare` % | DEBUG sums, INFO % |
| 2b — Demand profile | `observedHours`, per-bin counts summary, `avgDemandPerHour`, `peakDemandPerHour` (p95), `busiestHourCount` (abs max) | DEBUG detail, INFO headline (avg + peak) |
| 3 — Category shares | per category: count, `Σ ms`, `categoryShare_c` %; observed XXXL max | DEBUG |
| 4 — Representative cost (mean) | per category: the mean `representativeSec_c`, and whether the 0.1s floor was applied | DEBUG |
| 5 — Per-worker capacity | per `n`: `capacityPerHourSec(n)`, `reportCapacitySec(n)`, per-category `catCapacitySec_c(n)` | DEBUG |
| 5 — Results | per `n`: capacity total + all eight per-category job counts (empties `no tasks`); the verdict line | INFO |

- The demand INFO line and verdict line must name their basis ("/hour", "peak") so figures are
  unambiguous out of context.
- Skip cases (no REPORT / zero compute) and the demand-insufficient case each emit a single clear
  message; never an error.
- DEBUG lines require root `-v`; INFO appears at default level (unchanged convention).

### Error handling
- `--estimate-workers` values `< 1` → usage error before analysis (unchanged).
- Empty / non-REPORT data → skip-with-log, not an error (unchanged).
- Short data window → demand peak omitted with a log line, never an error.
- The estimate must never abort report generation; it runs after the report is written (unchanged).

### Constants (`estimate.go`)
- `costFloorSec = 0.1` (renamed from `modeFloorSec`)
- `secondsPerHour = 3600`
- `minObservedHours = 24` (NEW — peak-demand sufficiency threshold)
- `demandPeakPercentile = 95` (NEW)
- **Removed:** `modeRoundingSec`, `estimateMargin`.

### Open questions / notes (experimental)
- **Peak statistic.** p95 of hourly counts is the chosen "peak", trading off single-hour outliers
  vs. responsiveness. The percentile and the hour granularity are the most likely tuning knobs;
  both are named constants, not flags (yet).
- **Hour vs. day bins.** Clock-hour bins are used; for very long, smooth workloads a daily view
  could be added later, but hour is the operationally relevant grain for CE backlog.
- **Per-category demand** is intentionally out of scope (too sparse to be meaningful per
  category-hour).
- The per-hour basis assumes workers run continuously; a per-day figure is `× 24` if ever useful.

