// White-box tests for the httpserver package. Runs in package httpserver so
// it can inspect unexported fields (s.srv.ReadHeaderTimeout, s.lis).
package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// TestServer_ImplementsInitializer verifies that *Server satisfies the
// core.Initializer duck-type interface (Init(context.Context) error).
func TestServer_ImplementsInitializer(t *testing.T) {
	s := New(WithAddr("127.0.0.1:0"), WithHandler(http.NotFoundHandler()))
	type initializer interface {
		Init(ctx context.Context) error
	}
	if _, ok := any(s).(initializer); !ok {
		t.Error("*Server does not implement Initializer")
	}
}

// TestInit_RequiresAddr verifies that Init returns an error when no address
// is configured.
func TestInit_RequiresAddr(t *testing.T) {
	s := New(WithHandler(http.NotFoundHandler()))
	if err := s.Init(context.Background()); err == nil {
		t.Error("Init: expected error for missing addr, got nil")
	}
}

// TestInit_RequiresHandler verifies that Init returns an error when no handler
// is configured.
func TestInit_RequiresHandler(t *testing.T) {
	s := New(WithAddr(fmt.Sprintf("127.0.0.1:%d", freePort(t))))
	if err := s.Init(context.Background()); err == nil {
		t.Error("Init: expected error for missing handler, got nil")
	}
}

// TestInit_ReadHeaderTimeout_Default verifies that the http.Server built by
// Init has ReadHeaderTimeout set to the 5s default even when the caller has
// not provided WithReadHeaderTimeout.
func TestInit_ReadHeaderTimeout_Default(t *testing.T) {
	s := New(
		WithAddr(fmt.Sprintf("127.0.0.1:%d", freePort(t))),
		WithHandler(http.NotFoundHandler()),
	)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = s.lis.Close() })

	const want = 5 * time.Second
	if got := s.srv.ReadHeaderTimeout; got != want {
		t.Errorf("ReadHeaderTimeout = %v, want %v", got, want)
	}
}

// TestInit_ReadHeaderTimeout_Custom verifies that WithReadHeaderTimeout
// propagates to the http.Server built by Init.
func TestInit_ReadHeaderTimeout_Custom(t *testing.T) {
	const want = 3 * time.Second
	s := New(
		WithAddr(fmt.Sprintf("127.0.0.1:%d", freePort(t))),
		WithHandler(http.NotFoundHandler()),
		WithReadHeaderTimeout(want),
	)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = s.lis.Close() })

	if got := s.srv.ReadHeaderTimeout; got != want {
		t.Errorf("ReadHeaderTimeout = %v, want %v", got, want)
	}
}

// TestInit_ReadHeaderTimeout_ZeroWhenReadTimeoutZero verifies that setting
// WithReadHeaderTimeout(0) explicitly works (e.g. for tests), and independently
// verifies the SSE use-case: WithReadTimeout(0)+WithReadHeaderTimeout(5s) sets
// only ReadHeaderTimeout while ReadTimeout stays 0.
func TestInit_ReadHeaderTimeout_IndependentOfReadTimeout(t *testing.T) {
	const wantHeader = 5 * time.Second
	s := New(
		WithAddr(fmt.Sprintf("127.0.0.1:%d", freePort(t))),
		WithHandler(http.NotFoundHandler()),
		WithReadTimeout(0),
		WithReadHeaderTimeout(wantHeader),
	)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = s.lis.Close() })

	if got := s.srv.ReadHeaderTimeout; got != wantHeader {
		t.Errorf("ReadHeaderTimeout = %v, want %v", got, wantHeader)
	}
	if got := s.srv.ReadTimeout; got != 0 {
		t.Errorf("ReadTimeout = %v, want 0 (SSE use-case)", got)
	}
}

// TestServer_StopAfterInit_NoRace verifies that calling Stop() immediately
// after Init() — before Start() — is safe and returns nil. Under -race this
// exercises any synchronization on s.srv between Init and Stop.
func TestServer_StopAfterInit_NoRace(t *testing.T) {
	s := New(
		WithAddr(fmt.Sprintf("127.0.0.1:%d", freePort(t))),
		WithHandler(http.NotFoundHandler()),
	)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Stop(stopCtx); err != nil {
		t.Errorf("Stop after Init (no Start): %v", err)
	}
}

// TestServer_PreCancelledContext_NoRace is the key race regression test.
//
// Scenario: the lifecycle manager cancels the context before (or just as)
// Start() begins, so Stop() may run concurrently with Start()'s initialisation.
//
// With the OLD code (s.srv assigned inside Start()), running this under
// `go test -race` would flag a data race: Start writes s.srv while Stop reads
// it, with no synchronisation between them.
//
// With the NEW code (Init() assigns s.srv synchronously before any goroutine),
// Start() only READS s.srv, so there is no concurrent write and the race
// detector stays silent.
func TestServer_PreCancelledContext_NoRace(t *testing.T) {
	s := New(
		WithAddr(fmt.Sprintf("127.0.0.1:%d", freePort(t))),
		WithHandler(http.NotFoundHandler()),
	)
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel: context is done before Start runs

	startDone := make(chan error, 1)
	go func() { startDone <- s.Start(ctx) }()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := s.Stop(stopCtx); err != nil {
		t.Errorf("Stop: %v", err)
	}

	select {
	case err := <-startDone:
		if err != nil {
			t.Errorf("Start returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return within 2s after Stop")
	}
}
