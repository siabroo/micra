package otelpgx

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

// routeBaggageKey is the OTel baggage key whose value otelpgx reads to
// populate the sqlcommenter "route" field by default. Services stamp
// this at the request boundary (see README example).
const routeBaggageKey = "micra.route"

// traceparentFromContext returns the W3C traceparent string for the
// span in ctx, or "" if ctx has no valid span context. Format:
//
//	00-<32 hex trace id>-<16 hex span id>-<2 hex flags>
//
// The leading "00" is the W3C trace-context version byte.
func traceparentFromContext(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return fmt.Sprintf("00-%s-%s-%02x", sc.TraceID(), sc.SpanID(), byte(sc.TraceFlags()))
}

// routeFromBaggage returns the value of the routeBaggageKey baggage
// member in ctx, or "" if absent.
func routeFromBaggage(ctx context.Context) string {
	return baggage.FromContext(ctx).Member(routeBaggageKey).Value()
}
