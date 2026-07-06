package otelpgx

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceparentFromContext_NoSpan_ReturnsEmpty(t *testing.T) {
	if got := traceparentFromContext(context.Background()); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestTraceparentFromContext_ValidSpanContext(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	got := traceparentFromContext(ctx)
	want := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRouteFromBaggage_MissingKey_ReturnsEmpty(t *testing.T) {
	if got := routeFromBaggage(context.Background()); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestRouteFromBaggage_PresentKey_ReturnsValue(t *testing.T) {
	m, _ := baggage.NewMember(routeBaggageKey, "/auth.Auth/SignIn")
	bg, _ := baggage.New(m)
	ctx := baggage.ContextWithBaggage(context.Background(), bg)
	if got := routeFromBaggage(ctx); got != "/auth.Auth/SignIn" {
		t.Fatalf("got %q", got)
	}
}
