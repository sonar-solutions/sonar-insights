package bgtasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- findPercentileAnalyses ---

func TestFindPercentileAnalyses_BothPresent(t *testing.T) {
	analyses := []PercentileAnalysis{
		{Label: "All Buckets (24/7)"},
		{Label: "Weekday Buckets Only"},
	}
	allB, wd := findPercentileAnalyses(analyses)
	if allB == nil {
		t.Error("expected allBuckets to be found")
	}
	if wd == nil {
		t.Error("expected weekday to be found")
	}
}

func TestFindPercentileAnalyses_OnlyAllBuckets(t *testing.T) {
	analyses := []PercentileAnalysis{{Label: "All Buckets (24/7)"}}
	allB, wd := findPercentileAnalyses(analyses)
	if allB == nil {
		t.Error("expected allBuckets to be found")
	}
	if wd != nil {
		t.Error("expected weekday to be nil")
	}
}

func TestFindPercentileAnalyses_NeitherPresent(t *testing.T) {
	analyses := []PercentileAnalysis{{Label: "Something Else"}}
	allB, wd := findPercentileAnalyses(analyses)
	if allB != nil || wd != nil {
		t.Error("expected both to be nil for unrecognised labels")
	}
}

// --- extractAtPercentile ---

func TestExtractAtPercentile_NilAnalysis(t *testing.T) {
	pr, count := extractAtPercentile(nil, 0.99)
	if pr != nil || count != 0 {
		t.Errorf("nil analysis: got pr=%v count=%d, want nil,0", pr, count)
	}
}

func TestExtractAtPercentile_PercentileMissing(t *testing.T) {
	a := &PercentileAnalysis{
		BucketCount: 10,
		Percentiles: map[float64]PercentileResult{0.90: {PercentileMs: 1000}},
	}
	pr, count := extractAtPercentile(a, 0.99)
	if pr != nil || count != 0 {
		t.Errorf("missing percentile: got pr=%v count=%d, want nil,0", pr, count)
	}
}

func TestExtractAtPercentile_Found(t *testing.T) {
	a := &PercentileAnalysis{
		BucketCount: 42,
		Percentiles: map[float64]PercentileResult{
			0.99: {PercentileMs: 5000, SingleWorkerUtilization: 16.7},
		},
	}
	pr, count := extractAtPercentile(a, 0.99)
	if pr == nil {
		t.Fatal("expected PercentileResult, got nil")
	}
	if count != 42 {
		t.Errorf("bucket count=%d, want 42", count)
	}
	if pr.SingleWorkerUtilization != 16.7 {
		t.Errorf("utilization=%.1f, want 16.7", pr.SingleWorkerUtilization)
	}
}

// --- computeRecommendation ---

// makeAnalysis builds a PercentileAnalysis with a single percentile level at the given utilization.
func makeAnalysis(label string, util float64, bucketCount int) PercentileAnalysis {
	return PercentileAnalysis{
		Label:       label,
		BucketCount: bucketCount,
		Percentiles: map[float64]PercentileResult{
			recommendationPercentile: {
				PercentileMs:            float64(bucketLengthMin) * 60 * 1000 * util / 100,
				SingleWorkerUtilization: util,
			},
		},
	}
}

func TestComputeRecommendation_AllBucketsOnly(t *testing.T) {
	cd := CapacityDemandResults{
		BucketLengthMinutes: bucketLengthMin,
		PercentileAnalyses:  []PercentileAnalysis{makeAnalysis("All Buckets (24/7)", 60.0, 100)},
	}
	rec := computeRecommendation(cd)
	if rec == nil {
		t.Fatal("expected recommendation, got nil")
	}
	if rec.workersNeeded != 1 {
		t.Errorf("workersNeeded=%d, want 1", rec.workersNeeded)
	}
	if rec.source != "All Buckets (24/7)" {
		t.Errorf("source=%q, want %q", rec.source, "All Buckets (24/7)")
	}
}

func TestComputeRecommendation_WeekdayWins(t *testing.T) {
	// Weekday has higher utilization than all-buckets → should be selected.
	cd := CapacityDemandResults{
		BucketLengthMinutes: bucketLengthMin,
		PercentileAnalyses: []PercentileAnalysis{
			makeAnalysis("All Buckets (24/7)", 60.0, 100),
			makeAnalysis("Weekday Buckets Only", 150.0, 50), // >100% → 2 workers
		},
	}
	rec := computeRecommendation(cd)
	if rec == nil {
		t.Fatal("expected recommendation, got nil")
	}
	if rec.workersNeeded != 2 {
		t.Errorf("workersNeeded=%d, want 2", rec.workersNeeded)
	}
	if rec.source != "Weekday Buckets" {
		t.Errorf("source=%q, want %q", rec.source, "Weekday Buckets")
	}
}

func TestComputeRecommendation_AllBucketsWins(t *testing.T) {
	// All-buckets has higher utilization than weekday → should be selected.
	cd := CapacityDemandResults{
		BucketLengthMinutes: bucketLengthMin,
		PercentileAnalyses: []PercentileAnalysis{
			makeAnalysis("All Buckets (24/7)", 150.0, 100), // >100% → 2 workers
			makeAnalysis("Weekday Buckets Only", 60.0, 50),
		},
	}
	rec := computeRecommendation(cd)
	if rec == nil {
		t.Fatal("expected recommendation, got nil")
	}
	if rec.workersNeeded != 2 {
		t.Errorf("workersNeeded=%d, want 2", rec.workersNeeded)
	}
	if rec.source != "All Buckets (24/7)" {
		t.Errorf("source=%q, want %q", rec.source, "All Buckets (24/7)")
	}
}

func TestComputeRecommendation_NoData(t *testing.T) {
	cd := CapacityDemandResults{PercentileAnalyses: nil}
	if rec := computeRecommendation(cd); rec != nil {
		t.Errorf("expected nil recommendation for empty analyses, got %+v", rec)
	}
}

// --- buildCapacityHTML ---

func TestBuildCapacityHTML_Empty(t *testing.T) {
	html := buildCapacityHTML(CapacityDemandResults{})
	if !strings.Contains(html, "No capacity demand analysis data available") {
		t.Errorf("expected no-data message, got: %s", html)
	}
}

// --- ordinal ---

func TestOrdinal(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1, "First"},
		{2, "Second"},
		{3, "Third"},
		{4, "4th"},
		{10, "10th"},
	}
	for _, tc := range cases {
		if got := ordinal(tc.n); got != tc.want {
			t.Errorf("ordinal(%d)=%q, want %q", tc.n, got, tc.want)
		}
	}
}

// --- workerSuffix ---

func TestWorkerSuffix(t *testing.T) {
	if got := workerSuffix(1); got != "worker" {
		t.Errorf("workerSuffix(1)=%q, want %q", got, "worker")
	}
	if got := workerSuffix(2); got != "workers" {
		t.Errorf("workerSuffix(2)=%q, want %q", got, "workers")
	}
}

// --- BuildReport maps AnalysisResults fields to the rendered HTML ---

// reportFixture returns an AnalysisResults with distinctive sentinel values that
// are easy to locate in the rendered HTML, verifying the field→report mapping.
func reportFixture() AnalysisResults {
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	return AnalysisResults{
		DateRange: DateRange{
			EarliestSubmission: day,
			LatestCompletion:   end,
			DateRangeInDays:    73,
		},
		Overall: OverallMetrics{
			TotalUniqueTasks:      742,
			AverageTasksPerDay:    63.4,
			BusiestDayTaskCount:   487,
			FailedTasksPercentage: 1.2,
		},
		ProjectAnalysis: ProjectAnalysisMetrics{
			TotalProjectAnalysisTasks:     321,
			TotalProjectsAnalyzed:         17,
			AverageProjectAnalysisTasks:   4.4,
			TotalBranchAnalysisTasks:      311,
			TotalPullRequestAnalysisTasks: 10,
			BranchAndPrExecutionTimesSec:  map[string]float64{"Branch": 30.0, "PR": 15.0},
			TopProjects:                   []TopProject{{Key: "project-alpha", Count: 88, Percentage: 27.4}},
		},
		Charts: ChartDatasets{
			TasksPerDay:            map[time.Time]int{day: 10},
			ProjectAnalysesPerDay:  map[time.Time]int{day: 5},
			TasksPerDayByType:      map[time.Time]map[string]int{day: {"REPORT": 5}},
			AllTaskTypes:           []string{"REPORT"},
			SummaryByPendingTime:   []TimeCategoryMetric{{Label: "0-1s", Count: 1, Percentage: 100}},
			SummaryByExecutionTime: []TimeCategoryMetric{{Label: "0-1s", Count: 1, Percentage: 100}},
			SummaryByType:          map[string]int{"REPORT": 321},
			SummaryByStatus:        map[string]int{"SUCCESS": 742},
		},
		CapacityDemand: CapacityDemandResults{
			BucketLengthMinutes: bucketLengthMin,
			DemandPerBucket:     map[BucketKey]int{},
			PercentileAnalyses: []PercentileAnalysis{
				makeAnalysis("All Buckets (24/7)", 60.0, 100), // 60% → 1 worker recommended
			},
			AnalysisWindowStart: day,
			AnalysisWindowEnd:   end,
		},
	}
}

func TestBuildReport_MapsValuesToHTML(t *testing.T) {
	results := reportFixture()
	report := BuildReport(results)

	path := filepath.Join(t.TempDir(), "report.html")
	if err := RenderToFile(report, path); err != nil {
		t.Fatalf("render: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rendered file: %v", err)
	}
	html := string(data)

	checks := []struct {
		field string
		want  string
	}{
		{"DateRangeInDays", "73"},
		{"TotalUniqueTasks", "742"},
		{"AverageTasksPerDay", "63.4"},
		{"BusiestDayTaskCount", "487"},
		{"TotalProjectsAnalyzed", "17"},
		{"TotalProjectAnalysisTasks", "321"},
		{"worker recommendation", "1 Compute Engine worker"},
	}
	for _, c := range checks {
		if !strings.Contains(html, c.want) {
			t.Errorf("%s: rendered HTML does not contain %q", c.field, c.want)
		}
	}
}
