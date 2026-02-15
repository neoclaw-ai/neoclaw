package main

import (
	"os"

	"github.com/machinae/betterclaw/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
