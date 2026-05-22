package cmd

import (
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [targets...]",
	Short: "Collect data from SonarQube then generate an HTML report",
	Long:  "Convenience wrapper that runs collect followed by analyze in sequence.",
	RunE:  runRunCmd,
}

func init() {
	runCmd.Flags().String("url", "", "SonarQube base URL (env: SONAR_HOST_URL, default: https://sonarcloud.io)")
	runCmd.Flags().String("token", "", "SonarQube authentication token (env: SONAR_TOKEN)")
	runCmd.Flags().String("dir", "./sonar-data/", "directory for collected data")
	runCmd.Flags().String("report-name", "sonar-insights-report", "output report name (without extension)")
	rootCmd.AddCommand(runCmd)
}

func runRunCmd(cmd *cobra.Command, args []string) error {
	url, _ := cmd.Flags().GetString("url")
	token, _ := cmd.Flags().GetString("token")
	dir, _ := cmd.Flags().GetString("dir")
	reportName, _ := cmd.Flags().GetString("report-name")
	if err := runCollect(args, url, token, dir); err != nil {
		return err
	}
	return runAnalyze(args, dir, reportName)
}
