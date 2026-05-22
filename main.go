package main

import (
	"os"

	"github.com/lfrystak/sonar-insights/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
