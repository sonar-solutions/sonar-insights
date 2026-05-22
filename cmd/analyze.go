package cmd

import (
	"fmt"

	"github.com/lfrystak/sonar-insights/internal/analyzer"
	"github.com/spf13/cobra"
)

var (
	analyzeDir        string
	analyzeReportName string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [targets...]",
	Short: "Read collected data from disk and generate an HTML report",
	Long:  "Reads previously collected SonarQube data from a local directory and generates a static HTML report. Never connects to SonarQube.",
	RunE:  runAnalyzeCmd,
}

func init() {
	analyzeCmd.Flags().StringVar(&analyzeDir, "dir", "./sonar-data/", "directory containing collected data")
	analyzeCmd.Flags().StringVar(&analyzeReportName, "report-name", "sonar-insights-report", "output report name (without extension)")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyzeCmd(cmd *cobra.Command, args []string) error {
	return runAnalyze(args, analyzeDir, analyzeReportName)
}

func runAnalyze(targets []string, dir, reportName string) error {
	if len(targets) == 0 {
		targets = knownTargets()
	}

	for _, target := range targets {
		switch target {
		case "bgtasks":
			if err := analyzer.AnalyzeBgTasks(dir, reportName, logger); err != nil {
				return fmt.Errorf("analyze bgtasks: %w", err)
			}
		default:
			return fmt.Errorf("unknown target: %s", target)
		}
	}
	return nil
}
