// Package logging configures the process-wide structured logger (slog).
// Using slog gives machine-parseable JSON logs with consistent fields
// (including a trace_id) so requests can be correlated across log lines and
// with the usage_records table.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Init configures slog as the default logger. level accepts
// "debug"/"info"/"warn"/"error" (case-insensitive); anything else defaults to
// info. When json is false a human-readable text handler is used (handy for
// local development).
func Init(level string, json bool) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if json {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}
