// Package httpserver implements core.Component over net/http.Server.
// Intended for /metrics, /healthz, /debug/pprof — the library does not
// provide routing helpers; the user supplies an http.Handler.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// Server wraps an http.Server with micra lifecycle.
type Server struct {
	cfg config
	srv *http.Server
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

// Start implements core.Component. Blocks until Stop is called.
func (s *Server) Start(ctx context.Context) error {
	if s.cfg.addr == "" {
		return errors.New("httpserver: addr is empty (use WithAddr)")
	}
	if s.cfg.handler == nil {
		return errors.New("httpserver: handler is nil (use WithHandler)")
	}
	s.srv = &http.Server{
		Addr:         s.cfg.addr,
		Handler:      s.cfg.handler,
		ReadTimeout:  s.cfg.readTimeout,
		WriteTimeout: s.cfg.writeTimeout,
		IdleTimeout:  s.cfg.idleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("httpserver: listen: %w", err)
	case <-ctx.Done():
		// ctx cancelled — Stop is responsible for calling Shutdown.
		// Wait for ListenAndServe to return after Shutdown.
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
