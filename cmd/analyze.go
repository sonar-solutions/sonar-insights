package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/analyzer"
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Read collected data from disk and generate an HTML report",
	Long:  "Reads previously collected SonarQube data from a local directory and generates a static HTML report. Never connects to SonarQube.",
	RunE:  runAnalyzeCmd,
}

var analyzeBgtasksCmd = &cobra.Command{
	Use:   "bgtasks",
	Short: "Analyze collected background task data and generate an HTML report",
	RunE:  runAnalyzeBgtasksCmd,
}

const flagReportDir = "report-dir"

func init() {
	analyzeCmd.Flags().String(flagDataDir, "./sonar-data/", "directory containing collected data")
	analyzeCmd.Flags().String(flagReportDir, "./sonar-reports/", "directory where reports are written")

	analyzeBgtasksCmd.Flags().String("from", "", "include tasks submitted on or after this date (YYYY-MM-DD, UTC)")
	analyzeBgtasksCmd.Flags().String("to", "", "include tasks submitted on or before this date (YYYY-MM-DD, UTC)")
	analyzeBgtasksCmd.Flags().String("report-name", "report-bgtasks", "output report filename (without .html extension)")
	analyzeBgtasksCmd.Flags().IntSlice("estimate-workers", nil, "estimate analyses per hour for these worker counts (comma-separated, e.g. 4,8,16)")

	analyzeCmd.AddCommand(analyzeBgtasksCmd)
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyzeCmd(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString(flagDataDir)
	reportDir, _ := cmd.Flags().GetString(flagReportDir)
	return runAnalyze(knownTargets(), dir, reportDir, "", "")
}

func runAnalyzeBgtasksCmd(cmd *cobra.Command, _ []string) error {
	dir, _ := cmd.Parent().Flags().GetString(flagDataDir)
	reportDir, _ := cmd.Parent().Flags().GetString(flagReportDir)
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	reportName, _ := cmd.Flags().GetString("report-name")
	estimateWorkers, _ := cmd.Flags().GetIntSlice("estimate-workers")
	if err := validateReportName(reportName); err != nil {
		return err
	}
	if err := validateEstimateWorkers(estimateWorkers); err != nil {
		return err
	}
	return runAnalyze([]string{"bgtasks"}, dir, reportDir, from, to, withReportName(reportName), withEstimateWorkers(estimateWorkers))
}

func validateReportName(name string) error {
	if name == "" {
		return fmt.Errorf("--report-name must not be empty")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("--report-name must not contain path separators: %q", name)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("--report-name must not begin with '.': %q", name)
	}
	return nil
}

func validateEstimateWorkers(workers []int) error {
	for _, w := range workers {
		if w < 1 {
			return fmt.Errorf("--estimate-workers: worker counts must be >= 1, got %d", w)
		}
	}
	return nil
}

type analyzeOptions struct {
	reportName      string
	estimateWorkers []int
}

type analyzeOption func(*analyzeOptions)

func withReportName(name string) analyzeOption {
	return func(o *analyzeOptions) { o.reportName = name }
}

func withEstimateWorkers(workers []int) analyzeOption {
	return func(o *analyzeOptions) { o.estimateWorkers = workers }
}

func runAnalyze(targets []string, dir, reportDir, from, to string, opts ...analyzeOption) error {
	o := analyzeOptions{reportName: "report-bgtasks"}
	for _, opt := range opts {
		opt(&o)
	}

	fromTime, err := parseOptionalDate(from)
	if err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	toTime, err := parseOptionalDate(to)
	if err != nil {
		return fmt.Errorf("--to: %w", err)
	}

	for _, target := range targets {
		switch target {
		case "bgtasks":
			if err := analyzer.AnalyzeBgTasks(dir, reportDir, o.reportName, fromTime, toTime, logger, o.estimateWorkers); err != nil {
				return fmt.Errorf("analyze bgtasks: %w", err)
			}
		default:
			return fmt.Errorf("unknown target: %s", target)
		}
	}
	return nil
}

func parseOptionalDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q (expected YYYY-MM-DD): %w", s, err)
	}
	return &t, nil
}
