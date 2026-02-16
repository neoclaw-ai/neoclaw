package main

import (
	"os"

	"github.com/machinae/betterclaw/internal/cli"
	"github.com/machinae/betterclaw/internal/logging"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		logging.Logger().Error("fatal error", "err", err)
		os.Exit(1)
	}
}
