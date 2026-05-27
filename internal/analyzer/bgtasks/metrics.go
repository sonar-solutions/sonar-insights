package bgtasks

import (
	"fmt"
	"sort"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/mathutil"
)

const (
	typeReport    = "REPORT"
	typeIssueSync = "ISSUE_SYNC"
	statusFailed  = "FAILED"
	noProjectKey  = "no project key - error"
	noType        = "no type - error"
	noStatus      = "no status - error"
	noSubmitter   = "no submitter - error"
)

var timeThresholds = []struct {
	upperBoundSec int
	label         string
}{
	{1, "0-1s"},
	{3, "1-3s"},
	{5, "3-5s"},
	{10, "5-10s"},
	{30, "10-30s"},
	{60, "30-60s"},
	{180, "60-180s"},
	{int(^uint(0) >> 1), "> 180s"},
}

// DateRange holds the overall time span of the analysed tasks.
type DateRange struct {
	EarliestSubmission time.Time
	LatestCompletion   time.Time
	DateRangeInDays    int
}

// OverallMetrics holds aggregate statistics across all task types.
type OverallMetrics struct {
	TotalTasks            int
	TotalUniqueTasks      int
	AverageTasksPerDay    float64
	BusiestDayTaskCount   int
	FailedTasksPercentage float64
}

// ProjectAnalysisMetrics holds statistics specific to REPORT-type tasks.
type ProjectAnalysisMetrics struct {
	TotalProjectAnalysisTasks     int
	TotalProjectsAnalyzed         int
	AverageProjectAnalysisTasks   float64
	TotalPullRequestAnalysisTasks int
	TotalBranchAnalysisTasks      int
	BranchAndPrExecutionTimesSec  map[string]float64
	TopProjects                   []TopProject
}

// TopProject holds per-project rank data.
type TopProject struct {
	Key        string
	Count      int
	Percentage float64
}

// TimeCategoryMetric holds a histogram bucket.
type TimeCategoryMetric struct {
	UpperBoundSec int
	Label         string
	Count         int
	Percentage    float64
}

// BranchExecutionTimeSeries holds ordered execution times for one branch of one project.
type BranchExecutionTimeSeries struct {
	ProjectRank           int
	ProjectKey            string
	BranchName            string
	ExecutionTimesSeconds []ExecPoint
}

// ExecPoint is a (label, seconds) pair ordered by start time.
type ExecPoint struct {
	Label   string
	Seconds float64
}

// ChartDatasets holds all pre-computed chart data.
type ChartDatasets struct {
	TasksPerDay               map[time.Time]int
	ProjectAnalysesPerDay     map[time.Time]int
	TasksPerDayByType         map[time.Time]map[string]int
	AllTaskTypes              []string
	SummaryByPendingTime      []TimeCategoryMetric
	SummaryByExecutionTime    []TimeCategoryMetric
	SummaryByType             map[string]int
	SummaryByStatus           map[string]int
	SummaryBySubmitter        map[string]int
	SummaryByWarningCount     map[string]int
	TopProjectBranchExecTimes [3]BranchExecutionTimeSeries
}

// AnalysisResults is the top-level output of the analysis pipeline.
type AnalysisResults struct {
	DateRange       DateRange
	Overall         OverallMetrics
	ProjectAnalysis ProjectAnalysisMetrics
	Charts          ChartDatasets
	CapacityDemand  CapacityDemandResults
}

// AnalyzeDateRange computes the earliest submission and latest completion.
func AnalyzeDateRange(tasks []BgTask) DateRange {
	if len(tasks) == 0 {
		return DateRange{}
	}
	earliest := tasks[0].SubmittedAt
	for _, t := range tasks {
	latestSubmission := tasks[0].SubmittedAt
	for _, t := range tasks[1:] {
		if t.SubmittedAt.Before(earliest) {
			earliest = t.SubmittedAt
		}
		if !t.ExecutedAt.IsZero() {
			if executed := t.ExecutedAt.UTC(); executed.After(latest) {
				latest = executed
			}
		}
		if t.SubmittedAt.After(latestSubmission) {
			latestSubmission = t.SubmittedAt
		}
	}
	days := int(utcDate(latestSubmission).Sub(utcDate(earliest)).Hours()/24) + 1
	return DateRange{
		EarliestSubmission: earliest,
		LatestCompletion:   latest,
		DateRangeInDays:    days,
	}
}

// AnalyzeOverall computes aggregate metrics across all task types.
func AnalyzeOverall(tasks []BgTask, totalBeforeFilter int, dr DateRange) OverallMetrics {
	var avgPerDay float64
	if dr.DateRangeInDays > 0 {
		avgPerDay = float64(len(tasks)) / float64(dr.DateRangeInDays)
	}

	byDay := make(map[time.Time]int)
	for _, t := range tasks {
		byDay[utcDate(t.SubmittedAt)]++
	}
	busiest := 0
	for _, c := range byDay {
		if c > busiest {
			busiest = c
		}
	}

	failed := 0
	for _, t := range tasks {
		if t.Status == statusFailed {
			failed++
		}
	}
	var failedPct float64
	if len(tasks) > 0 {
		failedPct = float64(failed) / float64(len(tasks)) * 100
	}

	return OverallMetrics{
		TotalTasks:            totalBeforeFilter,
		TotalUniqueTasks:      len(tasks),
		AverageTasksPerDay:    avgPerDay,
		BusiestDayTaskCount:   busiest,
		FailedTasksPercentage: failedPct,
	}
}

// AnalyzeProjectAnalysis computes metrics for REPORT-type tasks.
func AnalyzeProjectAnalysis(tasks []BgTask, dr DateRange) ProjectAnalysisMetrics {
	var reportTasks []BgTask
	for _, t := range tasks {
		if t.Type == typeReport {
			reportTasks = append(reportTasks, t)
		}
	}

	projects := make(map[string]struct{})
	for _, t := range reportTasks {
		key := t.ComponentKey
		if key == "" {
			key = noProjectKey
		}
		projects[key] = struct{}{}
	}

	var avgPerDay float64
	if dr.DateRangeInDays > 0 {
		avgPerDay = float64(len(reportTasks)) / float64(dr.DateRangeInDays)
	}

	prCount := 0
	for _, t := range reportTasks {
		if t.PullRequestID != "" {
			prCount++
		}
	}

	return ProjectAnalysisMetrics{
		TotalProjectAnalysisTasks:     len(reportTasks),
		TotalProjectsAnalyzed:         len(projects),
		AverageProjectAnalysisTasks:   avgPerDay,
		TotalPullRequestAnalysisTasks: prCount,
		TotalBranchAnalysisTasks:      len(reportTasks) - prCount,
		BranchAndPrExecutionTimesSec:  analyzeBranchAndPrStats(tasks),
		TopProjects:                   analyzeTopProjects(tasks, len(reportTasks)),
	}
}

func analyzeBranchAndPrStats(tasks []BgTask) map[string]float64 {
	const p = 0.8
	var prTimes, branchTimes []float64
	for _, t := range tasks {
		if t.Type != typeReport {
			continue
		}
		sec := float64(t.ExecutionTimeMs) / 1000.0
		if t.PullRequestID != "" {
			prTimes = append(prTimes, sec)
		} else {
			branchTimes = append(branchTimes, sec)
		}
	}

	prAvg := avg(prTimes)
	branchAvg := avg(branchTimes)

	return map[string]float64{
		"PR Avg":                 prAvg,
		"PR 80th percentile":     mathutil.CalculatePercentile(prTimes, p),
		"Branch Avg":             branchAvg,
		"Branch 80th percentile": mathutil.CalculatePercentile(branchTimes, p),
	}
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func analyzeTopProjects(tasks []BgTask, totalReport int) []TopProject {
	counts := make(map[string]int)
	for _, t := range tasks {
		if t.Type != typeReport {
			continue
		}
		key := t.ComponentKey
		if key == "" {
			key = noProjectKey
		}
		counts[key]++
	}

	type entry struct {
		key   string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for k, c := range counts {
		entries = append(entries, entry{k, c})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].key < entries[j].key
	})

	safeTotal := totalReport
	if safeTotal == 0 {
		safeTotal = 1
	}
	top := entries
	if len(top) > 10 {
		top = top[:10]
	}
	result := make([]TopProject, len(top))
	for i, e := range top {
		result[i] = TopProject{
			Key:        e.key,
			Count:      e.count,
			Percentage: float64(e.count) / float64(safeTotal) * 100,
		}
	}
	return result
}

// AnalyzeCharts computes all chart datasets. from/to define the zero-fill range
// when non-zero; otherwise the task date range is used.
func AnalyzeCharts(tasks []BgTask, dr DateRange, pm ProjectAnalysisMetrics, from, to time.Time) ChartDatasets {
	rangeStart, rangeEnd := chartRange(dr, from, to)

	tasksPerDay := countByDay(tasks, func(t BgTask) bool { return true })
	projectAnalysesPerDay := countByDay(tasks, func(t BgTask) bool { return t.Type == typeReport })

	filledTasks := zeroFill(tasksPerDay, rangeStart, rangeEnd)
	filledProjectAnalyses := zeroFill(projectAnalysesPerDay, rangeStart, rangeEnd)

	allTypes := allDistinctTypes(tasks)
	tasksByType := buildTasksPerDayByType(tasks, allTypes)
	filledByType := zeroFillByType(tasksByType, allTypes, rangeStart, rangeEnd)

	return ChartDatasets{
		TasksPerDay:           filledTasks,
		ProjectAnalysesPerDay: filledProjectAnalyses,
		TasksPerDayByType:     filledByType,
		AllTaskTypes:          allTypes,
		SummaryByPendingTime: categorizeTime(tasks, pm.TotalProjectAnalysisTasks, func(t BgTask) float64 {
			return t.StartedAt.Sub(t.SubmittedAt).Seconds()
		}),
		SummaryByExecutionTime: categorizeTime(tasks, pm.TotalProjectAnalysisTasks, func(t BgTask) float64 {
			return float64(t.ExecutionTimeMs) / 1000.0
		}),
		SummaryByType: summaryByField(tasks, func(t BgTask) string {
			if t.Type == "" {
				return noType
			}
			return t.Type
		}),
		SummaryByStatus: summaryByField(tasks, func(t BgTask) string {
			if t.Status == "" {
				return noStatus
			}
			return t.Status
		}),
		SummaryBySubmitter: summaryByField(tasks, func(t BgTask) string {
			if t.Submitter == "" {
				return noSubmitter
			}
			return t.Submitter
		}),
		SummaryByWarningCount: summaryByField(tasks, func(t BgTask) string {
			return fmt.Sprintf("%d warnings", t.WarningCount)
		}),
		TopProjectBranchExecTimes: [3]BranchExecutionTimeSeries{
			getBranchExecTimeSeries(tasks, 0),
			getBranchExecTimeSeries(tasks, 1),
			getBranchExecTimeSeries(tasks, 2),
		},
	}
}

func chartRange(dr DateRange, from, to time.Time) (time.Time, time.Time) {
	start := utcDate(dr.EarliestSubmission)
	end := utcDate(dr.LatestCompletion)
	if !from.IsZero() {
		start = utcDate(from)
	}
	if !to.IsZero() {
		end = utcDate(to)
	}
	return start, end
}

func countByDay(tasks []BgTask, include func(BgTask) bool) map[time.Time]int {
	m := make(map[time.Time]int)
	for _, t := range tasks {
		if include(t) {
			m[utcDate(t.SubmittedAt)]++
		}
	}
	return m
}

func zeroFill(m map[time.Time]int, start, end time.Time) map[time.Time]int {
	out := make(map[time.Time]int)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out[d] = m[d]
	}
	return out
}

func allDistinctTypes(tasks []BgTask) []string {
	seen := make(map[string]struct{})
	var types []string
	for _, t := range tasks {
		tp := t.Type
		if tp == "" {
			tp = noType
		}
		if _, ok := seen[tp]; !ok {
			seen[tp] = struct{}{}
			types = append(types, tp)
		}
	}
	sort.Strings(types)
	return types
}

func buildTasksPerDayByType(tasks []BgTask, allTypes []string) map[time.Time]map[string]int {
	m := make(map[time.Time]map[string]int)
	for _, t := range tasks {
		day := utcDate(t.SubmittedAt)
		if m[day] == nil {
			m[day] = make(map[string]int, len(allTypes))
		}
		tp := t.Type
		if tp == "" {
			tp = noType
		}
		m[day][tp]++
	}
	return m
}

func zeroFillByType(m map[time.Time]map[string]int, allTypes []string, start, end time.Time) map[time.Time]map[string]int {
	out := make(map[time.Time]map[string]int)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		row := make(map[string]int, len(allTypes))
		for _, tp := range allTypes {
			if dayData, ok := m[d]; ok {
				row[tp] = dayData[tp]
			}
		}
		out[d] = row
	}
	return out
}

func categorizeTime(tasks []BgTask, totalReport int, selector func(BgTask) float64) []TimeCategoryMetric {
	var values []float64
	for _, t := range tasks {
		if t.Type == typeReport {
			values = append(values, selector(t))
		}
	}
	result := make([]TimeCategoryMetric, len(timeThresholds))
	for i, thresh := range timeThresholds {
		var lower int
		if i > 0 {
			lower = timeThresholds[i-1].upperBoundSec
		}
		count := countInBucket(values, lower, thresh.upperBoundSec, i == 0)
		var pct float64
		if totalReport > 0 {
			pct = float64(count) / float64(totalReport) * 100
		}
		result[i] = TimeCategoryMetric{UpperBoundSec: thresh.upperBoundSec, Label: thresh.label, Count: count, Percentage: pct}
	}
	return result
}

func countInBucket(values []float64, lower, upper int, inclusive bool) int {
	lo, up := float64(lower), float64(upper)
	count := 0
	for _, v := range values {
		if inclusive {
			if v >= lo && v <= up {
				count++
			}
		} else {
			if v > lo && v <= up {
				count++
			}
		}
	}
	return count
}

func summaryByField(tasks []BgTask, key func(BgTask) string) map[string]int {
	m := make(map[string]int)
	for _, t := range tasks {
		m[key(t)]++
	}
	return m
}

func getBranchExecTimeSeries(tasks []BgTask, rank int) BranchExecutionTimeSeries {
	projectKey, ok := rankedBranchProject(tasks, rank)
	if !ok {
		return BranchExecutionTimeSeries{ProjectRank: rank}
	}

	projectTasks := branchTasksForProject(tasks, projectKey)

	branchName, ok := topBranchName(projectTasks)
	if !ok {
		return BranchExecutionTimeSeries{ProjectRank: rank, ProjectKey: projectKey}
	}

	points := buildExecPoints(projectTasks, branchName)
	return BranchExecutionTimeSeries{
		ProjectRank:           rank,
		ProjectKey:            projectKey,
		BranchName:            branchName,
		ExecutionTimesSeconds: points,
	}
}

func rankedBranchProject(tasks []BgTask, rank int) (string, bool) {
	counts := make(map[string]int)
	for _, t := range tasks {
		if t.Type == typeReport && t.BranchType == "BRANCH" {
			key := t.ComponentKey
			if key == "" {
				key = noProjectKey
			}
			counts[key]++
		}
	}
	if len(counts) == 0 {
		return "", false
	}
	type entry struct {
		key   string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for k, c := range counts {
		entries = append(entries, entry{k, c})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].key < entries[j].key
	})
	if rank >= len(entries) {
		return "", false
	}
	return entries[rank].key, true
}

func branchTasksForProject(tasks []BgTask, projectKey string) []BgTask {
	var out []BgTask
	for _, t := range tasks {
		if t.Type != typeReport || t.BranchType != "BRANCH" {
			continue
		}
		key := t.ComponentKey
		if key == "" {
			key = noProjectKey
		}
		if key == projectKey {
			out = append(out, t)
		}
	}
	return out
}

func topBranchName(tasks []BgTask) (string, bool) {
	counts := make(map[string]int)
	for _, t := range tasks {
		counts[t.Branch]++
	}
	if len(counts) == 0 {
		return "", false
	}
	type entry struct {
		name  string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for b, c := range counts {
		entries = append(entries, entry{b, c})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].name < entries[j].name
	})
	return entries[0].name, true
}

func buildExecPoints(tasks []BgTask, branchName string) []ExecPoint {
	var filtered []BgTask
	for _, t := range tasks {
		if t.Branch == branchName {
			filtered = append(filtered, t)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartedAt.Before(filtered[j].StartedAt)
	})
	points := make([]ExecPoint, len(filtered))
	for i, t := range filtered {
		label := t.StartedAt.UTC().Format("2006-01-02 15:04:05")
		points[i] = ExecPoint{Label: label, Seconds: float64(t.ExecutionTimeMs) / 1000.0}
	}
	return points
}
