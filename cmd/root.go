package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var logger *slog.Logger
var timing bool
var startTime time.Time

var rootCmd = &cobra.Command{
	Use:   "sonar-insights",
	Short: "Provides insights into SonarQube usage",
	Long:  "sonar-insights collects data from SonarQube and generates static HTML reports.",
}

func Execute() error {
	return rootCmd.Execute()
}

func knownTargets() []string {
	return []string{"bgtasks"}
}

func init() {
	logger = slog.New(slog.NewTextHandler(os.Stderr, nil))

	rootCmd.PersistentFlags().BoolVar(&timing, "timing", false, "log total execution time on exit")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if timing {
			startTime = time.Now()
		}
	}

	rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		if timing {
			elapsed := time.Since(startTime)
			logger.Info(fmt.Sprintf("[timing] %s finished in %.2fs", cmd.Name(), elapsed.Seconds()))
		}
	}
}
