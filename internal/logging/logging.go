// Package logging configures structured logging for togglerino using slog.
package logging

import (
	"log/slog"
	"os"
)

// Setup configures the default slog logger based on the given format.
// If format is "text", a human-readable text handler is used.
// Otherwise (including "json" and empty string), a JSON handler is used.
func Setup(format string) {
	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, nil)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(handler))
}
