// Package otelinit installs the three globals every OTel-enabled micra
// service needs: the TracerProvider, the W3C TextMapPropagator (so
// traceparent + baggage cross service boundaries), and a sampler.
// Setup does NOT construct an exporter; the service builds an OTLP
// (or vendor-specific) exporter and a TracerProvider, then passes the
// provider here.
package otelinit

import (
	"context"
	"errors"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Setup installs the global TracerProvider, W3C TextMapPropagator
// (TraceContext + Baggage), and applies the sampler when the provider
// is an SDK TracerProvider. Returns a shutdown func that is safe to
// call any number of times; it forwards the first shutdown call to
// any provider with a Shutdown(context.Context) error method.
func Setup(ctx context.Context, opts ...Option) (shutdown func(context.Context) error, err error) {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.provider == nil {
		return nil, errors.New("otelinit: WithTracerProvider is required")
	}

	otel.SetTracerProvider(cfg.provider)
	if cfg.meterProvider != nil {
		otel.SetMeterProvider(cfg.meterProvider)
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	shutdowners := []any{cfg.provider}
	if cfg.meterProvider != nil {
		shutdowners = append(shutdowners, cfg.meterProvider)
	}
	var once sync.Once
	var shutdownErr error
	return func(ctx context.Context) error {
		once.Do(func() {
			for _, p := range shutdowners {
				if s, ok := p.(interface{ Shutdown(context.Context) error }); ok {
					if e := s.Shutdown(ctx); e != nil && shutdownErr == nil {
						shutdownErr = e
					}
				}
			}
		})
		return shutdownErr
	}, nil
}
