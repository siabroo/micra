package grpcserver

import (
	"net"

	"google.golang.org/grpc"
)

type config struct {
	name               string
	addr               string
	listener           net.Listener // overrides addr if non-nil; used for bufconn tests
	register           func(*grpc.Server)
	unaryICs           []grpc.UnaryServerInterceptor
	streamICs          []grpc.StreamServerInterceptor
	serverOpts         []grpc.ServerOption
	reflection         bool
	healthService      bool
	healthServiceNames []string
	payloadLogging     bool
	payloadMaxBytes    int
}

// Option configures Server via New.
type Option func(*config)

func defaults() config {
	return config{
		name:            "grpc",
		reflection:      false, // SECURITY DEFAULT: reflection must be explicitly enabled via WithReflection(true)
		healthService:   true,
		payloadMaxBytes: 200,
	}
}

// WithName overrides default component name "grpc".
func WithName(name string) Option { return func(c *config) { c.name = name } }

// WithAddr sets the listen address (host:port). Required unless
// WithListener is used.
func WithAddr(addr string) Option { return func(c *config) { c.addr = addr } }

// WithListener overrides addr; used by tests with bufconn.
func WithListener(l net.Listener) Option { return func(c *config) { c.listener = l } }

// WithRegister is the callback invoked with the *grpc.Server during
// Init, after Listen and after built-in interceptors.
func WithRegister(fn func(*grpc.Server)) Option {
	return func(c *config) { c.register = fn }
}

// WithUnaryInterceptors appends user interceptors after the built-ins.
func WithUnaryInterceptors(is ...grpc.UnaryServerInterceptor) Option {
	return func(c *config) { c.unaryICs = append(c.unaryICs, is...) }
}

// WithStreamInterceptors appends user stream interceptors.
func WithStreamInterceptors(is ...grpc.StreamServerInterceptor) Option {
	return func(c *config) { c.streamICs = append(c.streamICs, is...) }
}

// WithGRPCServerOptions appends raw grpc.ServerOption values.
func WithGRPCServerOptions(opts ...grpc.ServerOption) Option {
	return func(c *config) { c.serverOpts = append(c.serverOpts, opts...) }
}

// WithReflection toggles grpc.reflection.Register. Default is false (secure
// default). Pass WithReflection(true) to enable reflection for development or
// debugging; do not enable in production unless you control access.
func WithReflection(enabled bool) Option { return func(c *config) { c.reflection = enabled } }

// WithHealthService toggles registration of the standard grpc health
// service (default true).
func WithHealthService(enabled bool) Option { return func(c *config) { c.healthService = enabled } }

// WithHealthServiceNames adds named services to the health server in
// addition to the empty-string "overall" name.
func WithHealthServiceNames(names ...string) Option {
	return func(c *config) { c.healthServiceNames = append(c.healthServiceNames, names...) }
}

// WithRPCPayloadLogging toggles whether the built-in RPC log
// interceptor emits debug-level request/response payload lines.
// Default: false (security default).
func WithRPCPayloadLogging(enabled bool) Option {
	return func(c *config) { c.payloadLogging = enabled }
}

// WithRPCPayloadMaxBytes sets the truncation length for payload
// strings logged at debug level. Default: 200.
func WithRPCPayloadMaxBytes(n int) Option {
	return func(c *config) { c.payloadMaxBytes = n }
}
