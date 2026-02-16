package logging

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"golang.org/x/term"
)

var logger = slog.New(newHandler())

func newHandler() slog.Handler {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if isTerminal(os.Stderr) {
		return tint.NewHandler(os.Stderr, &tint.Options{
			Level:      slog.LevelInfo,
			TimeFormat: "15:04:05",
			ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
				if len(groups) > 0 || attr.Key != slog.LevelKey {
					return attr
				}
				level, ok := attr.Value.Any().(slog.Level)
				if !ok {
					return attr
				}
				switch {
				case level >= slog.LevelError:
					return tint.Attr(196, slog.Any(slog.LevelKey, level))
				case level >= slog.LevelWarn:
					return tint.Attr(208, slog.Any(slog.LevelKey, level))
				default:
					return attr
				}
			},
		})
	}
	return slog.NewTextHandler(os.Stderr, opts)
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// Logger returns the process logger.
func Logger() *slog.Logger {
	return logger
}
