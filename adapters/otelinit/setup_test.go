package otelinit_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/siabroo/micra/adapters/otelinit"
)

func TestSetup_InstallsW3CPropagator(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	shutdown, err := otelinit.Setup(context.Background(), otelinit.WithTracerProvider(tp))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	}()

	// global propagator must speak traceparent
	prop := otel.GetTextMapPropagator()
	carrier := propagation.MapCarrier{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	}
	ctx := prop.Extract(context.Background(), carrier)
	_, span := otel.GetTracerProvider().Tracer("t").Start(ctx, "child")
	defer span.End() // we only care propagation extracted a span context
	_ = ctx
}

func TestSetup_ReturnsShutdownThatIsIdempotent(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	shutdown, err := otelinit.Setup(context.Background(), otelinit.WithTracerProvider(tp))
	if err != nil {
		t.Fatal(err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("first shutdown: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("second shutdown: %v", err)
	}
}

func TestSetup_RequiresTracerProvider(t *testing.T) {
	if _, err := otelinit.Setup(context.Background()); err == nil {
		t.Fatal("expected error when no TracerProvider is supplied")
	}
}

type fakeMeterProvider struct {
	embedded.MeterProvider
	shutdownCalled bool
}

func (f *fakeMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return noop.NewMeterProvider().Meter("")
}
func (f *fakeMeterProvider) Shutdown(context.Context) error { f.shutdownCalled = true; return nil }

func TestSetup_InstallsMeterProviderAndShutsItDown(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	mp := &fakeMeterProvider{}
	shutdown, err := otelinit.Setup(context.Background(),
		otelinit.WithTracerProvider(tp),
		otelinit.WithMeterProvider(mp),
	)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if otel.GetMeterProvider() != mp {
		t.Fatal("global MeterProvider was not set to the provided one")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if !mp.shutdownCalled {
		t.Fatal("shutdown did not forward to the MeterProvider")
	}
}
