package main

import (
	"os"

	"github.com/opendatahub-io/odh-platform-chaos/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
