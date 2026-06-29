package bgtasks

import (
	"math"
	"strings"
	"testing"
	"time"
)

func makeTask(taskType string, execMs int) BgTask {
	return BgTask{Type: taskType, ExecutionTimeMs: execMs}
}

func makeTaskAt(taskType string, execMs int, submittedAt time.Time) BgTask {
	return BgTask{Type: taskType, ExecutionTimeMs: execMs, SubmittedAt: submittedAt}
}

func TestEstimateCapacity_MeanCost(t *testing.T) {
	// XXS: 400ms + 600ms → mean = 500ms = 0.5s
	// XS:  1500ms + 2500ms → mean = 2000ms = 2.0s
	tasks := []BgTask{
		makeTask(typeReport, 400),
		makeTask(typeReport, 600),
		makeTask(typeReport, 1500),
		makeTask(typeReport, 2500),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{4})
	if skipped {
		t.Fatal("expected estimate to run, got skipped")
	}

	we := est.WorkerEstimates[0]
	xxs := we.Categories[0]
	xs := we.Categories[1]

	if math.Abs(xxs.RepresentativeSec-0.5) > 1e-9 {
		t.Errorf("XXS mean=%.6f, want 0.5", xxs.RepresentativeSec)
	}
	if math.Abs(xs.RepresentativeSec-2.0) > 1e-9 {
		t.Errorf("XS mean=%.6f, want 2.0", xs.RepresentativeSec)
	}

	// Verify jobs_c = catCapacitySec / repSec
	reportShare := est.ReportShare
	capacitySec := 4.0 * float64(secondsPerHour)
	reportCapSec := capacitySec * reportShare
	wantXXSShare := 1000.0 / 5000.0
	wantXSShare := 4000.0 / 5000.0
	wantXXSJobs := reportCapSec * wantXXSShare / 0.5
	wantXSJobs := reportCapSec * wantXSShare / 2.0

	if math.Abs(xxs.Jobs-wantXXSJobs) > 1e-6 {
		t.Errorf("XXS Jobs=%.6f, want %.6f", xxs.Jobs, wantXXSJobs)
	}
	if math.Abs(xs.Jobs-wantXSJobs) > 1e-6 {
		t.Errorf("XS Jobs=%.6f, want %.6f", xs.Jobs, wantXSJobs)
	}
}

func TestEstimateCapacity_MeanIdentity(t *testing.T) {
	// With per-category means, totalJobs(n) == reportCapacitySec(n) / avgReportCost
	// (holds exactly when no category hits the cost floor).
	// 2 × 500ms (XXS) + 2 × 2000ms (XS)
	// totalReportMs=5000ms, |R|=4, avgCost=1.25s, reportShare=1.0
	tasks := []BgTask{
		makeTask(typeReport, 500),
		makeTask(typeReport, 500),
		makeTask(typeReport, 2000),
		makeTask(typeReport, 2000),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{4})
	if skipped {
		t.Fatal("expected estimate to run")
	}

	we := est.WorkerEstimates[0]
	reportCapSec := 4.0 * float64(secondsPerHour) * est.ReportShare
	avgCostSec := float64(est.TotalReportMs) / float64(est.ReportCount) / 1000.0
	wantTotalJobs := reportCapSec / avgCostSec

	if math.Abs(we.TotalJobs-wantTotalJobs) > 1e-6 {
		t.Errorf("TotalJobs=%.6f, want %.6f (mean identity)", we.TotalJobs, wantTotalJobs)
	}
}

func TestEstimateCapacity_CostFloor(t *testing.T) {
	// Sub-100ms tasks: mean = 15ms = 0.015s < costFloorSec → clamped to 0.1s.
	tasks := []BgTask{
		makeTask(typeReport, 10),
		makeTask(typeReport, 20),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{1})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	xxs := est.WorkerEstimates[0].Categories[0]
	if !xxs.FloorApplied {
		t.Error("expected FloorApplied=true for sub-100ms tasks")
	}
	if math.Abs(xxs.RepresentativeSec-costFloorSec) > 1e-9 {
		t.Errorf("RepresentativeSec=%.4f, want %.4f (floor)", xxs.RepresentativeSec, costFloorSec)
	}
	if xxs.Jobs <= 0 || math.IsInf(xxs.Jobs, 0) || math.IsNaN(xxs.Jobs) {
		t.Errorf("Jobs=%.4f, want finite positive", xxs.Jobs)
	}
}

func TestEstimateCapacity_DemandProfile(t *testing.T) {
	// 24-hour window: hours 0–19 have 10 tasks each, hours 20–23 have 50 tasks each.
	// total = 20×10 + 4×50 = 400 tasks
	// observedHours = 24
	// avgPerHour = 400/24 ≈ 16.667
	// peakPerHour (p95): sorted counts = [10×20, 50×4]; position = 0.95×23 = 21.85
	//   lo=21 → sorted[21]=50, hi=22 → sorted[22]=50 → p95 = 50.0
	// busiestHourCount = 50
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := make([]BgTask, 0, 400)
	for h := 0; h < 20; h++ {
		for i := 0; i < 10; i++ {
			tasks = append(tasks, makeTaskAt(typeReport, 1000, base.Add(time.Duration(h)*time.Hour+time.Duration(i)*time.Minute)))
		}
	}
	for h := 20; h < 24; h++ {
		for i := 0; i < 50; i++ {
			tasks = append(tasks, makeTaskAt(typeReport, 1000, base.Add(time.Duration(h)*time.Hour+time.Duration(i)*time.Minute)))
		}
	}

	est, skipped, _ := EstimateCapacity(tasks, []int{4})
	if skipped {
		t.Fatal("expected estimate to run")
	}

	d := est.Demand
	if !d.Available {
		t.Error("expected Demand.Available=true for 24-hour window")
	}
	if d.ObservedHours != 24 {
		t.Errorf("ObservedHours=%d, want 24", d.ObservedHours)
	}
	wantAvg := 400.0 / 24.0
	if math.Abs(d.AvgPerHour-wantAvg) > 1e-9 {
		t.Errorf("AvgPerHour=%.6f, want %.6f", d.AvgPerHour, wantAvg)
	}
	if math.Abs(d.PeakPerHour-50.0) > 1e-9 {
		t.Errorf("PeakPerHour=%.4f, want 50.0", d.PeakPerHour)
	}
	if d.BusiestHourCount != 50 {
		t.Errorf("BusiestHourCount=%d, want 50", d.BusiestHourCount)
	}
}

func TestEstimateCapacity_InsufficientSpan(t *testing.T) {
	// Window < 24 hours: base to base+22h59m → latestHour = 22:00 → observedHours = 23 < 24.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tasks := []BgTask{
		makeTaskAt(typeReport, 1000, base),
		makeTaskAt(typeReport, 1000, base.Add(22*time.Hour+59*time.Minute)),
	}

	est, skipped, _ := EstimateCapacity(tasks, []int{4})
	if skipped {
		t.Fatal("expected estimate to run")
	}

	d := est.Demand
	if d.Available {
		t.Errorf("expected Demand.Available=false for %d observed hours", d.ObservedHours)
	}
	if d.AvgPerHour <= 0 {
		t.Error("expected positive AvgPerHour even when insufficient span")
	}
}

func TestVerdictFor_Branches(t *testing.T) {
	d := DemandProfile{
		Available:   true,
		AvgPerHour:  38.0,
		PeakPerHour: 210.0,
	}

	// cap >= peak → covers peak
	v := VerdictFor(1240, d)
	if !strings.Contains(v, "covers your peak hour") {
		t.Errorf("want 'covers your peak hour', got %q", v)
	}

	// avg <= cap < peak → keeps up with average
	v = VerdictFor(100, d)
	if !strings.Contains(v, "keeps up with average demand") {
		t.Errorf("want 'keeps up with average demand', got %q", v)
	}

	// cap < avg → below average
	v = VerdictFor(10, d)
	if !strings.Contains(v, "below your average demand") {
		t.Errorf("want 'below your average demand', got %q", v)
	}

	// insufficient span → no peak in verdict
	d.Available = false
	v = VerdictFor(1000, d)
	if !strings.Contains(v, "insufficient time span") {
		t.Errorf("want 'insufficient time span', got %q", v)
	}
}

func TestEstimateCapacity_AllEightCategories(t *testing.T) {
	// Only XXS tasks; all other categories are empty.
	tasks := []BgTask{
		makeTask(typeReport, 500),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{4})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	cats := est.WorkerEstimates[0].Categories
	if len(cats) != 8 {
		t.Fatalf("len(Categories)=%d, want 8", len(cats))
	}
	if cats[0].BucketCount != 1 {
		t.Errorf("XXS BucketCount=%d, want 1", cats[0].BucketCount)
	}
	for i := 1; i < 8; i++ {
		if cats[i].BucketCount != 0 {
			t.Errorf("category %d BucketCount=%d, want 0 (empty)", i, cats[i].BucketCount)
		}
	}
}

func TestEstimateCapacity_IssuesSyncExcluded(t *testing.T) {
	// ISSUE_SYNC tasks must not affect reportShare or category sums.
	tasks := []BgTask{
		makeTask(typeIssueSync, 100_000), // huge — must be ignored
		makeTask(typeReport, 1000),
		makeTask(typeReport, 1000),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{1})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	// All non-ISSUE_SYNC is REPORT → reportShare == 1
	if math.Abs(est.ReportShare-1.0) > 1e-9 {
		t.Errorf("ReportShare=%.6f, want 1.0", est.ReportShare)
	}
}

func TestEstimateCapacity_AllNonSyncIsReport(t *testing.T) {
	tasks := []BgTask{
		makeTask(typeReport, 500),
		makeTask(typeReport, 2000),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{2})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	if math.Abs(est.ReportShare-1.0) > 1e-9 {
		t.Errorf("ReportShare=%.6f, want 1.0", est.ReportShare)
	}
}

func TestEstimateCapacity_CategorySharesSumToOne(t *testing.T) {
	tasks := []BgTask{
		makeTask(typeReport, 500),    // XXS
		makeTask(typeReport, 2000),   // XS
		makeTask(typeReport, 4000),   // S
		makeTask(typeReport, 7000),   // M
		makeTask(typeReport, 20000),  // L
		makeTask(typeReport, 45000),  // XL
		makeTask(typeReport, 120000), // XXL
		makeTask(typeReport, 200000), // XXXL
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{1})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	sum := 0.0
	for _, c := range est.WorkerEstimates[0].Categories {
		sum += c.Share
	}
	if math.Abs(sum-1.0) > 1e-9 {
		t.Errorf("category shares sum=%.10f, want 1.0", sum)
	}
}

func TestEstimateCapacity_NoReportTasks(t *testing.T) {
	tasks := []BgTask{
		makeTask("OTHER", 5000),
	}
	_, skipped, reason := EstimateCapacity(tasks, []int{4})
	if !skipped {
		t.Fatal("expected estimate to be skipped when no REPORT tasks")
	}
	if reason == "" {
		t.Error("expected a non-empty skip reason")
	}
}

func TestEstimateCapacity_AllZeroExecutionTime(t *testing.T) {
	tasks := []BgTask{
		makeTask(typeReport, 0),
	}
	_, skipped, reason := EstimateCapacity(tasks, []int{4})
	if !skipped {
		t.Fatal("expected estimate to be skipped when all execution times are zero")
	}
	if reason == "" {
		t.Error("expected a non-empty skip reason")
	}
}

func TestEstimateCapacity_DuplicateAndUnsortedWorkers(t *testing.T) {
	tasks := []BgTask{
		makeTask(typeReport, 1000),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{8, 4, 4, 8, 16})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	// de-duped and sorted: [4, 8, 16]
	if len(est.WorkerEstimates) != 3 {
		t.Fatalf("WorkerEstimates len=%d, want 3", len(est.WorkerEstimates))
	}
	if est.WorkerEstimates[0].Workers != 4 {
		t.Errorf("Workers[0]=%d, want 4", est.WorkerEstimates[0].Workers)
	}
	if est.WorkerEstimates[1].Workers != 8 {
		t.Errorf("Workers[1]=%d, want 8", est.WorkerEstimates[1].Workers)
	}
	if est.WorkerEstimates[2].Workers != 16 {
		t.Errorf("Workers[2]=%d, want 16", est.WorkerEstimates[2].Workers)
	}
}

func TestEstimateCapacity_XXXLLabel(t *testing.T) {
	// XXXL task: 300s = 300000ms
	tasks := []BgTask{
		makeTask(typeReport, 300_000),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{1})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	xxxl := est.WorkerEstimates[0].Categories[7]
	// maxObservedMs = 300000 → (300000 + 999) / 1000 = 300
	if xxxl.UpperBoundSec != 300 {
		t.Errorf("XXXL UpperBoundSec=%d, want 300", xxxl.UpperBoundSec)
	}
	wantLabel := "XXXL (180-300s)"
	if xxxl.Label != wantLabel {
		t.Errorf("XXXL Label=%q, want %q", xxxl.Label, wantLabel)
	}
}

func TestEstimateCapacity_EmptyCategories_NoDiv0(t *testing.T) {
	// Only XXS tasks; all other categories empty → no panic, zero jobs.
	tasks := []BgTask{
		makeTask(typeReport, 500),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{4})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	for i, c := range est.WorkerEstimates[0].Categories {
		if math.IsNaN(c.Jobs) || math.IsInf(c.Jobs, 0) {
			t.Errorf("category %d Jobs is NaN or Inf", i)
		}
	}
}

func TestSortedDedup(t *testing.T) {
	got := sortedDedup([]int{8, 4, 4, 16, 8})
	want := []int{4, 8, 16}
	if len(got) != len(want) {
		t.Fatalf("len=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%d, want %d", i, got[i], want[i])
		}
	}
}

func TestRepresentativeCost_EmptyBucket(t *testing.T) {
	got, floor := representativeCost(nil)
	if got != 0 {
		t.Errorf("got %.2f, want 0 for empty bucket", got)
	}
	if floor {
		t.Error("expected no floor applied for empty bucket")
	}
}

func TestEstimateCapacity_TwoWorkerCounts_Proportional(t *testing.T) {
	// 8 workers should give exactly 2× the jobs of 4 workers.
	tasks := []BgTask{
		makeTask(typeReport, 1000),
		makeTask(typeReport, 2000),
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{4, 8})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	jobs4 := est.WorkerEstimates[0].TotalJobs
	jobs8 := est.WorkerEstimates[1].TotalJobs
	if math.Abs(jobs8-jobs4*2) > 1e-9 {
		t.Errorf("jobs8=%.4f, want 2×jobs4=%.4f", jobs8, jobs4*2)
	}
}
