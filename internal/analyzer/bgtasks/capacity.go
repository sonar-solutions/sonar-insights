package bgtasks

import (
	"math"
	"sort"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/mathutil"
)

const bucketLengthMin = 5

var percentileLevels = []float64{0.85, 0.90, 0.95, 0.99, 0.995, 0.999, 1.0}

// BucketKey identifies a 5-minute UTC-aligned time bucket.
type BucketKey struct {
	Start time.Time
	End   time.Time
}

// PercentileResult holds the demand and single-worker utilisation at one percentile level.
type PercentileResult struct {
	PercentileMs            float64
	SingleWorkerUtilization float64
}

// PercentileAnalysis holds results for one slice of buckets (e.g. all vs. weekday-only).
type PercentileAnalysis struct {
	Label       string
	BucketCount int
	Percentiles map[float64]PercentileResult
}

// BusyCluster is a contiguous run of buckets above the 99.9th-percentile threshold.
type BusyCluster struct {
	Start           time.Time
	End             time.Time
	BucketCount     int
	DurationMinutes int
}

// CapacityDemandResults is the output of the capacity demand analysis.
type CapacityDemandResults struct {
	BucketLengthMinutes  int
	DemandPerBucket      map[BucketKey]int
	PercentileAnalyses   []PercentileAnalysis
	Percentiles          map[float64]PercentileResult // last-written analysis (24/7 then weekday)
	BusyClusters         []BusyCluster
	BusyBucketsCount     int
	BusyBucketsThreshold float64
	AnalysisWindowStart  time.Time
	AnalysisWindowEnd    time.Time
}

// CalculateCapacityDemand mirrors CapacityDemandCalculator.Calculate from the C# reference.
func CalculateCapacityDemand(tasks []BgTask, start, end time.Time) CapacityDemandResults {
	results := CapacityDemandResults{
		BucketLengthMinutes: bucketLengthMin,
		DemandPerBucket:     make(map[BucketKey]int),
		Percentiles:         make(map[float64]PercentileResult),
	}

	preGenerateBuckets(&results, start, end)
	populateDemand(&results, tasks)

	calcPercentiles(&results, func(k BucketKey) bool { return true }, "All Buckets (24/7 Coverage)")
	calcPercentiles(&results, func(k BucketKey) bool {
		wd := k.Start.Weekday()
		return wd != time.Saturday && wd != time.Sunday
	}, "Weekday Buckets Only (Monday-Friday)")

	if p999, ok := results.Percentiles[0.999]; ok {
		detectClusters(&results, p999.PercentileMs)
	}

	return results
}

func preGenerateBuckets(results *CapacityDemandResults, timelineStart, timelineEnd time.Time) {
	cur := bucketStart(timelineStart)
	results.AnalysisWindowStart = cur
	for cur.Before(timelineEnd) {
		next := cur.Add(time.Duration(bucketLengthMin) * time.Minute)
		results.DemandPerBucket[BucketKey{Start: cur, End: next}] = 0
		cur = next
	}
	results.AnalysisWindowEnd = cur
}

func populateDemand(results *CapacityDemandResults, tasks []BgTask) {
	for _, t := range tasks {
		execStart := t.SubmittedAt
		execEnd := t.SubmittedAt.Add(time.Duration(t.ExecutionTimeMs) * time.Millisecond)
		cur := bucketStart(execStart)
		for cur.Before(execEnd) {
			next := cur.Add(time.Duration(bucketLengthMin) * time.Minute)
			overlapStart := maxTime(execStart, cur)
			overlapEnd := minTime(execEnd, next)
			overlapMs := int(overlapEnd.Sub(overlapStart).Milliseconds())
			key := BucketKey{Start: cur, End: next}
			results.DemandPerBucket[key] += overlapMs
			cur = next
		}
	}
}

func calcPercentiles(results *CapacityDemandResults, filter func(BucketKey) bool, label string) {
	var demands []int
	for k, v := range results.DemandPerBucket {
		if filter(k) {
			demands = append(demands, v)
		}
	}
	if len(demands) == 0 {
		return
	}

	bucketMs := float64(bucketLengthMin) * 60 * 1000
	percentiles := make(map[float64]PercentileResult, len(percentileLevels))
	for _, p := range percentileLevels {
		pMs := mathutil.CalculatePercentile(demands, p)
		util := pMs / bucketMs * 100
		percentiles[p] = PercentileResult{PercentileMs: pMs, SingleWorkerUtilization: util}
	}

	results.PercentileAnalyses = append(results.PercentileAnalyses, PercentileAnalysis{
		Label:       label,
		BucketCount: len(demands),
		Percentiles: percentiles,
	})
	for p, pr := range percentiles {
		results.Percentiles[p] = pr
	}
}

type bucketDemand struct {
	key BucketKey
	val int
}

func detectClusters(results *CapacityDemandResults, threshold float64) {
	results.BusyBucketsThreshold = threshold

	var busy []bucketDemand
	for k, v := range results.DemandPerBucket {
		if float64(v) > threshold {
			busy = append(busy, bucketDemand{k, v})
		}
	}
	sort.Slice(busy, func(i, j int) bool {
		return busy[i].key.Start.Before(busy[j].key.Start)
	})
	results.BusyBucketsCount = len(busy)

	if len(busy) == 0 {
		return
	}

	results.BusyClusters = buildClusters(busy, bucketLengthMin)
}

func buildClusters(busy []bucketDemand, bucketMin int) []BusyCluster {
	var clusters []BusyCluster
	clusterStart := busy[0].key.Start
	count := 1
	for i := 1; i < len(busy); i++ {
		if busy[i].key.Start.Equal(busy[i-1].key.End) {
			count++
		} else {
			clusters = append(clusters, BusyCluster{
				Start:           clusterStart,
				End:             busy[i-1].key.End,
				BucketCount:     count,
				DurationMinutes: count * bucketMin,
			})
			clusterStart = busy[i].key.Start
			count = 1
		}
	}
	clusters = append(clusters, BusyCluster{
		Start:           clusterStart,
		End:             busy[len(busy)-1].key.End,
		BucketCount:     count,
		DurationMinutes: count * bucketMin,
	})
	return clusters
}

func bucketStart(t time.Time) time.Time {
	t = t.UTC()
	totalMin := t.Hour()*60 + t.Minute()
	idx := totalMin / bucketLengthMin
	startMin := idx * bucketLengthMin
	return time.Date(t.Year(), t.Month(), t.Day(), startMin/60, startMin%60, 0, 0, time.UTC)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// WorkersNeeded returns the ceiling of utilisation/100.
func WorkersNeeded(utilizationPct float64) int {
	return int(math.Ceil(utilizationPct / 100.0))
}
