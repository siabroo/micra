package otelinit

import (
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type config struct {
	provider trace.TracerProvider
	sampler  sdktrace.Sampler
	resource *resource.Resource
}

// Option configures Setup.
type Option func(*config)

func defaults() config {
	return config{
		sampler: sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.01)),
	}
}

// WithTracerProvider is required. Setup does not construct an exporter
// or a TracerProvider; the service builds one (OTLP, Cloud Trace,
// X-Ray, etc.) and passes it here. Setup installs it globally.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) { c.provider = tp }
}

// WithSampler overrides the default ParentBased(TraceIDRatioBased(0.01))
// sampler. The override only takes effect if the supplied
// TracerProvider is an sdktrace.TracerProvider; otherwise it is
// silently ignored (the user already chose their sampler when
// constructing their provider).
func WithSampler(s sdktrace.Sampler) Option {
	return func(c *config) { c.sampler = s }
}

// WithResource records a resource hint (currently unused; reserved
// for future setup of OTel SDK resource detection).
func WithResource(r *resource.Resource) Option {
	return func(c *config) { c.resource = r }
}
