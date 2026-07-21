// Package otelredis instruments a *redis.Client with OpenTelemetry using
// go-redis v9's native Hook interface. For every command (and every
// command in a pipeline) it emits:
//
//   - one CLIENT span per Redis command (tracing parity with adapters/otelpgx), and
//   - OTel metrics: a command-duration histogram and a cache hit/miss counter.
//
// Cache hit/miss is derived at the command level: a GET returning
// redis.Nil is a miss, otherwise a hit. This keeps the rediscache
// component free of any OTel dependency, exactly as pgxpool has no OTel
// coupling.
//
// Optionality: this module depends only on the OTel API
// (go.opentelemetry.io/otel*). It does NOT pull the OTel SDK. A service
// that imports adapters/otelredis but does not initialise a
// TracerProvider / MeterProvider gets the OTel defaults (noop provider):
// zero-cost spans and metrics. Wiring the SDK is the service's job (see
// adapters/otelinit).
//
// Unlike adapters/otelpgx, this module does NOT inject a sqlcommenter
// comment or route into commands: that mechanism is Postgres-specific.
package otelredis

import (
	"github.com/redis/go-redis/v9"
)

// WrapClient installs an OpenTelemetry hook on the given Redis client and
// returns the same client for convenient chaining. Apply functional
// options for application name and provider overrides.
//
// The hook is added via redis.Client.AddHook, so it composes with any
// other hooks already installed on the client.
func WrapClient(rdb *redis.Client, opts ...Option) *redis.Client {
	cfg := defaults()
	for _, opt := range opts {
		opt(&cfg)
	}
	rdb.AddHook(newHook(cfg))
	return rdb
}
