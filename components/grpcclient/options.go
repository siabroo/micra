package grpcclient

import "google.golang.org/grpc"

type config struct {
	name      string
	target    string
	dialOpts  []grpc.DialOption
	unaryICs  []grpc.UnaryClientInterceptor
	streamICs []grpc.StreamClientInterceptor
}

// Option configures Client via New.
type Option func(*config)

func defaults() config { return config{name: "grpc-client"} }

// WithName overrides default component name "grpc-client".
func WithName(name string) Option { return func(c *config) { c.name = name } }

// WithTarget sets the dial target (host:port, dns:///host:port,
// passthrough:///foo, etc.). Required.
func WithTarget(target string) Option { return func(c *config) { c.target = target } }

// WithDialOptions appends raw grpc.DialOption values.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(c *config) { c.dialOpts = append(c.dialOpts, opts...) }
}

// WithRoundRobin sets the default load-balancing policy to round_robin via
// the service config. Pair it with a headless-service dns:/// target so the
// client spreads RPCs across every backend pod (and re-resolves on rolling
// updates) instead of pinning to a single pod under gRPC's default
// pick_first policy — the common failure mode for long-lived HTTP/2
// connections behind a Kubernetes Service.
func WithRoundRobin() Option {
	return func(c *config) {
		c.dialOpts = append(c.dialOpts,
			grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	}
}

// WithUnaryInterceptors appends unary client interceptors (e.g.
// otelgrpc.UnaryClientInterceptor()).
func WithUnaryInterceptors(is ...grpc.UnaryClientInterceptor) Option {
	return func(c *config) { c.unaryICs = append(c.unaryICs, is...) }
}

// WithStreamInterceptors appends stream client interceptors.
func WithStreamInterceptors(is ...grpc.StreamClientInterceptor) Option {
	return func(c *config) { c.streamICs = append(c.streamICs, is...) }
}
