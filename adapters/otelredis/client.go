// Package otelredis instruments a *redis.Client with OpenTelemetry using
// go-redis v9's native Hook interface. It emits:
//
//   - one CLIENT span per standalone Redis command, and one CLIENT span
//     covering a whole pipeline (tracing parity with adapters/otelpgx), and
//   - OTel metrics per command — a command-duration histogram and a cache
//     hit/miss counter — recorded for every command, including each command
//     batched in a pipeline.
//
// Cache hit/miss is derived at the command level: a GET with a nil error
// is a hit, a GET returning redis.Nil is a miss, and a GET that fails with
// a genuine error (connection failure, timeout, WRONGTYPE, ...) is neither
// — it is excluded from the hit/miss counter so an outage does not inflate
// the hit ratio. This keeps the rediscache component free of any OTel
// dependency, exactly as pgxpool has no OTel coupling.
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
