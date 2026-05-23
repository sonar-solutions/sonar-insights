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
