package core

import (
	"fmt"
	"strings"
)

// ParseLevel converts a textual level name to a Level. The accepted
// names match slog's UnmarshalText conventions and are
// case-insensitive: "debug", "info", "warn" (or "warning"), "error".
//
// Returns LevelInfo and a non-nil error for unrecognised input so
// callers can decide whether to fall back to a default or fail fast.
// Adapters built on top of slog/zap/etc. should accept this Level
// type rather than re-implement a parser.
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug, nil
	case "info", "":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("core: unknown log level %q", s)
	}
}
