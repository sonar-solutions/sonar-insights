// Command sonar-insights collects data from a SonarQube instance and renders
// it into self-contained HTML reports. See the README for usage.
package main

import (
	"os"

	"github.com/sonar-solutions/sonar-insights/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
