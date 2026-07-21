package otelredis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// hook implements redis.Hook, emitting a CLIENT span and metrics per command.
type hook struct {
	cfg  config
	inst instruments
}

func newHook(cfg config) *hook {
	return &hook{cfg: cfg, inst: newInstruments(cfg.meter())}
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
		start := time.Now()
		err := next(ctx, cmd)
		h.record(ctx, cmd, err, time.Since(start))
		end(err)
		return err
	}
}

// record emits the per-command metrics: the operation-duration histogram for
// every command and, for GET commands, the cache hit/miss counter. redis.Nil
// is a key miss, not a failure, so it is excluded from the error attribute and
// counted as a "miss".
func (h *hook) record(ctx context.Context, cmd redis.Cmder, err error, dur time.Duration) {
	isErr := err != nil && !errors.Is(err, redis.Nil)
	h.inst.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(
		semconv.DBOperationName(cmd.FullName()),
		attribute.Bool("error", isErr),
	))
	if cmd.Name() == "get" {
		result := "hit"
		if errors.Is(err, redis.Nil) {
			result = "miss"
		}
		h.inst.cacheGets.Add(ctx, 1, metric.WithAttributes(
			attribute.String("result", result),
		))
	}
}

// ProcessPipelineHook wraps a whole pipeline in a single CLIENT span and
// records per-command duration and cache hit/miss metrics for every batched
// command. Individual command timings are not separable within a pipeline, so
// each command is recorded with the pipeline's total duration. redis.Nil on a
// batched command (e.g. a GET miss) is treated as a non-error, mirroring the
// single-command path.
func (h *hook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		ctx, end := h.startSpan(ctx, "pipeline", attribute.Int("db.operation.batch.size", len(cmds)))
		start := time.Now()
		err := next(ctx, cmds)
		dur := time.Since(start)
		for _, cmd := range cmds {
			h.record(ctx, cmd, cmd.Err(), dur)
		}
		end(err)
		return err
	}
}

// startSpan opens a CLIENT span named after the Redis command and returns
// the span context plus an end function that records the error (unless it
// is redis.Nil) and ends the span. Extra attributes are appended to the
// semconv defaults.
func (h *hook) startSpan(ctx context.Context, operation string, extra ...attribute.KeyValue) (context.Context, func(error)) {
	attrs := []attribute.KeyValue{
		semconv.DBSystemRedis,
		semconv.DBOperationName(operation),
	}
	attrs = append(attrs, extra...)
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
