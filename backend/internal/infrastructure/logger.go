package infrastructure

import (
	"log/slog"
	"os"
)

// NewLogger returns the structured JSON logger and makes it the default.
func NewLogger() *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	return logger
}
