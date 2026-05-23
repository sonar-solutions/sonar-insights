package cmd

import (
	"fmt"
	"os"

	"github.com/sonar-solutions/sonar-insights/internal/collector"
	"github.com/spf13/cobra"
)

var collectCmd = &cobra.Command{
	Use:   "collect [targets...]",
	Short: "Fetch raw data from SonarQube and write to a local directory",
	Long:  "Connects to a SonarQube server, fetches data for the given targets, and writes it to disk. Never generates reports.",
	RunE:  runCollectCmd,
}

func init() {
	collectCmd.Flags().String("url", "", "SonarQube base URL (env: SONAR_HOST_URL, default: https://sonarcloud.io)")
	collectCmd.Flags().String("token", "", "SonarQube authentication token (env: SONAR_TOKEN)")
	collectCmd.Flags().String("out-dir", "./sonar-data/", "directory to write collected data")
	rootCmd.AddCommand(collectCmd)
}

func runCollectCmd(cmd *cobra.Command, args []string) error {
	url, _ := cmd.Flags().GetString("url")
	token, _ := cmd.Flags().GetString("token")
	outDir, _ := cmd.Flags().GetString("out-dir")
	return runCollect(args, url, token, outDir)
}

func runCollect(targets []string, url, token, outDir string) error {
	if url == "" {
		url = os.Getenv("SONAR_HOST_URL")
	}
	if url == "" {
		url = "https://sonarcloud.io"
	}
	if token == "" {
		token = os.Getenv("SONAR_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("--token or SONAR_TOKEN is required")
	}

	if len(targets) == 0 {
		targets = knownTargets()
	}

	for _, target := range targets {
		switch target {
		case "bgtasks":
			if err := collector.CollectBgTasks(url, token, outDir, logger); err != nil {
				return fmt.Errorf("collect bgtasks: %w", err)
			}
		default:
			return fmt.Errorf("unknown target: %s", target)
		}
	}
	return nil
}
