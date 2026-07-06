// Package httpserver implements core.Component over net/http.Server.
// Intended for /metrics, /healthz, /debug/pprof — the library does not
// provide routing helpers; the user supplies an http.Handler.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// Server wraps an http.Server with micra lifecycle.
type Server struct {
	cfg config

	lis net.Listener // created in Init
	srv *http.Server // created in Init
}

// New constructs a Server. WithAddr and WithHandler are required.
func New(opts ...Option) *Server {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}
	return &Server{cfg: cfg}
}

// Name implements core.Component.
func (s *Server) Name() string { return s.cfg.name }

// Init implements core.Initializer: validates config, opens the TCP listener,
// and constructs the http.Server. Errors here abort startup fail-fast before
// any Start goroutine is launched.
//
// By moving construction here (rather than inside Start), s.srv is assigned
// exactly once, sequentially, before any concurrent goroutine can read it.
// This eliminates the data race between Start's former assignment and Stop's
// nil-check.
func (s *Server) Init(_ context.Context) error {
	if s.cfg.addr == "" {
		return errors.New("httpserver: addr is empty (use WithAddr)")
	}
	if s.cfg.handler == nil {
		return errors.New("httpserver: handler is nil (use WithHandler)")
	}

	lis, err := net.Listen("tcp", s.cfg.addr)
	if err != nil {
		return fmt.Errorf("httpserver: listen %s: %w", s.cfg.addr, err)
	}
	s.lis = lis
	s.srv = &http.Server{
		Addr:              s.cfg.addr,
		Handler:           s.cfg.handler,
		ReadHeaderTimeout: s.cfg.readHeaderTimeout,
		ReadTimeout:       s.cfg.readTimeout,
		WriteTimeout:      s.cfg.writeTimeout,
		IdleTimeout:       s.cfg.idleTimeout,
	}
	return nil
}

// Start implements core.Component. Blocks until Stop is called or the server
// fails. Init must be called before Start.
func (s *Server) Start(ctx context.Context) error {
	if s.srv == nil {
		return errors.New("httpserver: Init must be called before Start")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.srv.Serve(s.lis)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("httpserver: serve: %w", err)
	case <-ctx.Done():
		// ctx cancelled — Stop is responsible for calling Shutdown.
		// Wait for Serve to return after Shutdown.
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Stop implements core.Component.
func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	// Honour the smaller of caller's ctx and our shutdownGrace.
	stopCtx, cancel := context.WithTimeout(ctx, s.cfg.shutdownGrace)
	defer cancel()
	return s.srv.Shutdown(stopCtx)
}
