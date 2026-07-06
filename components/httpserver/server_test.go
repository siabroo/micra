package httpserver_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/siabroo/micra/components/httpserver"
)

// pickFreePort returns a port number that is free at this moment.
func pickFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// TestServer_ImplementsComponentAndInitializer verifies the interface contract.
func TestServer_ImplementsComponentAndInitializer(t *testing.T) {
	s := httpserver.New(httpserver.WithAddr("127.0.0.1:0"), httpserver.WithHandler(http.NotFoundHandler()))
	type component interface {
		Name() string
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
	}
	type initializer interface {
		Init(ctx context.Context) error
	}
	if _, ok := any(s).(component); !ok {
		t.Error("*Server does not implement core.Component")
	}
	if _, ok := any(s).(initializer); !ok {
		t.Error("*Server does not implement core.Initializer")
	}
}

func TestServer_ServesAndStops(t *testing.T) {
	port := pickFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hi"))
	})

	s := httpserver.New(httpserver.WithAddr(addr), httpserver.WithHandler(mux))

	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startDone := make(chan error, 1)
	go func() { startDone <- s.Start(ctx) }()

	// Server is already listening (Init opened the listener), but wait briefly
	// for Serve to accept connections.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("tcp", addr, 50*time.Millisecond); err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	resp, err := http.Get("http://" + addr + "/hi")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "hi" {
		t.Errorf("body = %q, want %q", string(body), "hi")
	}

	cancel()
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
