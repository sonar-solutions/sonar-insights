package bgtasks

import (
	"fmt"
	"math"
	"sort"
)

const (
	modeRoundingSec = 0.1
	modeFloorSec    = 0.1
	estimateMargin  = 0.20
	secondsPerHour  = 3600
)

// CategoryEstimate holds the capacity estimate for one size category.
type CategoryEstimate struct {
	Label             string
	UpperBoundSec     int
	Share             float64
	RepresentativeSec float64
	FloorApplied      bool
	BucketCount       int
	BucketSumMs       int64
	CatCapacitySec    float64
	Jobs              float64
	JobsLow           float64
	JobsHigh          float64
}

// WorkerEstimate holds the capacity estimate for one worker count.
type WorkerEstimate struct {
	Workers    int
	Categories []CategoryEstimate
	TotalJobs  float64
	TotalLow   float64
	TotalHigh  float64
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
	MarginPct       float64
	PerHour         bool
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

	// Step 3 & 4: categorise and compute representative costs
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
		MarginPct:       estimateMargin,
		PerHour:         true,
		WorkerEstimates: workerEstimates,
	}, false, ""
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
			upperBound = (maxObservedMs + 999) / 1000
			lo := timeThresholds[i-1].upperBoundSec
			label = fmt.Sprintf("%s (%d-%ds)", name, lo, upperBound)
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

// representativeCost returns the mode (rounded to modeRoundingSec) of execution
// times in the bucket, clamped to modeFloorSec. Returns (0, false) for empty buckets.
func representativeCost(tasks []BgTask) (float64, bool) {
	if len(tasks) == 0 {
		return 0, false
	}
	freq := make(map[int64]int, len(tasks))
	for _, t := range tasks {
		rounded := roundToGranularity(float64(t.ExecutionTimeMs)/1000.0, modeRoundingSec)
		freq[rounded]++
	}

	var modeKey int64
	modeCount := -1
	for k, c := range freq {
		if c > modeCount || (c == modeCount && k < modeKey) {
			modeCount = c
			modeKey = k
		}
	}
	result := float64(modeKey) * modeRoundingSec
	if result < modeFloorSec {
		return modeFloorSec, true
	}
	return result, false
}

// roundToGranularity rounds sec to the nearest granularity step and returns
// the step index (i.e. result / granularity as an integer).
func roundToGranularity(sec, granularity float64) int64 {
	return int64(math.Round(sec / granularity))
}

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
		cat.JobsLow = jobs * (1 - estimateMargin)
		cat.JobsHigh = jobs * (1 + estimateMargin)
		updated[i] = cat
		totalJobs += jobs
	}

	return WorkerEstimate{
		Workers:    n,
		Categories: updated,
		TotalJobs:  totalJobs,
		TotalLow:   totalJobs * (1 - estimateMargin),
		TotalHigh:  totalJobs * (1 + estimateMargin),
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
