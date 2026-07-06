// Package interceptors holds the built-in gRPC interceptors that
// components/grpcserver installs ahead of any user-supplied ones.
package interceptors

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/siabroo/micra/core"
)

// requestIDKey identifies the header micra uses for request id.
const requestIDKey = "x-request-id"

// RequestID extracts x-request-id from incoming metadata, generates a
// UUIDv4 if absent, appends to outgoing metadata for downstream calls,
// and tags the logger in ctx with requestId + method.
func RequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		rid := extractOrGenerate(ctx)
		ctx = metadata.AppendToOutgoingContext(ctx, requestIDKey, rid)

		base := core.LoggerFrom(ctx)
		tagged := base.With("requestId", rid, "method", info.FullMethod)
		ctx = core.ContextWithLogger(ctx, tagged)

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
