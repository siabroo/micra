// Example 03: L2 distributed cache + cross-instance pub/sub invalidation.
//
// It registers the rediscache.Client component, then (via RunOnce, once the
// client is Init'd) demonstrates the "proper usage" of the module:
//
//   - an L2 Cache: Set → Get(hit) → Delete → Get(miss);
//   - a Subscriber that logs invalidation events it receives;
//   - a Publisher that broadcasts a key invalidation which the Subscriber reacts
//     to — the mechanism sibling instances use to keep their caches in sync.
//
// The Redis client is wrapped with adapters/otelredis, so every command in the
// flow above — including the Get(hit) and the Get-after-delete(miss) — emits an
// OTel CLIENT span plus command-duration and cache hit/miss metrics. No OTel SDK
// is configured here, so the instrumentation resolves to the no-op provider at
// zero cost; wiring an exporter (see adapters/otelinit) is all it takes to see
// the spans and hit/miss counters.
//
// Run Redis first:
//
//	docker compose up -d
//	go run .
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/siabroo/micra/adapters/loggerslog"
	"github.com/siabroo/micra/adapters/otelredis"
	"github.com/siabroo/micra/components/rediscache"
	"github.com/siabroo/micra/core"
)

// addr is the Redis address published by docker-compose.yml.
const (
	addr    = "localhost:6379"
	channel = "cache:invalidation"
)

func main() {
	rawLog := loggerslog.New(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	app := core.New(
		core.WithName("example-redis-cache"),
		core.WithLogger(rawLog),
	)

	client := rediscache.New(rediscache.WithAddr(addr))
	if err := app.Register(client); err != nil {
		exit(1, err)
	}

	err := app.RunOnce(context.Background(), func(ctx context.Context) error {
		// Instrument the client so every command below flows through OTel
		// spans + hit/miss metrics. AddHook composes, so wrapping once here
		// covers the Cache, Publisher and Subscriber built from this client.
		rdb := otelredis.WrapClient(client.Redis(), otelredis.WithApplication("example-redis-cache"))
		return demo(ctx, rdb)
	})
	exit(code(err), nil)
}

// demo exercises the L2 cache and pub/sub invalidation against a ready client.
func demo(ctx context.Context, rdb *redis.Client) error {
	log := core.LoggerFrom(ctx)

	cache := rediscache.NewCache(rdb, rediscache.WithPrefix("demo"))
	pub := rediscache.NewPublisher(rdb, channel)
	sub := rediscache.NewSubscriber(rdb, channel)

	// A sibling instance would run this Subscriber to drop keys from its own
	// cache when another instance mutates shared state. Here we just log.
	received := make(chan []string, 1)
	if err := sub.Subscribe(ctx, func(keys []string) {
		log.Info("invalidation received", "keys", keys)
		select {
		case received <- keys:
		default:
		}
	}); err != nil {
		return err
	}

	const key = "user:42"
	if err := cache.Set(ctx, key, []byte("alice"), time.Minute); err != nil {
		return err
	}

	val, found, err := cache.Get(ctx, key)
	if err != nil {
		return err
	}
	log.Info("cache get", "key", key, "found", found, "value", string(val))

	if err := cache.Delete(ctx, key); err != nil {
		return err
	}

	_, found, err = cache.Get(ctx, key)
	if err != nil {
		return err
	}
	log.Info("cache get after delete", "key", key, "found", found)

	// Broadcast the invalidation so sibling instances drop their copy too.
	if err := pub.Publish(ctx, key); err != nil {
		return err
	}
	log.Info("invalidation published", "key", key)

	select {
	case <-received:
		log.Info("subscriber reacted to invalidation")
	case <-time.After(5 * time.Second):
		log.Warn("timed out waiting for invalidation")
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func exit(c int, err error) {
	if err != nil {
		slog.Error("fatal", "error", err)
	}
	os.Exit(c)
}

func code(err error) int {
	if err != nil {
		slog.Error("app", "error", err)
		return 1
	}
	return 0
}
