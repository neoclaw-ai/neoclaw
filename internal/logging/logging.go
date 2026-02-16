package logging

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

var logger = slog.New(newHandler())

func newHandler() slog.Handler {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if isTerminal(os.Stderr) {
		return tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelInfo,
			TimeFormat: "15:04:05",
		})
	}
	return slog.NewTextHandler(os.Stderr, opts)
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// Logger returns the process logger.
func Logger() *slog.Logger {
	return logger
}
