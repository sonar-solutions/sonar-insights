// Package analyzer orchestrates the analysis stage of the sonar-insights
// pipeline. It reads data previously written by the collector, dispatches to
// target-specific implementations such as the bgtasks subpackage, and writes
// the resulting HTML report to disk. The analyzer never connects to a
// SonarQube instance.
package analyzer

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/analyzer/bgtasks"
)

// AnalyzeBgTasks loads background task data from dir/bgtasks/, runs analysis, and writes
// the report to reportDir/reportName.html. from and to optionally bound the date filter.
// When estimateWorkers is non-empty, a capacity estimate is logged after the report is written.
func AnalyzeBgTasks(dir, reportDir, reportName string, from, to *time.Time, logger *slog.Logger, estimateWorkers []int) error {
	logger.Info("analyzing background tasks")

	dataDir := filepath.Join(dir, "bgtasks")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return fmt.Errorf("bgtasks data directory not found: %s", dataDir)
	}

	tasks, err := bgtasks.Load(dataDir, logger)
	if err != nil {
		return fmt.Errorf("load bgtasks: %w", err)
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no tasks found in %s", dataDir)
	}
	logger.Info(fmt.Sprintf("loaded %d unique background tasks", len(tasks)))

	totalBeforeFilter := len(tasks)

	if from != nil || to != nil {
		tasks = filterByDate(tasks, from, to)
		logger.Info(fmt.Sprintf("%d tasks remain after date filtering", len(tasks)))
		if len(tasks) == 0 {
			return fmt.Errorf("no tasks matched the specified date range")
		}
	}

	var fromTime, toTime time.Time
	if from != nil {
		fromTime = *from
	}
	if to != nil {
		toTime = *to
	}

	dr := bgtasks.AnalyzeDateRange(tasks)
	overall := bgtasks.AnalyzeOverall(tasks, totalBeforeFilter, dr)
	pm := bgtasks.AnalyzeProjectAnalysis(tasks, dr)
	charts := bgtasks.AnalyzeCharts(tasks, dr, pm, fromTime, toTime)

	nonIssueSync := filterType(tasks, "ISSUE_SYNC")
	cd := bgtasks.CalculateCapacityDemand(nonIssueSync, dr.EarliestSubmission, dr.LatestCompletion)

	results := bgtasks.AnalysisResults{
		DateRange:       dr,
		Overall:         overall,
		ProjectAnalysis: pm,
		Charts:          charts,
		CapacityDemand:  cd,
	}

	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}

	report := bgtasks.BuildReport(results)
	reportPath := filepath.Join(reportDir, reportName+".html")
	if err := bgtasks.RenderToFile(report, reportPath); err != nil {
		return fmt.Errorf("render report: %w", err)
	}
	logger.Info(fmt.Sprintf("report written to %s", reportPath))

	if len(estimateWorkers) > 0 {
		logCapacityEstimate(tasks, estimateWorkers, logger)
	}
	return nil
}

// logCapacityEstimate runs the capacity estimator and logs all results.
// It never returns an error — failures are logged and ignored so report generation is unaffected.
func logCapacityEstimate(tasks []bgtasks.BgTask, workerCounts []int, logger *slog.Logger) {
	est, skipped, reason := bgtasks.EstimateCapacity(tasks, workerCounts)
	if skipped {
		logger.Info(fmt.Sprintf("capacity estimate: %s", reason))
		return
	}

	// Step 0 — ISSUE_SYNC exclusion
	logger.Debug(fmt.Sprintf("capacity estimate step 0: excluded %d ISSUE_SYNC tasks, %d non-ISSUE_SYNC tasks remain",
		est.ExcludedCount, est.NonSyncCount))

	// Step 1 — REPORT isolation
	logger.Debug(fmt.Sprintf("capacity estimate step 1: %d REPORT tasks isolated", est.ReportCount))

	// Step 2 — report share (sums at DEBUG, headline at INFO)
	logger.Debug(fmt.Sprintf("capacity estimate step 2: Σ REPORT ms=%d, Σ non-ISSUE_SYNC ms=%d",
		est.TotalReportMs, est.TotalNonSyncMs))
	logger.Info(fmt.Sprintf("capacity estimate: REPORT (project analysis) accounts for %.1f%% of non-ISSUE_SYNC compute time",
		est.ReportShare*100))

	// Step 2b — demand profile
	logDemandProfile(est.Demand, logger)

	// Step 3 — category shares
	for _, c := range est.WorkerEstimates[0].Categories {
		logger.Debug(fmt.Sprintf("capacity estimate step 3: %-18s tasks=%d  sum_ms=%d  share=%.2f%%",
			c.Label, c.BucketCount, c.BucketSumMs, c.Share*100))
	}
	logger.Debug(fmt.Sprintf("capacity estimate step 3: XXXL max observed = %dms", est.MaxObservedMs))

	// Step 4 — representative costs (mean)
	for _, c := range est.WorkerEstimates[0].Categories {
		floor := ""
		if c.FloorApplied {
			floor = " (floor applied)"
		}
		logger.Debug(fmt.Sprintf("capacity estimate step 4: %-18s mean=%.3fs%s",
			c.Label, c.RepresentativeSec, floor))
	}

	// Step 5 — per-worker capacity and verdicts
	for _, we := range est.WorkerEstimates {
		logWorkerEstimate(we, est.Demand, est.ReportShare, logger)
	}
}

func logDemandProfile(d bgtasks.DemandProfile, logger *slog.Logger) {
	logger.Debug(fmt.Sprintf("capacity estimate step 2b: observedHours=%d  avgDemandPerHour=%.1f  peakDemandPerHour=%.1f (p95)  busiestHour=%d",
		d.ObservedHours, d.AvgPerHour, d.PeakPerHour, d.BusiestHourCount))
	if d.Available {
		days := d.ObservedHours / 24
		logger.Info(fmt.Sprintf("capacity estimate: REPORT demand averaged ≈ %.0f analyses/hour, peaking at ≈ %.0f/hour (over %d days)",
			d.AvgPerHour, d.PeakPerHour, days))
	} else {
		logger.Info(fmt.Sprintf("capacity estimate: REPORT demand averaged ≈ %.0f analyses/hour (insufficient span for peak estimate: %d hours observed)",
			d.AvgPerHour, d.ObservedHours))
	}
}

func logWorkerEstimate(we bgtasks.WorkerEstimate, demand bgtasks.DemandProfile, reportShare float64, logger *slog.Logger) {
	capacitySec := float64(we.Workers) * secondsPerHour
	reportCapacitySec := capacitySec * reportShare
	logger.Debug(fmt.Sprintf("capacity estimate step 5: workers=%d  capacityPerHour=%.0fs  reportCapacity=%.0fs",
		we.Workers, capacitySec, reportCapacitySec))
	for _, c := range we.Categories {
		logger.Debug(fmt.Sprintf("capacity estimate step 5: workers=%d  %-18s catCapacity=%.1fs",
			we.Workers, c.Label, c.CatCapacitySec))
	}

	logger.Info(fmt.Sprintf("capacity estimate (per hour) for %d workers: capacity ≈ %d analyses/hour",
		we.Workers, iround(we.TotalJobs)))

	line := "  "
	for _, c := range we.Categories {
		if c.BucketCount == 0 {
			line += fmt.Sprintf("%s: no tasks   ", c.Label)
		} else {
			line += fmt.Sprintf("%s: ≈ %d   ", c.Label, iround(c.Jobs))
		}
	}
	logger.Info(line)

	logger.Info(fmt.Sprintf("  verdict: %s", bgtasks.VerdictFor(we.TotalJobs, demand)))
}

const secondsPerHour = 3600

func iround(f float64) int {
	return int(math.Round(f))
}

func filterByDate(tasks []bgtasks.BgTask, from, to *time.Time) []bgtasks.BgTask {
	out := tasks[:0]
	for _, t := range tasks {
		submitted := t.SubmittedAt.UTC()
		if from != nil && submitted.Before(from.UTC()) {
			continue
		}
		if to != nil && submitted.After(toEndOfDay(*to)) {
			continue
		}
		out = append(out, t)
	}
	return out
}

func toEndOfDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 23, 59, 59, 0, time.UTC)
}

func filterType(tasks []bgtasks.BgTask, excludeType string) []bgtasks.BgTask {
	out := make([]bgtasks.BgTask, 0, len(tasks))
	for _, t := range tasks {
		if t.Type != excludeType {
			out = append(out, t)
		}
	}
	return out
}
