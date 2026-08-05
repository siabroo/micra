# Example 03 — Redis L2 cache + pub/sub invalidation

Demonstrates the `components/rediscache` module: the `Client` lifecycle
component, the L2 distributed `Cache` (`Set`/`Get`/`Delete`), and cross-instance
cache synchronisation via the `Publisher`/`Subscriber` invalidation channel.

The client is wrapped with [`adapters/otelredis`](../../adapters/otelredis) via
`otelredis.WrapClient`, so every command in the flow emits an OTel CLIENT span
plus command-duration and cache hit/miss metrics — the `Get` after `Set` records
a `hit`, the `Get` after `Delete` records a `miss`. No OTel SDK is configured
here, so the instrumentation resolves to the no-op provider at zero cost; add an
exporter (see `adapters/otelinit`) to surface the spans and hit/miss counters.

## Run

Start Redis, then run the example:

```sh
docker compose up -d
go run .
```

Expected: log lines for a cache hit, a miss after delete, a published
invalidation, and the subscriber reacting to it.

Tear down when done:

```sh
docker compose down
```
