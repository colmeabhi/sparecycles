// Package logging creates the structured logger for all binaries.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a text logger that writes to stderr.
// SC_LOG_LEVEL sets the level: debug, info (default), warn or error.
func New(component string) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(os.Getenv("SC_LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(h).With("component", component)
}
