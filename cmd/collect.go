package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	if err := writeCollectMetadata(instance, outDir); err != nil {
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

// prepareOutputDir ensures outDir exists without deleting any existing content.
// It rejects paths that are too shallow or known to be dangerous (system roots, home dir).
func prepareOutputDir(outDir string) error {
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return fmt.Errorf("resolve output directory path: %w", err)
	}
	if err := checkDangerousPath(abs); err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	return nil
}

func checkDangerousPath(abs string) error {
	if pathDepth(abs) < 2 {
		return fmt.Errorf("refusing to use shallow path as output directory: %s", abs)
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if abs == filepath.Clean(home) {
			return fmt.Errorf("refusing to use home directory as output directory: %s", abs)
		}
	}

	for _, sysRoot := range []string{"/usr", "/etc", "/var", "/opt", "/bin", "/sbin"} {
		if abs == filepath.Clean(sysRoot) {
			return fmt.Errorf("refusing to use system directory as output directory: %s", abs)
		}
	}

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		cleanGopath := filepath.Clean(gopath)
		if abs == cleanGopath || strings.HasPrefix(abs, cleanGopath+string(filepath.Separator)) {
			return fmt.Errorf("refusing to use GOPATH directory as output directory: %s", abs)
		}
	}

	if exe, err := os.Executable(); err == nil {
		if abs == filepath.Dir(exe) {
			return fmt.Errorf("refusing to use executable directory as output directory: %s", abs)
		}
	}

	return nil
}

// pathDepth counts the number of non-empty path components below the volume root.
// e.g. "/foo" → 1, "/foo/bar" → 2, "C:\foo\bar" → 2.
func pathDepth(absPath string) int {
	vol := filepath.VolumeName(absPath)
	rel := absPath[len(vol):]
	parts := strings.Split(filepath.ToSlash(rel), "/")
	count := 0
	for _, p := range parts {
		if p != "" {
			count++
		}
	}
	return count
}

type collectMetadata struct {
	SonarQubeURL        string  `json:"sonarqubeURL"`
	CollectionTimestamp string  `json:"collectionTimestamp"`
	SonarQubeVersion    *string `json:"sonarqubeVersion"`
}

func writeCollectMetadata(instance sonarqube.SonarInstance, outDir string) error {
	var version *string
	if instance.Product == sonarqube.Server {
		v := instance.Version
		version = &v
	}

	metadata := collectMetadata{
		SonarQubeURL:        instance.BaseURL,
		CollectionTimestamp: time.Now().UTC().Format(time.RFC3339),
		SonarQubeVersion:    version,
	}

	data, err := json.MarshalIndent(metadata, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	return os.WriteFile(filepath.Join(outDir, "collect-metadata.json"), data, 0o644)
}
