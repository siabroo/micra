package interceptors

import (
	"context"
	"fmt"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siabroo/micra/core"
)

// internalError is the fixed status returned to callers when a handler panics.
// Keeping it generic prevents leaking internal variable contents, file paths,
// or runtime error strings to untrusted clients.
var internalError = status.Error(codes.Internal, "internal error")

// Recovery converts panics from unary handlers into Internal gRPC errors and
// logs them with a stack trace server-side.
func Recovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				core.LoggerFrom(ctx).Error("panic in handler",
					"method", info.FullMethod,
					"panic", fmt.Sprintf("%v", r),
					"stack", string(debug.Stack()),
				)
				err = internalError
			}
		}()
		return handler(ctx, req)
	}
}

// StreamRecovery converts panics from streaming handlers into Internal gRPC
// errors and logs them with a stack trace server-side. It mirrors Recovery
// but satisfies grpc.StreamServerInterceptor.
func StreamRecovery() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				core.LoggerFrom(ss.Context()).Error("panic in stream handler",
					"method", info.FullMethod,
					"panic", fmt.Sprintf("%v", r),
					"stack", string(debug.Stack()),
				)
				err = internalError
			}
		}()
		return handler(srv, ss)
	}
}
