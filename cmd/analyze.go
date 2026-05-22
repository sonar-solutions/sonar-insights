package cmd

import (
	"fmt"

	"github.com/lfrystak/sonar-insights/internal/analyzer"
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [targets...]",
	Short: "Read collected data from disk and generate an HTML report",
	Long:  "Reads previously collected SonarQube data from a local directory and generates a static HTML report. Never connects to SonarQube.",
	RunE:  runAnalyzeCmd,
}

func init() {
	analyzeCmd.Flags().String("dir", "./sonar-data/", "directory containing collected data")
	analyzeCmd.Flags().String("report-name", "sonar-insights-report", "output report name (without extension)")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyzeCmd(cmd *cobra.Command, args []string) error {
	dir, _ := cmd.Flags().GetString("dir")
	reportName, _ := cmd.Flags().GetString("report-name")
	return runAnalyze(args, dir, reportName)
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
