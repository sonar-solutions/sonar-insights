package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/collector"
	"github.com/sonar-solutions/sonar-insights/internal/sonarqube"
	"github.com/spf13/cobra"
)

var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Fetch raw data from SonarQube and write to a local directory",
	Long:  "Connects to a SonarQube server, fetches data for the given targets, and writes it to disk. Never generates reports.",
	RunE:  runCollectCmd,
}

var collectBgtasksCmd = &cobra.Command{
	Use:   "bgtasks",
	Short: "Collect background task data from SonarQube",
	RunE:  runCollectBgtasksCmd,
}

const flagDataDir = "data-dir"

func init() {
	collectCmd.Flags().String("url", "", "SonarQube base URL (env: SONAR_HOST_URL)")
	collectCmd.Flags().String("token", "", "SonarQube authentication token (env: SONAR_TOKEN)")
	collectCmd.Flags().String(flagDataDir, "./sonar-data/", "directory to write collected data")
	collectCmd.Flags().Int("parallel", 5, "number of pages to fetch concurrently")

	collectCmd.AddCommand(collectBgtasksCmd)
	rootCmd.AddCommand(collectCmd)
}

func runCollectCmd(cmd *cobra.Command, _ []string) error {
	url, _ := cmd.Flags().GetString("url")
	token, _ := cmd.Flags().GetString("token")
	outDir, _ := cmd.Flags().GetString(flagDataDir)
	parallel, _ := cmd.Flags().GetInt("parallel")
	return runCollect(knownTargets(), url, token, outDir, parallel)
}

func runCollectBgtasksCmd(cmd *cobra.Command, _ []string) error {
	parent := cmd.Parent()
	url, _ := parent.Flags().GetString("url")
	token, _ := parent.Flags().GetString("token")
	outDir, _ := parent.Flags().GetString(flagDataDir)
	parallel, _ := parent.Flags().GetInt("parallel")
	return runCollect([]string{"bgtasks"}, url, token, outDir, parallel)
}

func runCollect(targets []string, url, token, outDir string, parallel int) error {
	if url == "" {
		url = os.Getenv("SONAR_HOST_URL")
	}
	if url == "" {
		return fmt.Errorf("--url or SONAR_HOST_URL is required")
	}
	if token == "" {
		token = os.Getenv("SONAR_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("--token or SONAR_TOKEN is required")
	}

	client := sonarqube.NewHTTPClient()
	instance, err := sonarqube.Detect(url, token, client)
	if err != nil {
		return fmt.Errorf("detect sonarqube instance: %w", err)
	}
	logger.Debug(fmt.Sprintf("detected SonarQube Server version: %s", instance.Version))

	if err := prepareOutputDir(outDir); err != nil {
		return err
	}

	if err := writeCollectMetadata(instance, targets, outDir); err != nil {
		return fmt.Errorf("write collect metadata: %w", err)
	}

	for _, target := range targets {
		switch target {
		case "bgtasks":
			if err := collector.CollectBgTasks(instance, outDir, parallel, logger); err != nil {
				return fmt.Errorf("collect bgtasks: %w", err)
			}
		default:
			return fmt.Errorf("unknown target: %s", target)
		}
	}
	return nil
}

func prepareOutputDir(outDir string) error {
	cleaned := filepath.Clean(outDir)
	if cleaned == "/" || cleaned == "." || cleaned == ".." || cleaned == os.Getenv("HOME") {
		return fmt.Errorf("refusing to remove dangerous path: %s", outDir)
	}
	if err := os.RemoveAll(outDir); err != nil {
		return fmt.Errorf("remove output directory: %w", err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	return nil
}

type collectMetadata struct {
	SonarQubeURL        string   `json:"sonarqubeURL"`
	CollectionTimestamp string   `json:"collectionTimestamp"`
	SonarQubeVersion    *string  `json:"sonarqubeVersion"`
	Targets             []string `json:"targets"`
}

func writeCollectMetadata(instance sonarqube.SonarInstance, targets []string, outDir string) error {
	var version *string
	if instance.Product == sonarqube.Server {
		v := instance.Version
		version = &v
	}

	metadata := collectMetadata{
		SonarQubeURL:        instance.BaseURL,
		CollectionTimestamp: time.Now().UTC().Format(time.RFC3339),
		SonarQubeVersion:    version,
		Targets:             targets,
	}

	data, err := json.MarshalIndent(metadata, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	return os.WriteFile(filepath.Join(outDir, "collect-metadata.json"), data, 0o644)
}
