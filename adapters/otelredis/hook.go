package otelredis

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// hook implements redis.Hook, emitting a CLIENT span per command.
type hook struct {
	cfg config
}

func newHook(cfg config) *hook {
	return &hook{cfg: cfg}
}

// DialHook is a passthrough: dialing is a connection-pool concern, not a
// command, so it is not traced.
func (h *hook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

// ProcessHook wraps a single command in a CLIENT span carrying semconv DB
// attributes. redis.Nil (key-miss on GET and friends) is treated as a
// non-error so a cache miss does not mark the span as failed.
func (h *hook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		ctx, end := h.startSpan(ctx, cmd.FullName())
		err := next(ctx, cmd)
		end(err)
		return err
	}
}

// ProcessPipelineHook is a passthrough until pipeline instrumentation is
// added.
func (h *hook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

// startSpan opens a CLIENT span named after the Redis command and returns
// the span context plus an end function that records the error (unless it
// is redis.Nil) and ends the span.
func (h *hook) startSpan(ctx context.Context, operation string) (context.Context, func(error)) {
	attrs := []attribute.KeyValue{
		semconv.DBSystemRedis,
		semconv.DBOperationName(operation),
	}
	if h.cfg.application != "" {
		attrs = append(attrs, attribute.String("db.client.application", h.cfg.application))
	}
	ctx, span := h.cfg.tracer().Start(ctx, operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
	return ctx, func(err error) {
		if err != nil && !errors.Is(err, redis.Nil) {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, err.Error())
		}
		span.End()
	}
}
