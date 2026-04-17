// Package logging owns process-wide slog configuration for slack-orchestrator.
//
// Call Init exactly once from main immediately after loading config, before any
// other package logs. All code should use the standard library log/slog package
// (slog.Info, slog.Warn, etc.); the default handler is always JSON to stdout
// so container runtimes and UIs that only collect stdout still receive logs.
package logging

import (
	"log/slog"
	"os"
)

// Init configures slog's default logger: JSON lines, info level, always stdout.
func Init() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(h))
}
