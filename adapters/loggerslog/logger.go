// Package loggerslog adapts the stdlib *slog.Logger to core.Logger.
package loggerslog

import (
	"context"
	"log/slog"
	"runtime"
	"time"

	"github.com/siabroo/micra/core"
)

// New wraps an *slog.Logger as core.Logger. Subsequent With calls
// produce new core.Logger instances backed by slog.Logger.With.
func New(l *slog.Logger) core.Logger { return &logger{l: l} }

type logger struct {
	l *slog.Logger
}

// log records at the given level using slog.NewRecord with an explicit
// program counter that points at the call site of the wrapping core
// method (Debug/Info/Warn/Error), not the wrapper itself. Without
// this, slog.HandlerOptions{AddSource: true} would always report this
// file as the source — useless to consumers.
//
// The skip value is 3: this frame + the wrapping method + runtime.Callers.
// See log/slog package docs "Wrapping output methods".
func (a *logger) log(level slog.Level, msg string, args ...any) {
	ctx := context.Background()
	if !a.l.Enabled(ctx, level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = a.l.Handler().Handle(ctx, r)
}

func (a *logger) Debug(msg string, args ...any) { a.log(slog.LevelDebug, msg, args...) }
func (a *logger) Info(msg string, args ...any)  { a.log(slog.LevelInfo, msg, args...) }
func (a *logger) Warn(msg string, args ...any)  { a.log(slog.LevelWarn, msg, args...) }
func (a *logger) Error(msg string, args ...any) { a.log(slog.LevelError, msg, args...) }

func (a *logger) With(args ...any) core.Logger { return &logger{l: a.l.With(args...)} }

func (a *logger) Enabled(level core.Level) bool {
	return a.l.Enabled(context.Background(), slog.Level(level))
}
