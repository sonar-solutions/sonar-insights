package cmd

import (
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Collect data from SonarQube then generate an HTML report",
	Long:  "Convenience wrapper that runs collect followed by analyze in sequence.",
	RunE:  runRunCmd,
}

var runBgtasksCmd = &cobra.Command{
	Use:   "bgtasks",
	Short: "Collect and analyze background task data",
	RunE:  runRunBgtasksCmd,
}

func init() {
	runCmd.Flags().String("url", "", "SonarQube base URL (env: SONAR_HOST_URL, default: https://sonarcloud.io)")
	runCmd.Flags().String("token", "", "SonarQube authentication token (env: SONAR_TOKEN)")
	runCmd.Flags().String(flagOutDir, "./sonar-data/", "directory to write collected data")
	runCmd.Flags().Int("parallel", 5, "number of pages to fetch concurrently")
	runCmd.Flags().String("dir", "./sonar-data/", "directory containing collected data (for analyze step)")
	runCmd.Flags().String(flagReportDir, "./sonar-reports/", "directory where reports are written")

	runBgtasksCmd.Flags().String("from", "", "include tasks submitted on or after this date (YYYY-MM-DD, UTC)")
	runBgtasksCmd.Flags().String("to", "", "include tasks submitted on or before this date (YYYY-MM-DD, UTC)")
	runBgtasksCmd.Flags().String("report-name", "report-bgtasks", "output report filename (without .html extension)")

	runCmd.AddCommand(runBgtasksCmd)
	rootCmd.AddCommand(runCmd)
}

func runRunCmd(cmd *cobra.Command, _ []string) error {
	url, _ := cmd.Flags().GetString("url")
	token, _ := cmd.Flags().GetString("token")
	outDir, _ := cmd.Flags().GetString(flagOutDir)
	parallel, _ := cmd.Flags().GetInt("parallel")
	dir, _ := cmd.Flags().GetString("dir")
	reportDir, _ := cmd.Flags().GetString(flagReportDir)
	if err := runCollect(knownTargets(), url, token, outDir, parallel); err != nil {
		return err
	}
	return runAnalyze(knownTargets(), dir, reportDir, "", "")
}

func runRunBgtasksCmd(cmd *cobra.Command, _ []string) error {
	parent := cmd.Parent()
	url, _ := parent.Flags().GetString("url")
	token, _ := parent.Flags().GetString("token")
	outDir, _ := parent.Flags().GetString(flagOutDir)
	parallel, _ := parent.Flags().GetInt("parallel")
	dir, _ := parent.Flags().GetString("dir")
	reportDir, _ := parent.Flags().GetString(flagReportDir)
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")
	reportName, _ := cmd.Flags().GetString("report-name")
	if err := runCollect([]string{"bgtasks"}, url, token, outDir, parallel); err != nil {
		return err
	}
	return runAnalyze([]string{"bgtasks"}, dir, reportDir, from, to, withReportName(reportName))
}
