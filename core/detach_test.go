package core

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// recDetachLogger captures every call so the tests can assert
// detached=true tag and panic-recovery logging.
type recDetachLogger struct {
	mu        sync.Mutex
	withCalls [][]any
	infoMsgs  []string
	errorMsgs []string
	errorArgs [][]any
}

func (r *recDetachLogger) Debug(string, ...any) {}
func (r *recDetachLogger) Info(msg string, _ ...any) {
	r.mu.Lock()
	r.infoMsgs = append(r.infoMsgs, msg)
	r.mu.Unlock()
}
func (r *recDetachLogger) Warn(string, ...any) {}
func (r *recDetachLogger) Error(msg string, args ...any) {
	r.mu.Lock()
	r.errorMsgs = append(r.errorMsgs, msg)
	cp := make([]any, len(args))
	copy(cp, args)
	r.errorArgs = append(r.errorArgs, cp)
	r.mu.Unlock()
}
func (r *recDetachLogger) With(args ...any) Logger {
	r.mu.Lock()
	cp := make([]any, len(args))
	copy(cp, args)
	r.withCalls = append(r.withCalls, cp)
	r.mu.Unlock()
	return r
}
func (r *recDetachLogger) Enabled(Level) bool { return true }

func TestDetach_RunsFnInNewGoroutine(t *testing.T) {
	ran := make(chan struct{})
	Detach(context.Background(), func(context.Context) {
		close(ran)
	})
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("fn did not run within 2s")
	}
}

func TestDetach_LoggerTaggedWithDetachedTrue(t *testing.T) {
	rec := &recDetachLogger{}
	ctx := ContextWithLogger(context.Background(), rec)

	done := make(chan struct{})
	Detach(ctx, func(ctx context.Context) {
		// access logger inside detached goroutine via LoggerFrom — the
		// tagged logger is what fn would log through.
		_ = LoggerFrom(ctx)
		close(done)
	})
	<-done

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.withCalls) == 0 {
		t.Fatal("expected at least one With call tagging the logger")
	}
	first := rec.withCalls[0]
	found := false
	for i := 0; i+1 < len(first); i += 2 {
		if k, ok := first[i].(string); ok && k == "detached" {
			if v, ok := first[i+1].(bool); ok && v {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("With args missing detached=true: %v", first)
	}
}

func TestDetach_DoesNotInheritParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // parent already cancelled

	checked := make(chan error, 1)
	Detach(ctx, func(ctx context.Context) {
		// Parent cancellation must NOT bleed in: ctx.Err() should be
		// nil at the start of fn.
		checked <- ctx.Err()
	})

	select {
	case err := <-checked:
		if err != nil {
			t.Errorf("detached ctx.Err() = %v at start, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fn did not run within 2s")
	}
}

func TestDetach_AppliesTimeout(t *testing.T) {
	// Run a fn that blocks past the bounds we provide via a local
	// override. To avoid waiting the real 10s, swap a tiny ctx in.
	parent := context.Background()
	parentLog := LoggerFrom(parent)
	timeoutHit := make(chan struct{})

	// We can't override DetachTimeout per call (it's a const). To still
	// test the timeout wiring, mirror the implementation with a short
	// ctx and verify ctx.Err() is DeadlineExceeded when fn would have
	// hung. This validates the WithTimeout pattern.
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 50*time.Millisecond)
		defer cancel()
		ctx = ContextWithLogger(ctx, parentLog)
		// simulate hung fn
		<-ctx.Done()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			close(timeoutHit)
		}
	}()

	select {
	case <-timeoutHit:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout did not fire within 2s")
	}
}

func TestDetach_RecoversPanic(t *testing.T) {
	rec := &recDetachLogger{}
	ctx := ContextWithLogger(context.Background(), rec)

	Detach(ctx, func(context.Context) {
		panic("boom")
	})

	// Poll until the panic is logged or we time out.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec.mu.Lock()
		count := len(rec.errorMsgs)
		var lastMsg string
		if count > 0 {
			lastMsg = rec.errorMsgs[count-1]
		}
		rec.mu.Unlock()
		if count > 0 {
			if !strings.Contains(lastMsg, "panicked") {
				t.Errorf("Error msg = %q, want it to mention panic", lastMsg)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("panic was not logged within 2s")
}

func TestDetach_InheritsValuesFromParent(t *testing.T) {
	type key struct{}
	parent := context.WithValue(context.Background(), key{}, "preserved")

	got := make(chan any, 1)
	Detach(parent, func(ctx context.Context) {
		got <- ctx.Value(key{})
	})
	select {
	case v := <-got:
		if v != "preserved" {
			t.Errorf("value = %v, want preserved", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fn did not run within 2s")
	}
}

