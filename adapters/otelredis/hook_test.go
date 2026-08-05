package otelredis

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// newRecordingTracerProvider returns a TracerProvider backed by an
// in-memory SpanRecorder so tests can assert on emitted spans.
func newRecordingTracerProvider() (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	return tp, sr
}

func attrValue(attrs []attribute.KeyValue, key attribute.Key) (attribute.Value, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

func TestProcessHookEmitsSpanPerCommand(t *testing.T) {
	tp, sr := newRecordingTracerProvider()
	h := newHook(configWith(WithTracerProvider(tp)))

	next := func(_ context.Context, _ redis.Cmder) error { return nil }
	cmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")

	if err := h.ProcessHook(next)(context.Background(), cmd); err != nil {
		t.Fatalf("ProcessHook returned error: %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name() != "set" {
		t.Errorf("span name = %q, want %q", span.Name(), "set")
	}
	if span.SpanKind() != trace.SpanKindClient {
		t.Errorf("span kind = %v, want client", span.SpanKind())
	}
	if v, ok := attrValue(span.Attributes(), semconv.DBSystemKey); !ok || v.AsString() != "redis" {
		t.Errorf("missing/incorrect db.system attribute: %v (present=%v)", v.AsString(), ok)
	}
	if v, ok := attrValue(span.Attributes(), "db.operation.name"); !ok || v.AsString() != "set" {
		t.Errorf("missing/incorrect db.operation.name attribute: %v (present=%v)", v.AsString(), ok)
	}
	if span.Status().Code == otelcodes.Error {
		t.Errorf("span unexpectedly marked as error")
	}
}

func TestProcessHookRedisNilIsNotError(t *testing.T) {
	tp, sr := newRecordingTracerProvider()
	h := newHook(configWith(WithTracerProvider(tp)))

	next := func(_ context.Context, cmd redis.Cmder) error {
		cmd.SetErr(redis.Nil)
		return redis.Nil
	}
	cmd := redis.NewStringCmd(context.Background(), "get", "missing")

	err := h.ProcessHook(next)(context.Background(), cmd)
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("expected redis.Nil to propagate, got %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status().Code == otelcodes.Error {
		t.Errorf("redis.Nil must not mark the span as error")
	}
}

func TestProcessHookRealErrorMarksSpan(t *testing.T) {
	tp, sr := newRecordingTracerProvider()
	h := newHook(configWith(WithTracerProvider(tp)))

	sentinel := errors.New("boom")
	next := func(_ context.Context, _ redis.Cmder) error { return sentinel }
	cmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")

	if err := h.ProcessHook(next)(context.Background(), cmd); !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status().Code != otelcodes.Error {
		t.Errorf("real error must mark span as error, got %v", spans[0].Status().Code)
	}
}

func TestProcessHookWithApplicationSetsAttribute(t *testing.T) {
	tp, sr := newRecordingTracerProvider()
	h := newHook(configWith(WithTracerProvider(tp), WithApplication("svc")))

	next := func(_ context.Context, _ redis.Cmder) error { return nil }
	cmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")
	if err := h.ProcessHook(next)(context.Background(), cmd); err != nil {
		t.Fatalf("ProcessHook returned error: %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if v, ok := attrValue(spans[0].Attributes(), "db.client.application"); !ok || v.AsString() != "svc" {
		t.Errorf("db.client.application = %v (present=%v), want %q", v.AsString(), ok, "svc")
	}
}

func TestProcessHookWithoutApplicationOmitsAttribute(t *testing.T) {
	tp, sr := newRecordingTracerProvider()
	h := newHook(configWith(WithTracerProvider(tp)))

	next := func(_ context.Context, _ redis.Cmder) error { return nil }
	cmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")
	if err := h.ProcessHook(next)(context.Background(), cmd); err != nil {
		t.Fatalf("ProcessHook returned error: %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if _, ok := attrValue(spans[0].Attributes(), "db.client.application"); ok {
		t.Errorf("db.client.application must be omitted when WithApplication is unset")
	}
}

func TestProcessHookNoopProviderDoesNotPanic(t *testing.T) {
	// No provider override => resolves to the global noop provider.
	h := newHook(defaults())
	next := func(_ context.Context, _ redis.Cmder) error { return nil }
	cmd := redis.NewStatusCmd(context.Background(), "ping")

	if err := h.ProcessHook(next)(context.Background(), cmd); err != nil {
		t.Fatalf("unexpected error under noop provider: %v", err)
	}
}

func TestProcessPipelineHookEmitsSpanAndPerCommandMetrics(t *testing.T) {
	tp, sr := newRecordingTracerProvider()
	mp, reader := newRecordingMeterProvider()
	h := newHook(configWith(WithTracerProvider(tp), WithMeterProvider(mp)))

	setCmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")
	hitCmd := redis.NewStringCmd(context.Background(), "get", "present")
	missCmd := redis.NewStringCmd(context.Background(), "get", "missing")
	cmds := []redis.Cmder{setCmd, hitCmd, missCmd}

	next := func(_ context.Context, cmds []redis.Cmder) error {
		// Simulate a GET miss on the third command.
		missCmd.SetErr(redis.Nil)
		return redis.Nil
	}

	err := h.ProcessPipelineHook(next)(context.Background(), cmds)
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("expected redis.Nil to propagate, got %v", err)
	}

	// Exactly one span covers the whole pipeline.
	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 pipeline span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name() != "pipeline" {
		t.Errorf("span name = %q, want %q", span.Name(), "pipeline")
	}
	if span.SpanKind() != trace.SpanKindClient {
		t.Errorf("span kind = %v, want client", span.SpanKind())
	}
	if v, ok := attrValue(span.Attributes(), "db.operation.batch.size"); !ok || v.AsInt64() != 3 {
		t.Errorf("db.operation.batch.size = %d (present=%v), want 3", v.AsInt64(), ok)
	}
	// A pipeline whose final error is redis.Nil must not be marked as error.
	if span.Status().Code == otelcodes.Error {
		t.Errorf("redis.Nil pipeline result must not mark the span as error")
	}

	// Duration histogram gets one recording per batched command.
	rm := collect(t, reader)
	dm, ok := findMetric(rm, metricOperationDuration)
	if !ok {
		t.Fatalf("missing %q metric", metricOperationDuration)
	}
	hist := dm.Data.(metricdata.Histogram[float64])
	var total uint64
	for _, dp := range hist.DataPoints {
		total += dp.Count
	}
	if total != 3 {
		t.Errorf("total histogram count = %d, want 3", total)
	}

	// The two GETs increment the cache-gets counter: one hit, one miss.
	cm, ok := findMetric(rm, metricCacheGets)
	if !ok {
		t.Fatalf("missing %q metric", metricCacheGets)
	}
	sum := cm.Data.(metricdata.Sum[int64])
	results := map[string]int64{}
	for _, dp := range sum.DataPoints {
		if v, ok := dp.Attributes.Value("result"); ok {
			results[v.AsString()] += dp.Value
		}
	}
	if results["hit"] != 1 {
		t.Errorf("hit count = %d, want 1", results["hit"])
	}
	if results["miss"] != 1 {
		t.Errorf("miss count = %d, want 1", results["miss"])
	}
}

// TestProcessPipelineHookRealErrorAfterMissMarksSpan guards the case where the
// pipeline's aggregate error (the first command's error) is redis.Nil — an
// early GET miss — while a later command failed for real. The span must be
// marked as error even though the redis.Nil aggregate is not.
func TestProcessPipelineHookRealErrorAfterMissMarksSpan(t *testing.T) {
	tp, sr := newRecordingTracerProvider()
	h := newHook(configWith(WithTracerProvider(tp)))

	missCmd := redis.NewStringCmd(context.Background(), "get", "missing")
	failCmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")
	cmds := []redis.Cmder{missCmd, failCmd}

	sentinel := errors.New("boom")
	next := func(_ context.Context, _ []redis.Cmder) error {
		// Early GET miss, then a genuine failure on a later command. go-redis
		// returns the first command's error (redis.Nil) as the aggregate.
		missCmd.SetErr(redis.Nil)
		failCmd.SetErr(sentinel)
		return redis.Nil
	}

	// The hook propagates the aggregate (redis.Nil) unchanged to the caller.
	if err := h.ProcessPipelineHook(next)(context.Background(), cmds); !errors.Is(err, redis.Nil) {
		t.Fatalf("expected redis.Nil aggregate to propagate, got %v", err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 pipeline span, got %d", len(spans))
	}
	if spans[0].Status().Code != otelcodes.Error {
		t.Errorf("a genuine command error behind a redis.Nil miss must mark the span as error, got %v", spans[0].Status().Code)
	}
}

// TestProcessPipelineHookPoolErrorMarksEveryCommand guards the case where the
// pipeline fails at the pool/transport level (e.g. go-redis cannot acquire a
// connection) before any per-command error is populated: next returns a genuine
// non-redis.Nil error while every cmd.Err() stays nil. The span must be marked
// as error and the per-command metrics must attribute the aggregate failure to
// every command — recording error=true and NOT counting the queued GETs as
// hits — so a Redis outage is not masked in metrics.
func TestProcessPipelineHookPoolErrorMarksEveryCommand(t *testing.T) {
	tp, sr := newRecordingTracerProvider()
	mp, reader := newRecordingMeterProvider()
	h := newHook(configWith(WithTracerProvider(tp), WithMeterProvider(mp)))

	getCmd := redis.NewStringCmd(context.Background(), "get", "present")
	setCmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")
	cmds := []redis.Cmder{getCmd, setCmd}

	sentinel := errors.New("connection pool exhausted")
	next := func(_ context.Context, _ []redis.Cmder) error {
		// Pool/transport failure before any command is written: the aggregate is
		// non-nil but no command carries its own error.
		return sentinel
	}

	if err := h.ProcessPipelineHook(next)(context.Background(), cmds); !errors.Is(err, sentinel) {
		t.Fatalf("expected pool error to propagate, got %v", err)
	}

	// The span reflects the failure.
	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 pipeline span, got %d", len(spans))
	}
	if spans[0].Status().Code != otelcodes.Error {
		t.Errorf("a pool-level pipeline failure must mark the span as error, got %v", spans[0].Status().Code)
	}

	rm := collect(t, reader)

	// Every command's duration is recorded with error=true.
	dm, ok := findMetric(rm, metricOperationDuration)
	if !ok {
		t.Fatalf("missing %q metric", metricOperationDuration)
	}
	hist := dm.Data.(metricdata.Histogram[float64])
	var total uint64
	for _, dp := range hist.DataPoints {
		total += dp.Count
		if v, ok := dp.Attributes.Value("error"); !ok || !v.AsBool() {
			t.Errorf("duration error attribute = %v (present=%v), want true", v.AsBool(), ok)
		}
	}
	if total != 2 {
		t.Errorf("total histogram count = %d, want 2", total)
	}

	// The failed GET must NOT be counted as a hit (or miss): the outage must not
	// inflate the hit ratio.
	if _, ok := findMetric(rm, metricCacheGets); ok {
		t.Errorf("a GET failing under a pool-level pipeline error must not record the %q counter", metricCacheGets)
	}
}

// configWith builds a config from options, mirroring WrapClient's setup.
func configWith(opts ...Option) config {
	cfg := defaults()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
