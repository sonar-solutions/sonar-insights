package analyzer

import (
	"fmt"
	"log/slog"

	"github.com/sonar-solutions/sonar-insights/internal/reporter"
)

// AnalyzeBgTasks reads collected background task data from dir and contributes to the report.
func AnalyzeBgTasks(dir, reportName string, logger *slog.Logger) error {
	logger.Info(fmt.Sprintf("[analyze] bgtasks: would read from %s", dir))
	// TODO: read bgtasks.json from dir, compute metrics, build report sections

	if err := reporter.Generate(reportName, logger); err != nil {
		return fmt.Errorf("generate report: %w", err)
	}
	return nil
}
