package otelredis

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// newRecordingMeterProvider returns a MeterProvider backed by a manual Reader
// so tests can collect and assert on emitted metrics.
func newRecordingMeterProvider() (*sdkmetric.MeterProvider, *sdkmetric.ManualReader) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	return mp, reader
}

// collect gathers the current metrics from the reader.
func collect(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	return rm
}

// findMetric locates a metric by name across all scopes, or returns false.
func findMetric(rm metricdata.ResourceMetrics, name string) (metricdata.Metrics, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m, true
			}
		}
	}
	return metricdata.Metrics{}, false
}

func TestProcessHookRecordsDurationPerCommand(t *testing.T) {
	mp, reader := newRecordingMeterProvider()
	h := newHook(configWith(WithMeterProvider(mp)))

	next := func(_ context.Context, _ redis.Cmder) error { return nil }
	cmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")
	if err := h.ProcessHook(next)(context.Background(), cmd); err != nil {
		t.Fatalf("ProcessHook returned error: %v", err)
	}

	m, ok := findMetric(collect(t, reader), metricOperationDuration)
	if !ok {
		t.Fatalf("missing %q metric", metricOperationDuration)
	}
	hist, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("%q is not a float64 histogram: %T", metricOperationDuration, m.Data)
	}
	if len(hist.DataPoints) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(hist.DataPoints))
	}
	dp := hist.DataPoints[0]
	if dp.Count != 1 {
		t.Errorf("histogram count = %d, want 1", dp.Count)
	}
	if v, ok := dp.Attributes.Value("db.operation.name"); !ok || v.AsString() != "set" {
		t.Errorf("db.operation.name = %v (present=%v), want set", v.AsString(), ok)
	}
	if v, ok := dp.Attributes.Value("error"); !ok || v.AsBool() {
		t.Errorf("error attribute = %v (present=%v), want false", v.AsBool(), ok)
	}
}

func TestProcessHookRecordsDurationWithErrorAttribute(t *testing.T) {
	mp, reader := newRecordingMeterProvider()
	h := newHook(configWith(WithMeterProvider(mp)))

	next := func(_ context.Context, _ redis.Cmder) error { return errors.New("boom") }
	cmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")
	_ = h.ProcessHook(next)(context.Background(), cmd)

	m, ok := findMetric(collect(t, reader), metricOperationDuration)
	if !ok {
		t.Fatalf("missing %q metric", metricOperationDuration)
	}
	hist := m.Data.(metricdata.Histogram[float64])
	if v, ok := hist.DataPoints[0].Attributes.Value("error"); !ok || !v.AsBool() {
		t.Errorf("error attribute = %v (present=%v), want true", v.AsBool(), ok)
	}
}

func TestProcessHookRecordsCacheHit(t *testing.T) {
	mp, reader := newRecordingMeterProvider()
	h := newHook(configWith(WithMeterProvider(mp)))

	next := func(_ context.Context, _ redis.Cmder) error { return nil }
	cmd := redis.NewStringCmd(context.Background(), "get", "present")
	if err := h.ProcessHook(next)(context.Background(), cmd); err != nil {
		t.Fatalf("ProcessHook returned error: %v", err)
	}

	assertCacheGet(t, reader, "hit", 1)
}

func TestProcessHookRecordsCacheMiss(t *testing.T) {
	mp, reader := newRecordingMeterProvider()
	h := newHook(configWith(WithMeterProvider(mp)))

	next := func(_ context.Context, cmd redis.Cmder) error {
		cmd.SetErr(redis.Nil)
		return redis.Nil
	}
	cmd := redis.NewStringCmd(context.Background(), "get", "missing")
	if err := h.ProcessHook(next)(context.Background(), cmd); !errors.Is(err, redis.Nil) {
		t.Fatalf("expected redis.Nil, got %v", err)
	}

	assertCacheGet(t, reader, "miss", 1)
}

func TestProcessHookNonGetDoesNotTouchCounter(t *testing.T) {
	mp, reader := newRecordingMeterProvider()
	h := newHook(configWith(WithMeterProvider(mp)))

	next := func(_ context.Context, _ redis.Cmder) error { return nil }
	cmd := redis.NewStatusCmd(context.Background(), "set", "k", "v")
	if err := h.ProcessHook(next)(context.Background(), cmd); err != nil {
		t.Fatalf("ProcessHook returned error: %v", err)
	}

	if _, ok := findMetric(collect(t, reader), metricCacheGets); ok {
		t.Errorf("non-GET command must not record the %q counter", metricCacheGets)
	}
}

// assertCacheGet asserts the cache-gets counter has a single data point with
// the given result attribute and value.
func assertCacheGet(t *testing.T, reader *sdkmetric.ManualReader, wantResult string, wantValue int64) {
	t.Helper()
	m, ok := findMetric(collect(t, reader), metricCacheGets)
	if !ok {
		t.Fatalf("missing %q metric", metricCacheGets)
	}
	sum, ok := m.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("%q is not an int64 sum: %T", metricCacheGets, m.Data)
	}
	if len(sum.DataPoints) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(sum.DataPoints))
	}
	dp := sum.DataPoints[0]
	if v, ok := dp.Attributes.Value("result"); !ok || v.AsString() != wantResult {
		t.Errorf("result attribute = %v (present=%v), want %q", v.AsString(), ok, wantResult)
	}
	if dp.Value != wantValue {
		t.Errorf("counter value = %d, want %d", dp.Value, wantValue)
	}
}
