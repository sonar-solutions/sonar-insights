// Package analyzer orchestrates the analysis stage of the sonar-insights
// pipeline. It reads data previously written by the collector, dispatches to
// target-specific implementations such as the bgtasks subpackage, and writes
// the resulting HTML report to disk. The analyzer never connects to a
// SonarQube instance.
package analyzer

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/analyzer/bgtasks"
)

// AnalyzeBgTasks loads background task data from dir/bgtasks/, runs analysis, and writes
// the report to reportDir/reportName.html. from and to optionally bound the date filter.
func AnalyzeBgTasks(dir, reportDir, reportName string, from, to *time.Time, logger *slog.Logger) error {
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
	return nil
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
