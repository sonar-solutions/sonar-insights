package bgtasks

import (
	"math"
	"testing"
	"time"
)

// loadGoldenTasks loads the C# test data and deduplicates, matching the golden master setup.
func loadGoldenTasks(t *testing.T) []BgTask {
	t.Helper()
	tasks, err := Load("../../../test-data", nopLogger)
	if err != nil {
		t.Fatalf("load test data: %v", err)
	}
	return tasks
}

func TestAnalyzeDateRange_Empty(t *testing.T) {
	dr := AnalyzeDateRange(nil)
	if dr.DateRangeInDays != 0 {
		t.Errorf("empty: DateRangeInDays=%d, want 0", dr.DateRangeInDays)
	}
}

func TestAnalyzeDateRange_SingleTask(t *testing.T) {
	submitted := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	executed := time.Date(2026, 1, 1, 10, 0, 5, 0, time.UTC)
	tasks := []BgTask{{SubmittedAt: submitted, ExecutedAt: executed}}
	dr := AnalyzeDateRange(tasks)
	if dr.DateRangeInDays != 1 {
		t.Errorf("single task same day: DateRangeInDays=%d, want 1", dr.DateRangeInDays)
	}
}

func TestAnalyzeDateRange_SpansTwoDays(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 2, 23, 59, 59, 0, time.UTC)
	tasks := []BgTask{
		{SubmittedAt: t1, ExecutedAt: t1.Add(time.Minute)},
		{SubmittedAt: t2, ExecutedAt: t2.Add(time.Minute)},
	}
	dr := AnalyzeDateRange(tasks)
	if dr.DateRangeInDays != 2 {
		t.Errorf("two days: DateRangeInDays=%d, want 2", dr.DateRangeInDays)
	}
}

func TestAnalyzeDateRange_ZeroExecutedAt(t *testing.T) {
	submitted := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	tasks := []BgTask{{SubmittedAt: submitted}} // ExecutedAt is zero
	dr := AnalyzeDateRange(tasks)
	if !dr.LatestCompletion.IsZero() {
		t.Errorf("LatestCompletion = %v, want zero when no task has ExecutedAt", dr.LatestCompletion)
	}
}

func TestAnalyzeDateRange_MixedDataset(t *testing.T) {
	day := func(n int) time.Time {
		return time.Date(2026, 1, n, 0, 0, 0, 0, time.UTC)
	}
	tasks := []BgTask{
		{SubmittedAt: day(1), ExecutedAt: day(5)},
		{SubmittedAt: day(10), ExecutedAt: time.Time{}},
	}
	dr := AnalyzeDateRange(tasks)
	if !dr.LatestCompletion.Equal(day(5)) {
		t.Errorf("LatestCompletion = %v, want day 5", dr.LatestCompletion)
	}
}

func TestAnalyzeDateRange_GoldenMaster(t *testing.T) {
	tasks := loadGoldenTasks(t)
	dr := AnalyzeDateRange(tasks)
	// Golden master: DateRangeInDays=50
	if dr.DateRangeInDays != 50 {
		t.Errorf("DateRangeInDays=%d, want 50", dr.DateRangeInDays)
	}
}

func TestAnalyzeOverall_GoldenMaster(t *testing.T) {
	tasks := loadGoldenTasks(t)
	dr := AnalyzeDateRange(tasks)
	// totalBeforeFilter = 3816 (deduplicated but unfiltered), unique = 3815
	overall := AnalyzeOverall(tasks, 3816, dr)

	if overall.TotalTasks != 3816 {
		t.Errorf("TotalTasks=%d, want 3816", overall.TotalTasks)
	}
	if overall.TotalUniqueTasks != 3815 {
		t.Errorf("TotalUniqueTasks=%d, want 3815", overall.TotalUniqueTasks)
	}
	// AverageTasksPerDay = 3815/50 = 76.3
	wantAvg := 76.3
	if math.Abs(overall.AverageTasksPerDay-wantAvg) > 0.05 {
		t.Errorf("AverageTasksPerDay=%.4f, want %.4f", overall.AverageTasksPerDay, wantAvg)
	}
	if overall.BusiestDayTaskCount != 800 {
		t.Errorf("BusiestDayTaskCount=%d, want 800", overall.BusiestDayTaskCount)
	}
	// FailedTasksPercentage ≈ 0.0786%
	if math.Abs(overall.FailedTasksPercentage-0.0786) > 0.001 {
		t.Errorf("FailedTasksPercentage=%.4f, want ~0.0786", overall.FailedTasksPercentage)
	}
}

func TestAnalyzeProjectAnalysis_GoldenMaster(t *testing.T) {
	tasks := loadGoldenTasks(t)
	dr := AnalyzeDateRange(tasks)
	pm := AnalyzeProjectAnalysis(tasks, dr)

	if pm.TotalProjectAnalysisTasks != 1342 {
		t.Errorf("TotalProjectAnalysisTasks=%d, want 1342", pm.TotalProjectAnalysisTasks)
	}
	if pm.TotalProjectsAnalyzed != 42 {
		t.Errorf("TotalProjectsAnalyzed=%d, want 42", pm.TotalProjectsAnalyzed)
	}
	if pm.TotalBranchAnalysisTasks != 1341 {
		t.Errorf("TotalBranchAnalysisTasks=%d, want 1341", pm.TotalBranchAnalysisTasks)
	}
	if pm.TotalPullRequestAnalysisTasks != 1 {
		t.Errorf("TotalPullRequestAnalysisTasks=%d, want 1", pm.TotalPullRequestAnalysisTasks)
	}
	if len(pm.TopProjects) != 10 {
		t.Errorf("TopProjects count=%d, want 10", len(pm.TopProjects))
	}
}

func TestAnalyzeTopProjects_Order(t *testing.T) {
	tasks := []BgTask{
		{Type: typeReport, ComponentKey: "b"},
		{Type: typeReport, ComponentKey: "b"},
		{Type: typeReport, ComponentKey: "a"},
		{Type: typeReport, ComponentKey: "a"},
		{Type: typeReport, ComponentKey: "c"},
	}
	top := analyzeTopProjects(tasks, 5)
	// b and a both have 2, c has 1; ties broken alphabetically: a before b
	if top[0].Key != "a" || top[1].Key != "b" || top[2].Key != "c" {
		t.Errorf("unexpected order: %v %v %v", top[0].Key, top[1].Key, top[2].Key)
	}
}

func TestAnalyzeTopProjects_LimitsTo10(t *testing.T) {
	tasks := make([]BgTask, 15)
	for i := range tasks {
		tasks[i] = BgTask{Type: typeReport, ComponentKey: string(rune('a' + i))}
	}
	top := analyzeTopProjects(tasks, 15)
	if len(top) != 10 {
		t.Errorf("top count=%d, want 10", len(top))
	}
}

func TestCategorizeTime_BucketBoundaries(t *testing.T) {
	// The 0-1s bucket is inclusive on both bounds (value of 1s goes into 0-1s, not 1-3s).
	// The 1-3s bucket: value of 3s goes into 1-3s (inclusive upper, exclusive lower).
	tasks := []BgTask{
		{Type: typeReport, ExecutionTimeMs: 1000, SubmittedAt: time.Now(), StartedAt: time.Now()}, // 1s → 0-1s
		{Type: typeReport, ExecutionTimeMs: 3000, SubmittedAt: time.Now(), StartedAt: time.Now()}, // 3s → 1-3s
		{Type: typeReport, ExecutionTimeMs: 5000, SubmittedAt: time.Now(), StartedAt: time.Now()}, // 5s → 3-5s
	}
	result := categorizeTime(tasks, 3, func(t BgTask) float64 { return float64(t.ExecutionTimeMs) / 1000 })
	if result[0].Label != "0-1s" || result[0].Count != 1 {
		t.Errorf("0-1s bucket: count=%d, want 1", result[0].Count)
	}
	if result[1].Label != "1-3s" || result[1].Count != 1 {
		t.Errorf("1-3s bucket: count=%d, want 1", result[1].Count)
	}
	if result[2].Label != "3-5s" || result[2].Count != 1 {
		t.Errorf("3-5s bucket: count=%d, want 1", result[2].Count)
	}
}

func TestAnalyzeCharts_GoldenMaster(t *testing.T) {
	tasks := loadGoldenTasks(t)
	dr := AnalyzeDateRange(tasks)
	pm := AnalyzeProjectAnalysis(tasks, dr)
	charts := AnalyzeCharts(tasks, dr, pm, time.Time{}, time.Time{})

	// Golden: TaskTypeCount=5
	if len(charts.SummaryByType) != 5 {
		t.Errorf("SummaryByType count=%d, want 5", len(charts.SummaryByType))
	}
	// Golden: StatusTypeCount=2
	if len(charts.SummaryByStatus) != 2 {
		t.Errorf("SummaryByStatus count=%d, want 2", len(charts.SummaryByStatus))
	}
	// Golden: UniqueSubmittersCount=3
	if len(charts.SummaryBySubmitter) != 3 {
		t.Errorf("SummaryBySubmitter count=%d, want 3", len(charts.SummaryBySubmitter))
	}
}

func TestAnalyzeCharts_ZeroFill(t *testing.T) {
	// Two tasks three days apart — the middle day should be zero-filled.
	d1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC)
	tasks := []BgTask{
		{Type: "REPORT", SubmittedAt: d1, StartedAt: d1, ExecutedAt: d1.Add(time.Minute)},
		{Type: "REPORT", SubmittedAt: d3, StartedAt: d3, ExecutedAt: d3.Add(time.Minute)},
	}
	dr := AnalyzeDateRange(tasks)
	pm := AnalyzeProjectAnalysis(tasks, dr)
	charts := AnalyzeCharts(tasks, dr, pm, time.Time{}, time.Time{})

	if len(charts.TasksPerDay) != 3 {
		t.Errorf("TasksPerDay length=%d, want 3 (zero-filled)", len(charts.TasksPerDay))
	}
	middle := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if v, ok := charts.TasksPerDay[middle]; !ok || v != 0 {
		t.Errorf("middle day count=%d (ok=%v), want 0", v, ok)
	}
}

func TestGetBranchExecTimeSeries_Empty(t *testing.T) {
	s := getBranchExecTimeSeries(nil, 0)
	if s.ProjectRank != 0 || s.ProjectKey != "" {
		t.Errorf("empty: unexpected result %+v", s)
	}
}

func TestGetBranchExecTimeSeries_TieBreak(t *testing.T) {
	// Projects "a" and "b" both have 2 tasks; "a" should rank first (alphabetical tiebreak).
	tasks := []BgTask{
		{Type: typeReport, BranchType: "BRANCH", ComponentKey: "b", Branch: "main",
			StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ExecutionTimeMs: 1000,
			SubmittedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ExecutedAt: time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC)},
		{Type: typeReport, BranchType: "BRANCH", ComponentKey: "b", Branch: "main",
			StartedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), ExecutionTimeMs: 2000,
			SubmittedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), ExecutedAt: time.Date(2026, 1, 2, 0, 0, 2, 0, time.UTC)},
		{Type: typeReport, BranchType: "BRANCH", ComponentKey: "a", Branch: "main",
			StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ExecutionTimeMs: 500,
			SubmittedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ExecutedAt: time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC)},
		{Type: typeReport, BranchType: "BRANCH", ComponentKey: "a", Branch: "main",
			StartedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), ExecutionTimeMs: 600,
			SubmittedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), ExecutedAt: time.Date(2026, 1, 2, 0, 0, 1, 0, time.UTC)},
	}
	s := getBranchExecTimeSeries(tasks, 0)
	if s.ProjectKey != "a" {
		t.Errorf("rank 0 project=%q, want \"a\" (alphabetical tiebreak)", s.ProjectKey)
	}
}
