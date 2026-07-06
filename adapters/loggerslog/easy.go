package loggerslog

import (
	"io"
	"log/slog"
	"os"

	"github.com/siabroo/micra/core"
)

// Format selects the slog handler used by NewSimple.
type Format string

const (
	// FormatJSON uses slog.NewJSONHandler. Suitable for production:
	// each log line is a single JSON object that structured log
	// collectors (Loki, Cloudwatch, Datadog) parse natively.
	FormatJSON Format = "json"

	// FormatText uses slog.NewTextHandler. Suitable for local development
	// — output is human-readable `time=... level=INFO msg=...` without
	// ANSI colour codes. Operators that want colour can build their own
	// *slog.Logger with a coloured handler (e.g. lmittmann/tint) and
	// pass it to New.
	//
	// The string "pretty" is accepted as an alias so deployment configs
	// that previously read "pretty" continue to work.
	FormatText Format = "text"

	// formatPrettyAlias is the legacy spelling. Treated as FormatText.
	formatPrettyAlias Format = "pretty"
)

// Config configures NewSimple. Zero-value yields a JSON logger at
// LevelInfo writing to os.Stdout with no source info.
type Config struct {
	Level  core.Level
	Format Format    // FormatJSON, FormatText, or "pretty" (alias of text)
	Output io.Writer // nil → os.Stdout

	// Source toggles slog.HandlerOptions.AddSource. When true, every
	// log line carries a "source" attribute with the file, line, and
	// function where the log call was made. Useful for tracing log
	// origin in dev/staging; adds a runtime.Caller per log call so
	// hot-path debug logging may want this off.
	Source bool
}

// NewSimple builds an *slog.Logger from cfg and wraps it as
// core.Logger. Use this when the only knobs you need are level and
// format; callers that need slog.HandlerOptions.ReplaceAttr, attribute
// groups, or a custom handler should construct an *slog.Logger
// themselves and pass it to New instead.
//
// Symmetry note: future micra logger adapters (e.g. loggerszap) expose
// the same NewSimple + Config + Format triple so that consumers can
// swap implementations without rewiring their bootstrap.
func NewSimple(cfg Config) core.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}
	opts := &slog.HandlerOptions{
		Level:     slog.Level(cfg.Level),
		AddSource: cfg.Source,
	}

	var h slog.Handler
	switch cfg.Format {
	case FormatText, formatPrettyAlias:
		h = slog.NewTextHandler(out, opts)
	default:
		h = slog.NewJSONHandler(out, opts)
	}
	return New(slog.New(h))
}
