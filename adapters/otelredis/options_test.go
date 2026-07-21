package otelredis

import (
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestWithApplication(t *testing.T) {
	cfg := configWith(WithApplication("svc"))
	if cfg.application != "svc" {
		t.Errorf("application = %q, want %q", cfg.application, "svc")
	}
}

func TestWithTracerProviderOverridesGlobal(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	cfg := configWith(WithTracerProvider(tp))
	if cfg.tracerProvider != tp {
		t.Errorf("tracerProvider not set to override")
	}
	// tracer() should use the override, not the global.
	if got := cfg.tracer(); got == nil {
		t.Errorf("tracer() returned nil")
	}
}

func TestTracerFallsBackToGlobal(t *testing.T) {
	cfg := defaults()
	if cfg.tracerProvider != nil {
		t.Fatalf("default tracerProvider should be nil (lazy)")
	}
	// tracer() resolves to the global provider without panicking.
	if got := cfg.tracer(); got == nil {
		t.Errorf("tracer() returned nil for global provider")
	}
	_ = otel.GetTracerProvider()
}
