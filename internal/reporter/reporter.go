package reporter

import (
	"fmt"
	"log/slog"
)

// TODO: import github.com/lfrystak/rptgen once report implementation begins

// Generate builds and writes the HTML report to disk.
func Generate(reportName string, logger *slog.Logger) error {
	logger.Info(fmt.Sprintf("[reporter] would generate report: %s.html", reportName))
	// TODO: use rptgen to assemble sections and write the HTML report file
	return nil
}
