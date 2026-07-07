// Package interceptors holds the built-in gRPC interceptors that
// components/grpcserver installs ahead of any user-supplied ones.
package interceptors

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siabroo/micra/core"
)

// requestIDKey identifies the header micra uses for request id.
const requestIDKey = "x-request-id"

// sessionIDBaggageKey is the OTel Baggage member carrying the caller's
// session id. Pass-through only: micra never mints it.
const sessionIDBaggageKey = "session.id"

// RequestID extracts x-request-id from incoming metadata (generating a
// UUIDv4 if absent), appends it to outgoing metadata for downstream
// calls, enriches the active span with correlation ids, and tags the
// ctx logger with requestId + method + traceId/spanId/traceSampled, plus
// sessionId when a session.id baggage member is present.
func RequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		rid := extractOrGenerate(ctx)
		ctx = metadata.AppendToOutgoingContext(ctx, requestIDKey, rid)

		sid := baggage.FromContext(ctx).Member(sessionIDBaggageKey).Value()

		// Enrich the active span (set by otelgrpc's StatsHandler upstream).
		if span := trace.SpanFromContext(ctx); span.IsRecording() {
			span.SetAttributes(attribute.String("request.id", rid))
			if sid != "" {
				span.SetAttributes(attribute.String("session.id", sid))
			}
		}

		// Tag the ctx logger: requestId + method (as before), plus
		// traceId/spanId (for log->trace pivot), traceSampled (the span's real
		// sampling decision, so a backend can render correct log<->trace
		// correlation), and sessionId when present.
		fields := []any{"requestId", rid, "method", info.FullMethod}
		if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
			fields = append(fields,
				"traceId", sc.TraceID().String(),
				"spanId", sc.SpanID().String(),
				"traceSampled", sc.IsSampled(),
			)
		}
		if sid != "" {
			fields = append(fields, "sessionId", sid)
		}
		ctx = core.ContextWithLogger(ctx, core.LoggerFrom(ctx).With(fields...))

		return handler(ctx, req)
	}
}

func extractOrGenerate(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get(requestIDKey); len(v) > 0 && v[0] != "" {
			return v[0]
		}
	}
	return uuid.NewString()
}
