package core

import (
	"context"
	"testing"
)

func TestNoOpLogger_DiscardsEverything(t *testing.T) {
	l := NewNoOpLogger()

	// Should not panic on any method.
	l.Debug("d", "k", "v")
	l.Info("i", "k", "v")
	l.Warn("w", "k", "v")
	l.Error("e", "k", "v")

	// With returns a Logger that also discards.
	l2 := l.With("a", 1)
	if l2 == nil {
		t.Fatal("With returned nil")
	}
	l2.Info("still discarded")

	// Enabled returns false for every level.
	for _, lvl := range []Level{LevelDebug, LevelInfo, LevelWarn, LevelError} {
		if l.Enabled(lvl) {
			t.Errorf("NoOpLogger.Enabled(%d) = true, want false", lvl)
		}
	}
}

func TestLoggerFrom_ReturnsNoOpWhenEmpty(t *testing.T) {
	ctx := context.Background()
	l := LoggerFrom(ctx)
	if l == nil {
		t.Fatal("LoggerFrom returned nil")
	}
	// Must not panic — NoOp by contract.
	l.Info("ok")
	if l.Enabled(LevelInfo) {
		t.Error("expected NoOp logger to report Enabled=false")
	}
}

// stubLogger captures the With call so we can verify ContextWithLogger
// stores exactly what was given, no wrapping.
type stubLogger struct{ tag string }

func (s *stubLogger) Debug(string, ...any)   {}
func (s *stubLogger) Info(string, ...any)    {}
func (s *stubLogger) Warn(string, ...any)    {}
func (s *stubLogger) Error(string, ...any)   {}
func (s *stubLogger) With(...any) Logger     { return s }
func (s *stubLogger) Enabled(Level) bool     { return true }

func TestContextWithLogger_RoundTrip(t *testing.T) {
	stub := &stubLogger{tag: "stub"}
	ctx := ContextWithLogger(context.Background(), stub)
	got, ok := LoggerFrom(ctx).(*stubLogger)
	if !ok {
		t.Fatalf("LoggerFrom returned %T, want *stubLogger", LoggerFrom(ctx))
	}
	if got.tag != "stub" {
		t.Errorf("got tag %q, want %q", got.tag, "stub")
	}
}
