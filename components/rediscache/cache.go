package rediscache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// cacheConfig holds Cache tunables set via CacheOption.
type cacheConfig struct {
	prefix     string
	defaultTTL time.Duration
}

// CacheOption configures a Cache via NewCache.
type CacheOption func(*cacheConfig)

// WithPrefix sets a key namespace applied to every key as "<prefix>:<key>".
// An empty prefix (the default) leaves keys unchanged.
func WithPrefix(prefix string) CacheOption {
	return func(c *cacheConfig) { c.prefix = prefix }
}

// WithDefaultTTL sets the TTL applied by Set when its ttl argument is 0.
// A zero default TTL means keys are stored without expiry.
func WithDefaultTTL(ttl time.Duration) CacheOption {
	return func(c *cacheConfig) { c.defaultTTL = ttl }
}

// Cache is an L2 (shared, Redis-backed) key/value cache. There is no in-memory
// L1 tier; every operation talks to Redis. Keys are namespaced with an optional
// prefix and stored with a per-call or default TTL.
type Cache struct {
	rdb redis.Cmdable
	cfg cacheConfig
}

// NewCache wraps a redis client (typically Client.Redis()) as an L2 Cache.
func NewCache(rdb redis.Cmdable, opts ...CacheOption) *Cache {
	var cfg cacheConfig
	for _, o := range opts {
		o(&cfg)
	}
	return &Cache{rdb: rdb, cfg: cfg}
}

// namespaced returns the storage key: "<prefix>:<key>" when a prefix is set,
// otherwise the bare key.
func (c *Cache) namespaced(key string) string {
	if c.cfg.prefix == "" {
		return key
	}
	return c.cfg.prefix + ":" + key
}

// effectiveTTL returns ttl, or the configured default when ttl is 0.
func (c *Cache) effectiveTTL(ttl time.Duration) time.Duration {
	if ttl == 0 {
		return c.cfg.defaultTTL
	}
	return ttl
}

// Get returns the value for key. On a cache miss it returns (nil, false, nil);
// any other Redis error is returned with found=false.
func (c *Cache) Get(ctx context.Context, key string) (val []byte, found bool, err error) {
	b, err := c.rdb.Get(ctx, c.namespaced(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// Set stores val under key. A ttl of 0 uses the configured default TTL (which,
// when itself 0, stores the key without expiry).
func (c *Cache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.namespaced(key), val, c.effectiveTTL(ttl)).Err()
}

// Delete removes the given keys. Missing keys are ignored. Returns nil when no
// keys are supplied.
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	full := make([]string, len(keys))
	for i, k := range keys {
		full[i] = c.namespaced(k)
	}
	return c.rdb.Del(ctx, full...).Err()
}
