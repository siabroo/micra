//go:build integration

package otelredis_test

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/siabroo/micra/adapters/otelredis"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// startRedis boots a throwaway Redis container and returns its DSN.
func startRedis(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	rc, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() {
		if err := rc.Terminate(ctx); err != nil {
			t.Logf("terminate: %v", err)
		}
	})
	addr, err := rc.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return addr
}

// TestIntegration_SpansAndMetrics drives real Redis commands through a wrapped
// client and asserts that spans and metrics are recorded via in-memory SDK
// collectors: a Set, a Get hit, a Get miss and a pipeline.
func TestIntegration_SpansAndMetrics(t *testing.T) {
	ctx := context.Background()
	dsn := startRedis(t)

	opts, err := redis.ParseURL(dsn)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	otelredis.WrapClient(rdb,
		otelredis.WithTracerProvider(tp),
		otelredis.WithMeterProvider(mp),
	)

	// Set, then a Get hit.
	if err := rdb.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatalf("SET: %v", err)
	}
	if got, err := rdb.Get(ctx, "k").Result(); err != nil || got != "v" {
		t.Fatalf("GET hit: got=%q err=%v", got, err)
	}
	// A Get miss.
	if _, err := rdb.Get(ctx, "absent").Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("GET miss: want redis.Nil, got %v", err)
	}
	// A pipeline with a set and a get.
	pipe := rdb.Pipeline()
	pipe.Set(ctx, "p", "1", 0)
	getInPipe := pipe.Get(ctx, "p")
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("pipeline exec: %v", err)
	}
	if getInPipe.Val() != "1" {
		t.Fatalf("pipeline GET: got %q, want 1", getInPipe.Val())
	}

	// Spans: at least SET, GET hit, GET miss and one pipeline span.
	spans := sr.Ended()
	names := map[string]int{}
	for _, s := range spans {
		names[s.Name()]++
	}
	if names["set"] < 1 {
		t.Errorf("expected at least one set span, got %d", names["set"])
	}
	if names["get"] < 2 {
		t.Errorf("expected at least two get spans, got %d", names["get"])
	}
	if names["pipeline"] < 1 {
		t.Errorf("expected at least one pipeline span, got %d", names["pipeline"])
	}

	// Metrics: duration histogram recorded, hit and miss counters incremented.
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}

	results := map[string]int64{}
	var haveDuration bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Histogram[float64]:
				if m.Name == "db.client.operation.duration" {
					haveDuration = true
				}
			case metricdata.Sum[int64]:
				if m.Name == "redis.cache.gets" {
					for _, dp := range data.DataPoints {
						if v, ok := dp.Attributes.Value("result"); ok {
							results[v.AsString()] += dp.Value
						}
					}
				}
			}
		}
	}

	if !haveDuration {
		t.Error("missing db.client.operation.duration histogram")
	}
	if results["hit"] < 1 {
		t.Errorf("expected at least one cache hit, got %d", results["hit"])
	}
	if results["miss"] < 1 {
		t.Errorf("expected at least one cache miss, got %d", results["miss"])
	}
}
