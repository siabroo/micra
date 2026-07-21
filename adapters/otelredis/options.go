package otelredis

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/siabroo/micra/adapters/otelredis"

type config struct {
	application    string
	tracerProvider trace.TracerProvider
}

// Option configures a Redis client wrapper via WrapClient.
type Option func(*config)

func defaults() config {
	return config{
		tracerProvider: nil, // resolved lazily to otel.GetTracerProvider()
	}
}

// WithApplication sets a static application name recorded as the
// "db.client.application" span attribute. Usually the service name.
// Omitted if empty.
func WithApplication(name string) Option {
	return func(c *config) { c.application = name }
}

// WithTracerProvider overrides the global otel.GetTracerProvider() for
// span emission. Useful in tests; production services configure the
// global provider via adapters/otelinit and leave this unset.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *config) { c.tracerProvider = tp }
}

func (c *config) tracer() trace.Tracer {
	tp := c.tracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	return tp.Tracer(instrumentationName)
}
