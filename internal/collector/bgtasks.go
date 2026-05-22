package collector

import (
	"fmt"
	"log/slog"
)

// CollectBgTasks fetches background task data from SonarQube and writes it to outDir.
func CollectBgTasks(url, token, outDir string, logger *slog.Logger) error {
	logger.Info(fmt.Sprintf("[collect] bgtasks: would fetch from %s", url))
	// TODO: GET /api/ce/activity and write JSON results to outDir/bgtasks.json
	return nil
}
