package bgtasks

import (
	"math"
	"testing"
)

func makeTask(taskType string, execMs int) BgTask {
	return BgTask{Type: taskType, ExecutionTimeMs: execMs}
}

func TestEstimateCapacity_HappyPath(t *testing.T) {
	// Non-ISSUE_SYNC non-REPORT: 2 tasks × 1000ms = 2000ms
	// REPORT XXS: 2 tasks × 500ms = 1000ms
	// REPORT XS:  2 tasks × 2000ms = 4000ms
	// totalNonSyncMs = 2000 + 1000 + 4000 = 7000
	// totalReportMs  = 1000 + 4000        = 5000
	// reportShare = 5000/7000
	tasks := []BgTask{
		makeTask("OTHER", 1000),
		makeTask("OTHER", 1000),
		makeTask(typeReport, 500),
		makeTask(typeReport, 500),
		makeTask(typeReport, 2000),
		makeTask(typeReport, 2000),
	}

	est, skipped, _ := EstimateCapacity(tasks, []int{4})
	if skipped {
		t.Fatal("expected estimate to run, got skipped")
	}

	wantReportShare := 5000.0 / 7000.0
	if math.Abs(est.ReportShare-wantReportShare) > 1e-9 {
		t.Errorf("ReportShare=%.6f, want %.6f", est.ReportShare, wantReportShare)
	}

	if len(est.WorkerEstimates) != 1 {
		t.Fatalf("WorkerEstimates len=%d, want 1", len(est.WorkerEstimates))
	}
	we := est.WorkerEstimates[0]
	if we.Workers != 4 {
		t.Errorf("Workers=%d, want 4", we.Workers)
	}

	// XXS = index 0, XS = index 1
	xxs := we.Categories[0]
	xs := we.Categories[1]

	wantXXSShare := 1000.0 / 5000.0
	if math.Abs(xxs.Share-wantXXSShare) > 1e-9 {
		t.Errorf("XXS Share=%.6f, want %.6f", xxs.Share, wantXXSShare)
	}
	wantXSShare := 4000.0 / 5000.0
	if math.Abs(xs.Share-wantXSShare) > 1e-9 {
		t.Errorf("XS Share=%.6f, want %.6f", xs.Share, wantXSShare)
	}

	// mode of [500ms, 500ms] rounded to 0.1s = 0.5s
	if math.Abs(xxs.RepresentativeSec-0.5) > 1e-9 {
		t.Errorf("XXS RepresentativeSec=%.2f, want 0.5", xxs.RepresentativeSec)
	}
	// mode of [2000ms, 2000ms] rounded to 0.1s = 2.0s
	if math.Abs(xs.RepresentativeSec-2.0) > 1e-9 {
		t.Errorf("XS RepresentativeSec=%.2f, want 2.0", xs.RepresentativeSec)
	}

	// 4 workers × 3600s × (5000/7000) × (1000/5000) / 0.5
	capacitySec := 4.0 * 3600.0
	reportCap := capacitySec * wantReportShare
	wantXXSJobs := reportCap * wantXXSShare / 0.5
	wantXSJobs := reportCap * wantXSShare / 2.0
	wantTotal := wantXXSJobs + wantXSJobs

	if math.Abs(xxs.Jobs-wantXXSJobs) > 1e-6 {
		t.Errorf("XXS Jobs=%.4f, want %.4f", xxs.Jobs, wantXXSJobs)
	}
	if math.Abs(xs.Jobs-wantXSJobs) > 1e-6 {
		t.Errorf("XS Jobs=%.4f, want %.4f", xs.Jobs, wantXSJobs)
	}
	if math.Abs(we.TotalJobs-wantTotal) > 1e-6 {
		t.Errorf("TotalJobs=%.4f, want %.4f", we.TotalJobs, wantTotal)
	}

	// ±20% margin
	if math.Abs(we.TotalLow-wantTotal*0.8) > 1e-6 {
		t.Errorf("TotalLow=%.4f, want %.4f", we.TotalLow, wantTotal*0.8)
	}
	if math.Abs(we.TotalHigh-wantTotal*1.2) > 1e-6 {
		t.Errorf("TotalHigh=%.4f, want %.4f", we.TotalHigh, wantTotal*1.2)
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

func TestEstimateCapacity_ModeFloor(t *testing.T) {
	// Tasks with sub-50ms execution times: mode rounds to 0s → clamped to 0.1s.
	tasks := []BgTask{
		makeTask(typeReport, 10), // 0.01s → rounds to 0 → clamped to 0.1s
		makeTask(typeReport, 20), // 0.02s → rounds to 0 → clamped to 0.1s
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{1})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	xxs := est.WorkerEstimates[0].Categories[0]
	if math.Abs(xxs.RepresentativeSec-0.1) > 1e-9 {
		t.Errorf("RepresentativeSec=%.4f, want 0.1 (floor)", xxs.RepresentativeSec)
	}
	// Job count must be finite and positive
	if xxs.Jobs <= 0 || math.IsInf(xxs.Jobs, 0) || math.IsNaN(xxs.Jobs) {
		t.Errorf("Jobs=%.4f, want finite positive", xxs.Jobs)
	}
}

func TestEstimateCapacity_ModeTieBreak(t *testing.T) {
	// Two equally-frequent rounded values: 0.5s and 1.0s → pick smaller (0.5s).
	tasks := []BgTask{
		makeTask(typeReport, 500),  // 0.5s
		makeTask(typeReport, 1000), // 1.0s  (but still in XXS: 0<x≤1)
	}
	est, skipped, _ := EstimateCapacity(tasks, []int{1})
	if skipped {
		t.Fatal("expected estimate to run")
	}
	xxs := est.WorkerEstimates[0].Categories[0]
	// Both round to different values (0.5 and 1.0), frequency 1 each → pick smaller: 0.5
	if math.Abs(xxs.RepresentativeSec-0.5) > 1e-9 {
		t.Errorf("RepresentativeSec=%.2f, want 0.5 (smaller tie)", xxs.RepresentativeSec)
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
