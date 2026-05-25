// Package cmd wires the sonar-insights command-line interface together using
// cobra. It defines the root command and the collect, analyze, and run
// subcommand trees, resolves shared flags such as the SonarQube URL, token,
// and output directories, and dispatches into the internal analyzer and
// collector packages.
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
var verbose bool
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
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if verbose {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		}
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
