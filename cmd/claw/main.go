// Package main is the entry point for the claw binary.
// It delegates immediately to the CLI command tree.
package main

import (
	"context"
	"os"

	"github.com/machinae/betterclaw/internal/cli"
	"github.com/machinae/betterclaw/internal/logging"
)

func main() {
	if err := cli.NewRootCmd().ExecuteContext(context.Background()); err != nil {
		logging.Logger().Error("fatal error", "err", err)
		os.Exit(1)
	}
}
