package otelredis

import (
	"go.opentelemetry.io/otel/metric"
)

const (
	// metricOperationDuration is the histogram of Redis command durations.
	metricOperationDuration = "db.client.operation.duration"
	// metricCacheGets counts Redis GETs partitioned by hit/miss result.
	metricCacheGets = "redis.cache.gets"
)

// instruments holds the OTel metric instruments recorded by the hook. They
// are constructed once per wrapped client from the configured meter; under
// the global no-op meter provider they resolve to no-op instruments, so the
// hook stays zero-cost when no provider is configured.
type instruments struct {
	duration  metric.Float64Histogram
	cacheGets metric.Int64Counter
}

// newInstruments builds the metric instruments from m. Construction errors
// are intentionally ignored: a failing instrument returns a usable no-op, and
// swallowing the error keeps WrapClient free of an error return, matching the
// OTel-API-only optionality contract.
func newInstruments(m metric.Meter) instruments {
	duration, _ := m.Float64Histogram(
		metricOperationDuration,
		metric.WithUnit("s"),
		metric.WithDescription("Duration of Redis client operations."),
	)
	cacheGets, _ := m.Int64Counter(
		metricCacheGets,
		metric.WithDescription("Count of Redis cache GETs partitioned by hit/miss result."),
	)
	return instruments{duration: duration, cacheGets: cacheGets}
}
