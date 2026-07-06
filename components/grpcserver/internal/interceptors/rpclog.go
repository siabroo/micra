package interceptors

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siabroo/micra/core"
)

// Config controls the level-aware RPC log interceptor's payload
// behaviour. See spec §7.1.
type Config struct {
	PayloadLogging  bool // gates debug-level payload emission
	PayloadMaxBytes int  // truncate request/response payload strings
}

// RPCLog logs at info on rpc.end (always) and at debug on rpc.start +
// rpc.end (with payloads, gated by Config.PayloadLogging AND the
// logger's Enabled(Debug)).
//
// Assumes RequestID has already tagged the logger in ctx with
// "requestId" and "method"; rpc.start/end do not re-emit those keys
// to avoid duplicates in slog output.
func RPCLog(cfg Config) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		log := core.LoggerFrom(ctx)
		start := time.Now()

		emitDebug := cfg.PayloadLogging && log.Enabled(core.LevelDebug)
		if emitDebug {
			log.Debug("rpc.start",
				"req", truncate(fmt.Sprintf("%v", req), cfg.PayloadMaxBytes),
				"metadata", incomingMetadata(ctx),
			)
		}

		resp, err := handler(ctx, req)
		durMs := time.Since(start).Milliseconds()
		code := status.Code(err).String()

		end := []any{"code", code, "durationMs", durMs}
		if err != nil {
			end = append(end, "error", err.Error())
		}
		if emitDebug {
			end = append(end, "resp", truncate(fmt.Sprintf("%v", resp), cfg.PayloadMaxBytes))
		}
		log.Info("rpc.end", end...)
		return resp, err
	}
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func incomingMetadata(ctx context.Context) any {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	return md
}
