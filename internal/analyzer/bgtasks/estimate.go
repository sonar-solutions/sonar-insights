package bgtasks

import (
	"fmt"
	"sort"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/mathutil"
)

const (
	costFloorSec         = 0.1
	secondsPerHour       = 3600
	minObservedHours     = 24
	demandPeakPercentile = 95
)

// CategoryEstimate holds the capacity estimate for one size category.
type CategoryEstimate struct {
	Label             string
	UpperBoundSec     int
	Share             float64
	RepresentativeSec float64 // mean, clamped to costFloorSec
	FloorApplied      bool
	BucketCount       int
	BucketSumMs       int64
	CatCapacitySec    float64
	Jobs              float64 // jobs/hour
}

// WorkerEstimate holds the capacity estimate for one worker count.
type WorkerEstimate struct {
	Workers    int
	Categories []CategoryEstimate
	TotalJobs  float64 // capacity jobs/hour
}

// DemandProfile describes how REPORT analyses arrive over time.
type DemandProfile struct {
	Available        bool // false when observedHours < minObservedHours
	ObservedHours    int
	AvgPerHour       float64
	PeakPerHour      float64 // p95 of hourly counts (zero-filled)
	BusiestHourCount int     // absolute max hourly count
}

// CapacityEstimate is the top-level result of the capacity estimation.
type CapacityEstimate struct {
	ReportShare     float64
	TotalNonSyncMs  int64
	TotalReportMs   int64
	ExcludedCount   int
	NonSyncCount    int
	ReportCount     int
	MaxObservedMs   int
	PerHour         bool
	Demand          DemandProfile
	WorkerEstimates []WorkerEstimate
}

// categoryNames maps time threshold index to size category name.
var categoryNames = []string{"XXS", "XS", "S", "M", "L", "XL", "XXL", "XXXL"}

// EstimateCapacity computes the capacity estimate for the given worker counts.
// tasks is the post-load, post-date-filter task slice.
// Returns (estimate, skipped, reason) where skipped is true when there is
// insufficient data to compute the estimate.
func EstimateCapacity(tasks []BgTask, workerCounts []int) (CapacityEstimate, bool, string) {
	// Step 0: exclude ISSUE_SYNC
	excluded := countType(tasks, typeIssueSync)
	nonSync := excludeIssueSync(tasks)

	// Step 1: isolate REPORT tasks
	reportTasks := filterReport(nonSync)

	// Guard: empty REPORT set or zero total compute
	if len(reportTasks) == 0 {
		return CapacityEstimate{
			ExcludedCount: excluded,
			NonSyncCount:  len(nonSync),
		}, true, "no REPORT tasks found — skipping capacity estimate"
	}
	totalNonSyncMs := sumMs(nonSync)
	if totalNonSyncMs == 0 {
		return CapacityEstimate{
			ExcludedCount: excluded,
			NonSyncCount:  len(nonSync),
			ReportCount:   len(reportTasks),
		}, true, "total non-ISSUE_SYNC execution time is zero — skipping capacity estimate"
	}

	// Step 2: report share
	totalReportMs := sumMs(reportTasks)
	reportShare := float64(totalReportMs) / float64(totalNonSyncMs)
	maxObservedMs := maxExecutionMs(reportTasks)

	// Step 2b: demand profile
	demand := computeDemandProfile(reportTasks)

	// Step 3 & 4: categorise and compute representative costs (mean)
	buckets := bucketReportTasks(reportTasks)
	categories := buildCategoryEstimates(buckets, totalReportMs, maxObservedMs)

	// Step 5: per-worker estimates
	sorted := sortedDedup(workerCounts)
	workerEstimates := make([]WorkerEstimate, 0, len(sorted))
	for _, n := range sorted {
		we := computeWorkerEstimate(n, reportShare, categories)
		workerEstimates = append(workerEstimates, we)
	}

	return CapacityEstimate{
		ReportShare:     reportShare,
		TotalNonSyncMs:  totalNonSyncMs,
		TotalReportMs:   totalReportMs,
		ExcludedCount:   excluded,
		NonSyncCount:    len(nonSync),
		ReportCount:     len(reportTasks),
		MaxObservedMs:   maxObservedMs,
		PerHour:         true,
		Demand:          demand,
		WorkerEstimates: workerEstimates,
	}, false, ""
}

// computeDemandProfile measures REPORT arrival rate using SubmittedAt.
// Tasks are bucketed into UTC clock-hour bins; hours with no arrivals are
// zero-filled so the average reflects real calendar time.
func computeDemandProfile(reportTasks []BgTask) DemandProfile {
	hourCounts := make(map[time.Time]int)
	var earliest, latest time.Time
	for i, t := range reportTasks {
		hour := t.SubmittedAt.UTC().Truncate(time.Hour)
		hourCounts[hour]++
		if i == 0 || t.SubmittedAt.Before(earliest) {
			earliest = t.SubmittedAt
		}
		if i == 0 || t.SubmittedAt.After(latest) {
			latest = t.SubmittedAt
		}
	}

	earliestHour := earliest.UTC().Truncate(time.Hour)
	latestHour := latest.UTC().Truncate(time.Hour)
	observedHours := int(latestHour.Sub(earliestHour).Hours()) + 1

	counts := make([]int, 0, observedHours)
	busiestHourCount := 0
	for h := earliestHour; !h.After(latestHour); h = h.Add(time.Hour) {
		c := hourCounts[h]
		counts = append(counts, c)
		if c > busiestHourCount {
			busiestHourCount = c
		}
	}

	avgPerHour := float64(len(reportTasks)) / float64(observedHours)
	peakPerHour := mathutil.CalculatePercentile(counts, float64(demandPeakPercentile)/100.0)

	return DemandProfile{
		Available:        observedHours >= minObservedHours,
		ObservedHours:    observedHours,
		AvgPerHour:       avgPerHour,
		PeakPerHour:      peakPerHour,
		BusiestHourCount: busiestHourCount,
	}
}

// VerdictFor returns the plain-language verdict comparing capacityPerHour jobs/hour to d.
func VerdictFor(capacityPerHour float64, d DemandProfile) string {
	avg := d.AvgPerHour
	if !d.Available {
		return fmt.Sprintf("average demand ≈ %.0f/hr; insufficient time span to estimate peak-hour load", avg)
	}
	peak := d.PeakPerHour
	if capacityPerHour >= peak {
		return fmt.Sprintf("covers your peak hour (≈ %.0f/hr) with ≈ %.0f/hr to spare", peak, capacityPerHour-peak)
	}
	if capacityPerHour >= avg {
		return fmt.Sprintf("keeps up with average demand (≈ %.0f/hr) but during peak hours (≈ %.0f/hr) ≈ %.0f/hr would queue and drain in quieter periods", avg, peak, peak-capacityPerHour)
	}
	return fmt.Sprintf("below your average demand (≈ %.0f/hr) — sustained backlog likely; consider more workers", avg)
}

func countType(tasks []BgTask, taskType string) int {
	n := 0
	for _, t := range tasks {
		if t.Type == taskType {
			n++
		}
	}
	return n
}

func excludeIssueSync(tasks []BgTask) []BgTask {
	out := make([]BgTask, 0, len(tasks))
	for _, t := range tasks {
		if t.Type != typeIssueSync {
			out = append(out, t)
		}
	}
	return out
}

func filterReport(tasks []BgTask) []BgTask {
	out := make([]BgTask, 0, len(tasks))
	for _, t := range tasks {
		if t.Type == typeReport {
			out = append(out, t)
		}
	}
	return out
}

func sumMs(tasks []BgTask) int64 {
	var total int64
	for _, t := range tasks {
		total += int64(t.ExecutionTimeMs)
	}
	return total
}

func maxExecutionMs(tasks []BgTask) int {
	result := 0
	for _, t := range tasks {
		if t.ExecutionTimeMs > result {
			result = t.ExecutionTimeMs
		}
	}
	return result
}

// bucketReportTasks assigns each REPORT task to a time-threshold bucket.
func bucketReportTasks(tasks []BgTask) [][]BgTask {
	buckets := make([][]BgTask, len(timeThresholds))
	for _, t := range tasks {
		execSec := float64(t.ExecutionTimeMs) / 1000.0
		for i, thresh := range timeThresholds {
			var lower int
			if i > 0 {
				lower = timeThresholds[i-1].upperBoundSec
			}
			if inBucket(execSec, lower, thresh.upperBoundSec, i == 0) {
				buckets[i] = append(buckets[i], t)
				break
			}
		}
	}
	return buckets
}

func inBucket(sec float64, lower, upper int, inclusive bool) bool {
	lo, up := float64(lower), float64(upper)
	if inclusive {
		return sec >= lo && sec <= up
	}
	return sec > lo && sec <= up
}

func buildCategoryEstimates(buckets [][]BgTask, totalReportMs int64, maxObservedMs int) []CategoryEstimate {
	cats := make([]CategoryEstimate, len(timeThresholds))
	for i, thresh := range timeThresholds {
		name := categoryNames[i]
		label := fmt.Sprintf("%s (%s)", name, thresh.label)

		upperBound := thresh.upperBoundSec
		if i == len(timeThresholds)-1 {
			lo := timeThresholds[i-1].upperBoundSec
			upperBound = (maxObservedMs + 999) / 1000
			if upperBound <= lo {
				label = fmt.Sprintf("%s (> %ds)", name, lo)
			} else {
				label = fmt.Sprintf("%s (%d-%ds)", name, lo, upperBound)
			}
		}

		bucket := buckets[i]
		bucketMs := sumMs(bucket)
		var share float64
		if totalReportMs > 0 {
			share = float64(bucketMs) / float64(totalReportMs)
		}

		repSec, floorApplied := representativeCost(bucket)

		cats[i] = CategoryEstimate{
			Label:             label,
			UpperBoundSec:     upperBound,
			Share:             share,
			RepresentativeSec: repSec,
			FloorApplied:      floorApplied,
			BucketCount:       len(bucket),
			BucketSumMs:       bucketMs,
		}
	}
	return cats
}

// representativeCost returns the mean execution time of tasks (in seconds),
// clamped to costFloorSec. Returns (0, false) for empty buckets.
func representativeCost(tasks []BgTask) (float64, bool) {
	if len(tasks) == 0 {
		return 0, false
	}
	var totalMs int64
	for _, t := range tasks {
		totalMs += int64(t.ExecutionTimeMs)
	}
	mean := float64(totalMs) / float64(len(tasks)) / 1000.0
	if mean < costFloorSec {
		return costFloorSec, true
	}
	return mean, false
}

// computeWorkerEstimate computes capacity for n workers.
// Capacity chain: capacityPerHourSec(n) → reportCapacitySec(n) → catCapacitySec_c(n) → jobs_c(n).
func computeWorkerEstimate(n int, reportShare float64, cats []CategoryEstimate) WorkerEstimate {
	capacitySec := float64(n) * secondsPerHour
	reportCapacitySec := capacitySec * reportShare

	updated := make([]CategoryEstimate, len(cats))
	totalJobs := 0.0
	for i, c := range cats {
		catCapacitySec := reportCapacitySec * c.Share
		var jobs float64
		if c.RepresentativeSec > 0 {
			jobs = catCapacitySec / c.RepresentativeSec
		}
		cat := c
		cat.CatCapacitySec = catCapacitySec
		cat.Jobs = jobs
		updated[i] = cat
		totalJobs += jobs
	}

	return WorkerEstimate{
		Workers:    n,
		Categories: updated,
		TotalJobs:  totalJobs,
	}
}

func sortedDedup(vals []int) []int {
	seen := make(map[int]struct{}, len(vals))
	out := make([]int, 0, len(vals))
	for _, v := range vals {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	sort.Ints(out)
	return out
}
