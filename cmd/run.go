package cmd

import (
	"github.com/spf13/cobra"
)

var (
	runURL        string
	runToken      string
	runDir        string
	runReportName string
)

var runCmd = &cobra.Command{
	Use:   "run [targets...]",
	Short: "Collect data from SonarQube then generate an HTML report",
	Long:  "Convenience wrapper that runs collect followed by analyze in sequence.",
	RunE:  runRunCmd,
}

func init() {
	runCmd.Flags().StringVar(&runURL, "url", "", "SonarQube base URL (env: SONAR_HOST_URL, default: https://sonarcloud.io)")
	runCmd.Flags().StringVar(&runToken, "token", "", "SonarQube authentication token (env: SONAR_TOKEN)")
	runCmd.Flags().StringVar(&runDir, "dir", "./sonar-data/", "directory for collected data")
	runCmd.Flags().StringVar(&runReportName, "report-name", "sonar-insights-report", "output report name (without extension)")
	rootCmd.AddCommand(runCmd)
}

func runRunCmd(cmd *cobra.Command, args []string) error {
	if err := runCollect(args, runURL, runToken, runDir); err != nil {
		return err
	}
	return runAnalyze(args, runDir, runReportName)
}
