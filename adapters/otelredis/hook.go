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
// counted as a "miss". A GET that fails with a genuine (non-redis.Nil) error —
// a connection failure, timeout, WRONGTYPE, etc. — is neither a hit nor a miss
// and is therefore excluded from the hit/miss counter, so an outage does not
// inflate the hit ratio.
func (h *hook) record(ctx context.Context, cmd redis.Cmder, err error, dur time.Duration) {
	isErr := err != nil && !errors.Is(err, redis.Nil)
	h.inst.duration.Record(ctx, dur.Seconds(), metric.WithAttributes(
		semconv.DBOperationName(cmd.FullName()),
		attribute.Bool("error", isErr),
	))
	if cmd.Name() == "get" && !isErr {
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
		fallback := pipelineFallbackErr(err, cmds)
		for _, cmd := range cmds {
			cmdErr := cmd.Err()
			if cmdErr == nil {
				cmdErr = fallback
			}
			h.record(ctx, cmd, cmdErr, dur)
		}
		end(pipelineSpanErr(err, cmds))
		return err
	}
}

// pipelineSpanErr derives the error used to set the pipeline span's status.
// go-redis reports a pipeline's aggregate error as the first command's error
// (cmdsFirstErr), which may be redis.Nil (a GET miss) even when a later command
// failed for real. Since startSpan ignores redis.Nil, using the aggregate alone
// would mark a batch with a genuine failure as successful. Scan every command
// for the first non-redis.Nil error and prefer it; fall back to the aggregate
// (which carries transport-level failures not attached to any command).
func pipelineSpanErr(aggregate error, cmds []redis.Cmder) error {
	for _, cmd := range cmds {
		if err := cmd.Err(); err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
	}
	return aggregate
}

// pipelineFallbackErr returns the pipeline's aggregate error when it should be
// attributed to every batched command's metrics, or nil otherwise. go-redis can
// return a pool- or transport-level failure (e.g. it fails to acquire a
// connection) before any per-command error is populated; in that case the
// aggregate is the only signal that the batch failed, and without it each
// command is recorded with error=false and queued GETs are counted as hits,
// masking the outage. When any command already carries its own error, go-redis
// has propagated the failure to the affected commands (read errors set the
// error on the failing command and every command after it), so the aggregate
// must not override the earlier commands that genuinely succeeded. redis.Nil is
// a key miss, not a failure, so it is never used as a fallback.
func pipelineFallbackErr(aggregate error, cmds []redis.Cmder) error {
	if aggregate == nil || errors.Is(aggregate, redis.Nil) {
		return nil
	}
	for _, cmd := range cmds {
		if cmd.Err() != nil {
			return nil
		}
	}
	return aggregate
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
