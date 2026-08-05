# adapters/otelredis

Wrap a `*redis.Client` so every command emits an OTel CLIENT span (a pipeline
emits one span for the whole batch) and every command — including each one
batched in a pipeline — records command-duration and cache hit/miss metrics,
using go-redis v9's native `Hook` interface.

```go
import (
    "github.com/redis/go-redis/v9"
    "github.com/siabroo/micra/adapters/otelredis"
)

rdb := redis.NewClient(&redis.Options{Addr: addr})
rdb = otelredis.WrapClient(rdb, otelredis.WithApplication("auth-go"))
// Use rdb as usual — Cache/Publisher/Subscriber built from it inherit the hook.
```

`WrapClient` installs the hook via `redis.Client.AddHook`, so it composes with
any other hooks already on the client and returns the same client for chaining.

## What it emits

Spans — one CLIENT span per command, plus one span per pipeline:

| Attribute | Value |
|---|---|
| span name | the command name (e.g. `get`, `set`), or `pipeline` |
| `db.system` | `redis` |
| `db.operation.name` | the command name (or `pipeline`) |
| `db.operation.batch.size` | pipeline only — number of batched commands |
| `db.client.application` | from `WithApplication` (omitted if empty) |

A `redis.Nil` result (key miss on `GET` and friends) is treated as a non-error,
so a cache miss does **not** mark the span as failed.

Metrics:

| Instrument | Type | Attributes |
|---|---|---|
| `db.client.operation.duration` | Float64 histogram (seconds) | `db.operation.name`, `error` (bool) |
| `redis.cache.gets` | Int64 counter | `result` = `hit` or `miss` |

Cache hit/miss is derived at the command level: a `GET` with a nil error is a
`hit`, a `GET` returning `redis.Nil` is a `miss`, and a `GET` that fails with a
genuine error (connection failure, timeout, `WRONGTYPE`, ...) is neither — it is
excluded from the counter so an outage does not inflate the hit ratio. This
keeps the `components/rediscache` component free of any OTel dependency, exactly
as `pgxpool` has no OTel coupling.

## Optionality — no SDK, no-op by default

This module depends only on the OTel API (`go.opentelemetry.io/otel*`); it does
**not** pull the OTel SDK (the SDK is a test-only dependency). A service that
imports `adapters/otelredis` but configures no `TracerProvider` / `MeterProvider`
gets the OTel defaults (no-op provider): zero-cost spans and metrics. Wiring the
SDK and an exporter is the service's job — see `adapters/otelinit`.

## Options

| Option | Default | Purpose |
|---|---|---|
| `WithApplication(string)` | "" (omitted) | Static service name in the `db.client.application` span attribute. |
| `WithTracerProvider(tp)` | `otel.GetTracerProvider()` | Override for tests. |
| `WithMeterProvider(mp)` | `otel.GetMeterProvider()` | Override for tests. |

## What this module does NOT do

- Configure the OTel SDK, exporter, or W3C propagator (use `adapters/otelinit`).
- Inject a sqlcommenter comment or route into commands. That mechanism is
  Postgres-specific (`adapters/otelpgx`) and has no Redis equivalent.
- Instrument the `rediscache` component directly. Hit/miss is measured in the
  hook so the component stays OTel-free; the trade-off is that hit/miss is
  derived from `GET`/`redis.Nil` at the command level rather than from the
  cache's own `found` bool.
