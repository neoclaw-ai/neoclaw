package logging

import (
	"log/slog"
	"os"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// Logger returns the process logger.
func Logger() *slog.Logger {
	return logger
}
