package bgtasks

import (
	"math"
	"os"
	"testing"
	"time"
)

func ts(year, month, day, hour, min int) time.Time {
	return time.Date(year, time.Month(month), day, hour, min, 0, 0, time.UTC)
}

func TestBucketStart_AlignedToFiveMinutes(t *testing.T) {
	cases := []struct {
		input time.Time
		want  time.Time
	}{
		{ts(2026, 1, 1, 10, 0), ts(2026, 1, 1, 10, 0)},
		{ts(2026, 1, 1, 10, 3), ts(2026, 1, 1, 10, 0)},
		{ts(2026, 1, 1, 10, 5), ts(2026, 1, 1, 10, 5)},
		{ts(2026, 1, 1, 10, 7), ts(2026, 1, 1, 10, 5)},
		{ts(2026, 1, 1, 10, 59), ts(2026, 1, 1, 10, 55)},
		{ts(2026, 1, 1, 23, 58), ts(2026, 1, 1, 23, 55)},
	}
	for _, tc := range cases {
		got := bucketStart(tc.input)
		if !got.Equal(tc.want) {
			t.Errorf("bucketStart(%v) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestPreGenerateBuckets_Count(t *testing.T) {
	start := ts(2026, 1, 1, 0, 0)
	end := ts(2026, 1, 1, 1, 0) // 1 hour = 12 buckets of 5 min
	results := CapacityDemandResults{
		DemandPerBucket: make(map[BucketKey]int),
		Percentiles:     make(map[float64]PercentileResult),
	}
	preGenerateBuckets(&results, start, end)
	if len(results.DemandPerBucket) != 12 {
		t.Errorf("bucket count=%d, want 12", len(results.DemandPerBucket))
	}
	if !results.AnalysisWindowStart.Equal(start) {
		t.Errorf("AnalysisWindowStart=%v, want %v", results.AnalysisWindowStart, start)
	}
	if !results.AnalysisWindowEnd.Equal(end) {
		t.Errorf("AnalysisWindowEnd=%v, want %v", results.AnalysisWindowEnd, end)
	}
}

func TestPopulateDemand_SingleBucket(t *testing.T) {
	// Task submitted at 10:00, runs for 1 minute — all demand in one bucket.
	start := ts(2026, 1, 1, 10, 0)
	end := ts(2026, 1, 1, 10, 5)
	results := CapacityDemandResults{
		DemandPerBucket: map[BucketKey]int{
			{Start: start, End: end}: 0,
		},
		Percentiles: make(map[float64]PercentileResult),
	}
	task := BgTask{
		SubmittedAt:     start,
		ExecutionTimeMs: 60_000, // 1 minute
	}
	populateDemand(&results, []BgTask{task})

	key := BucketKey{Start: start, End: end}
	if results.DemandPerBucket[key] != 60_000 {
		t.Errorf("demand=%d, want 60000", results.DemandPerBucket[key])
	}
}

func TestPopulateDemand_SpansMultipleBuckets(t *testing.T) {
	// Task submitted at 10:03, runs for 6 minutes → spans two 5-min buckets.
	// Bucket 1: 10:00–10:05, overlap = 2 min (10:03–10:05) = 120_000 ms
	// Bucket 2: 10:05–10:10, overlap = 4 min (10:05–10:09) = 240_000 ms
	b1s, b1e := ts(2026, 1, 1, 10, 0), ts(2026, 1, 1, 10, 5)
	b2s, b2e := ts(2026, 1, 1, 10, 5), ts(2026, 1, 1, 10, 10)
	results := CapacityDemandResults{
		DemandPerBucket: map[BucketKey]int{
			{Start: b1s, End: b1e}: 0,
			{Start: b2s, End: b2e}: 0,
		},
		Percentiles: make(map[float64]PercentileResult),
	}
	submittedAt := ts(2026, 1, 1, 10, 3)
	task := BgTask{SubmittedAt: submittedAt, ExecutionTimeMs: 6 * 60 * 1000}
	populateDemand(&results, []BgTask{task})

	got1 := results.DemandPerBucket[BucketKey{Start: b1s, End: b1e}]
	got2 := results.DemandPerBucket[BucketKey{Start: b2s, End: b2e}]
	if got1 != 2*60*1000 {
		t.Errorf("bucket1 demand=%d, want %d", got1, 2*60*1000)
	}
	if got2 != 4*60*1000 {
		t.Errorf("bucket2 demand=%d, want %d", got2, 4*60*1000)
	}
}

func TestCalculateCapacityDemand_Percentiles(t *testing.T) {
	// One task filling one bucket completely (300s = 5 min bucket).
	start := ts(2026, 1, 1, 10, 0)
	end := ts(2026, 1, 1, 10, 5)
	task := BgTask{SubmittedAt: start, ExecutionTimeMs: 5 * 60 * 1000}
	results := CalculateCapacityDemand([]BgTask{task}, start, end)

	// The one non-zero bucket should have 100% utilisation.
	if len(results.PercentileAnalyses) == 0 {
		t.Fatal("no percentile analyses produced")
	}
	p100, ok := results.PercentileAnalyses[0].Percentiles[1.0]
	if !ok {
		t.Fatal("missing 100th percentile")
	}
	wantUtil := 100.0
	if math.Abs(p100.SingleWorkerUtilization-wantUtil) > 0.01 {
		t.Errorf("100th percentile utilization=%.2f, want %.2f", p100.SingleWorkerUtilization, wantUtil)
	}
}

func TestDetectClusters_ConsecutiveBuckets(t *testing.T) {
	b1s, b1e := ts(2026, 1, 1, 10, 0), ts(2026, 1, 1, 10, 5)
	b2s, b2e := ts(2026, 1, 1, 10, 5), ts(2026, 1, 1, 10, 10)
	b3s, b3e := ts(2026, 1, 1, 10, 15), ts(2026, 1, 1, 10, 20) // gap after b2
	results := CapacityDemandResults{
		DemandPerBucket: map[BucketKey]int{
			{Start: b1s, End: b1e}: 100,
			{Start: b2s, End: b2e}: 100,
			{Start: b3s, End: b3e}: 100,
		},
		Percentiles: make(map[float64]PercentileResult),
	}
	detectClusters(&results, 50) // threshold=50, all three are busy

	if results.BusyBucketsCount != 3 {
		t.Errorf("BusyBucketsCount=%d, want 3", results.BusyBucketsCount)
	}
	// b1+b2 are consecutive → 1 cluster; b3 is separated → 1 cluster
	if len(results.BusyClusters) != 2 {
		t.Errorf("cluster count=%d, want 2", len(results.BusyClusters))
	}
	if results.BusyClusters[0].BucketCount != 2 {
		t.Errorf("first cluster BucketCount=%d, want 2", results.BusyClusters[0].BucketCount)
	}
	if results.BusyClusters[0].DurationMinutes != 10 {
		t.Errorf("first cluster DurationMinutes=%d, want 10", results.BusyClusters[0].DurationMinutes)
	}
}

func TestDetectClusters_NoBusyBuckets(t *testing.T) {
	b1s, b1e := ts(2026, 1, 1, 10, 0), ts(2026, 1, 1, 10, 5)
	results := CapacityDemandResults{
		DemandPerBucket: map[BucketKey]int{
			{Start: b1s, End: b1e}: 10,
		},
		Percentiles: make(map[float64]PercentileResult),
	}
	detectClusters(&results, 50) // threshold higher than any demand

	if results.BusyBucketsCount != 0 {
		t.Errorf("BusyBucketsCount=%d, want 0", results.BusyBucketsCount)
	}
	if len(results.BusyClusters) != 0 {
		t.Errorf("cluster count=%d, want 0", len(results.BusyClusters))
	}
}

func TestCalculateCapacityDemand_GoldenMaster(t *testing.T) {
	dir := "/Users/lukas/repos/sonar-insights-cs/test-data"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skip("C# test data not available")
	}
	tasks, err := Load(dir, nopLogger)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Exclude ISSUE_SYNC tasks as the analyzer does.
	var filtered []BgTask
	for _, t := range tasks {
		if t.Type != typeIssueSync {
			filtered = append(filtered, t)
		}
	}

	dr := AnalyzeDateRange(tasks)
	cap := CalculateCapacityDemand(filtered, dr.EarliestSubmission, dr.LatestCompletion)

	// Golden master: TotalBuckets=14138, BusyBucketsCount=15
	// BusyBucketsCount uses the 24/7 99.9th-percentile threshold against all buckets,
	// so weekend buckets above that threshold are correctly included.
	if len(cap.DemandPerBucket) != 14138 {
		t.Errorf("TotalBuckets=%d, want 14138", len(cap.DemandPerBucket))
	}
	if cap.BusyBucketsCount != 15 {
		t.Errorf("BusyBucketsCount=%d, want 15", cap.BusyBucketsCount)
	}
}

func TestWorkerRecommendation(t *testing.T) {
	cases := []struct {
		util int
		want int
	}{
		{0, 0},
		{1, 1},
		{100, 1},
		{101, 2},
		{200, 2},
		{201, 3},
	}
	for _, tc := range cases {
		got := WorkersNeeded(float64(tc.util))
		if got != tc.want {
			t.Errorf("WorkersNeeded(%d)=%d, want %d", tc.util, got, tc.want)
		}
	}
}
