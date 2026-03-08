package main

import (
	"os"

	"github.com/maxitosh/katapult/internal/cli"
)

// @cpt-dod:cpt-katapult-dod-api-cli-cli-tool:p1

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
