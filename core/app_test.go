package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestApp_RegisterValidatesUniqueNames(t *testing.T) {
	app := New(WithoutSignals())
	a := &fakeComponent{name: "x"}
	b := &fakeComponent{name: "x"}
	if err := app.Register(a); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := app.Register(b); !errors.Is(err, ErrDuplicateComponent) {
		t.Errorf("second register: got %v, want ErrDuplicateComponent", err)
	}
}

func TestApp_RunOnce_CallsFnWithLoggerInCtx(t *testing.T) {
	app := New(WithName("test-svc"), WithoutSignals())

	var seen Logger
	err := app.RunOnce(context.Background(), func(ctx context.Context) error {
		seen = LoggerFrom(ctx)
		return nil
	})
	if err != nil {
		t.Fatalf("RunOnce err: %v", err)
	}
	if seen == nil {
		t.Fatal("fn did not receive a logger in ctx")
	}
}

func TestApp_Run_FailsFastOnStartError(t *testing.T) {
	bErr := errors.New("boom")
	app := New(WithoutSignals(), WithStopTimeout(200*time.Millisecond))
	if err := app.Register(&fakeComponent{name: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := app.Register(&fakeComponent{name: "b", startErr: bErr}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := app.Run(ctx)
	if !errors.Is(err, bErr) {
		t.Errorf("Run returned %v, want chain containing %v", err, bErr)
	}
}

func TestApp_DoubleRunReturnsErrAlreadyStarted(t *testing.T) {
	app := New(WithoutSignals())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediate exit
	_ = app.Run(ctx)
	if err := app.Run(context.Background()); !errors.Is(err, ErrAppAlreadyStarted) {
		t.Errorf("second Run returned %v, want ErrAppAlreadyStarted", err)
	}
	if err := app.RunOnce(context.Background(), func(context.Context) error { return nil }); !errors.Is(err, ErrAppAlreadyStarted) {
		t.Errorf("RunOnce after Run returned %v, want ErrAppAlreadyStarted", err)
	}
}

func TestApp_LoggerCarriesServiceAndVersion(t *testing.T) {
	// We rely on the wrapper logger's With being called with the right
	// args. Use a recorder logger.
	rec := &recordingLogger{}
	app := New(WithName("svc-x"), WithVersion("v-y"), WithLogger(rec), WithoutSignals())
	if got, want := app.Name(), "svc-x"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := app.Version(), "v-y"; got != want {
		t.Errorf("Version = %q, want %q", got, want)
	}

	// app.Logger() should be the result of rec.With("service", "svc-x", "version", "v-y")
	// — i.e. the recorded With call.
	if len(rec.withCalls) != 1 {
		t.Fatalf("expected one With call on raw logger, got %d", len(rec.withCalls))
	}
	got := rec.withCalls[0]
	want := []any{"service", "svc-x", "version", "v-y"}
	if !sliceEq(got, want) {
		t.Errorf("With called with %v, want %v", got, want)
	}
}

func TestApp_RunOnce_FnError_StopsAndReturnsFnErr(t *testing.T) {
	app := New(WithoutSignals())
	c := &fakeComponent{name: "a"}
	if err := app.Register(c); err != nil {
		t.Fatal(err)
	}
	fnErr := errors.New("fn boom")
	err := app.RunOnce(context.Background(), func(ctx context.Context) error {
		return fnErr
	})
	if !errors.Is(err, fnErr) {
		t.Errorf("got %v, want chain containing %v", err, fnErr)
	}
	if !c.stopCalled {
		t.Error("Stop was not called")
	}
}

// recordingLogger captures With calls so the test can verify what tags
// App attached.
type recordingLogger struct {
	withCalls [][]any
}

func (r *recordingLogger) Debug(string, ...any) {}
func (r *recordingLogger) Info(string, ...any)  {}
func (r *recordingLogger) Warn(string, ...any)  {}
func (r *recordingLogger) Error(string, ...any) {}
func (r *recordingLogger) With(args ...any) Logger {
	cp := make([]any, len(args))
	copy(cp, args)
	r.withCalls = append(r.withCalls, cp)
	return r
}
func (r *recordingLogger) Enabled(Level) bool { return true }

func sliceEq(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

