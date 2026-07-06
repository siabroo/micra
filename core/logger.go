// Package core defines micra's lifecycle primitives: App, Component,
// Initializer, and the Logger interface. See micra's README for an
// overview and docs/superpowers/specs/2026-06-04-go-service-runtime-design.md
// in the parent monorepo for the design rationale.
package core

import "context"

// Level mirrors the levels recognised by slog. Values match slog's
// integer levels so adapters can pass through with minimal arithmetic.
type Level int

// Log-level constants corresponding to slog.Level values.
const (
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
)

// Logger is the minimal interface micra needs from any logger.
// Implementations must be safe for concurrent use. Implementations
// should accept attrs in slog-style alternating key/value pairs
// (k1, v1, k2, v2, ...).
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)

	// With returns a Logger that includes the given attrs in every log
	// line, in addition to those of the receiver.
	With(args ...any) Logger

	// Enabled reports whether the Logger will record a message at the
	// given level. Used by interceptors to skip expensive formatting
	// (e.g. payload truncation) when the logger would discard the line.
	Enabled(level Level) bool
}

// NewNoOpLogger returns a Logger that discards all input and reports
// Enabled = false for every level. Used as the default when WithLogger
// is not set on an App. The returned value is a shared sentinel — no
// allocation per call.
func NewNoOpLogger() Logger { return noopLoggerSingleton }

type noopLogger struct{}

var noopLoggerSingleton Logger = noopLogger{}

func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}
func (noopLogger) With(...any) Logger   { return noopLoggerSingleton }
func (noopLogger) Enabled(Level) bool   { return false }

// loggerCtxKey is the unexported key under which Loggers are stored in
// context. Using a struct type makes accidental key collisions
// impossible.
type loggerCtxKey struct{}

// ContextWithLogger returns a copy of ctx that carries the given Logger.
// Used by App to propagate the service+version-tagged logger into each
// Component's Start ctx, and by the grpcserver interceptor to attach
// per-RPC tags.
func ContextWithLogger(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// LoggerFrom returns the Logger attached to ctx, or a NoOp logger if
// none is attached. Never panics. The canonical accessor for
// Component implementations and handler code.
func LoggerFrom(ctx context.Context) Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(Logger); ok {
		return l
	}
	return NewNoOpLogger()
}
