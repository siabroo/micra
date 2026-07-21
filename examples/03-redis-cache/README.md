# Example 03 — Redis L2 cache + pub/sub invalidation

Demonstrates the `components/rediscache` module: the `Client` lifecycle
component, the L2 distributed `Cache` (`Set`/`Get`/`Delete`), and cross-instance
cache synchronisation via the `Publisher`/`Subscriber` invalidation channel.

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
