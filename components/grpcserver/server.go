// Package grpcserver exposes a gRPC server as a micra
// core.Component + core.Initializer. The built-in interceptors
// (request-id, RPC logging, panic recovery) are installed ahead of
// any user-supplied ones. See the design spec §7.1.
package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/siabroo/micra/components/grpcserver/internal/interceptors"
	"github.com/siabroo/micra/core"
)

// Server wraps a *grpc.Server with micra lifecycle.
type Server struct {
	cfg config

	lis    net.Listener
	srv    *grpc.Server
	health *health.Server // non-nil when cfg.healthService; flipped in Stop
	srvErr chan error
}

// New constructs a Server. Either WithAddr or WithListener is
// required, plus WithRegister.
func New(opts ...Option) *Server {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}
	return &Server{cfg: cfg, srvErr: make(chan error, 1)}
}

// Name implements core.Component.
func (s *Server) Name() string { return s.cfg.name }

// Init implements core.Initializer: open the listener and build the
// grpc.Server (so the user's WithRegister can be invoked here too).
// Listen failures are reported here, fail-fast before any Start runs.
func (s *Server) Init(ctx context.Context) error {
	if s.cfg.register == nil {
		return errors.New("grpcserver: WithRegister is required")
	}

	lis := s.cfg.listener
	if lis == nil {
		if s.cfg.addr == "" {
			return errors.New("grpcserver: WithAddr or WithListener is required")
		}
		var err error
		lis, err = net.Listen("tcp", s.cfg.addr)
		if err != nil {
			return fmt.Errorf("grpcserver: listen %s: %w", s.cfg.addr, err)
		}
	}
	s.lis = lis

	// Seed the App-provided logger into every per-RPC context. gRPC
	// creates fresh request contexts from context.Background, so
	// without this bridge the downstream interceptors would see only
	// a NoOp logger and the RPC log lines would be discarded.
	appLogger := core.LoggerFrom(ctx)
	seedLogger := func(c context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		return h(core.ContextWithLogger(c, appLogger), req)
	}

	unary := []grpc.UnaryServerInterceptor{
		seedLogger,
		interceptors.RequestID(),
		interceptors.Recovery(),
		interceptors.RPCLog(interceptors.Config{
			PayloadLogging:  s.cfg.payloadLogging,
			PayloadMaxBytes: s.cfg.payloadMaxBytes,
		}),
	}
	unary = append(unary, s.cfg.unaryICs...)

	opts := []grpc.ServerOption{grpc.ChainUnaryInterceptor(unary...)}
	if len(s.cfg.streamICs) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(s.cfg.streamICs...))
	}
	opts = append(opts, s.cfg.serverOpts...)

	s.srv = grpc.NewServer(opts...)

	if s.cfg.healthService {
		h := health.NewServer()
		h.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
		for _, name := range s.cfg.healthServiceNames {
			h.SetServingStatus(name, grpc_health_v1.HealthCheckResponse_SERVING)
		}
		grpc_health_v1.RegisterHealthServer(s.srv, h)
		s.health = h
	}
	if s.cfg.reflection {
		reflection.Register(s.srv)
	}

	s.cfg.register(s.srv)

	core.LoggerFrom(ctx).Info("grpc server initialised",
		"name", s.cfg.name,
		"addr", lis.Addr().String(),
	)
	return nil
}

// Start implements core.Component: Serve until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	go func() {
		s.srvErr <- s.srv.Serve(s.lis)
	}()
	select {
	case err := <-s.srvErr:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		// Stop is responsible for GracefulStop; wait for serve to return.
		err := <-s.srvErr
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc serve: %w", err)
		}
		return nil
	}
}

// Stop implements core.Component: GracefulStop bounded by ctx,
// falling back to Stop on deadline.
//
// Before draining, the health service (if enabled) is flipped to
// NOT_SERVING so a gRPC readiness probe fails and the pod is pulled from
// the Service EndpointSlice ahead of the drain — avoiding the window where
// kube-proxy still routes new connections to a terminating pod. Pair with a
// preStop hook so the endpoint removal has time to propagate.
func (s *Server) Stop(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	if s.health != nil {
		s.health.Shutdown() // all statuses -> NOT_SERVING
	}
	done := make(chan struct{})
	go func() {
		s.srv.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.srv.Stop()
		<-done
		return ctx.Err()
	}
}
