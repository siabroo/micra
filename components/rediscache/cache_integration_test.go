//go:build integration

package rediscache_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	micraredis "github.com/siabroo/micra/components/rediscache"
)

func newCacheForTest(t *testing.T, opts ...micraredis.CacheOption) *micraredis.Cache {
	t.Helper()
	ctx := context.Background()
	dsn := startRedis(t)
	c := micraredis.New(micraredis.WithDSN(dsn))
	if err := c.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = c.Stop(ctx) })
	return micraredis.NewCache(c.Redis(), opts...)
}

func TestCache_SetGetDelete(t *testing.T) {
	ctx := context.Background()
	cache := newCacheForTest(t, micraredis.WithPrefix("app"))

	// Miss before set.
	if _, found, err := cache.Get(ctx, "k1"); err != nil || found {
		t.Fatalf("Get before Set: found=%v err=%v, want found=false err=nil", found, err)
	}

	if err := cache.Set(ctx, "k1", []byte("v1"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, found, err := cache.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if !found {
		t.Fatal("Get after Set: found=false, want true")
	}
	if !bytes.Equal(val, []byte("v1")) {
		t.Fatalf("Get value = %q, want %q", val, "v1")
	}

	if err := cache.Delete(ctx, "k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, found, err := cache.Get(ctx, "k1"); err != nil || found {
		t.Fatalf("Get after Delete: found=%v err=%v, want found=false err=nil", found, err)
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	cache := newCacheForTest(t)

	if err := cache.Set(ctx, "temp", []byte("x"), 200*time.Millisecond); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, found, err := cache.Get(ctx, "temp"); err != nil || !found {
		t.Fatalf("Get before expiry: found=%v err=%v, want found=true", found, err)
	}

	time.Sleep(400 * time.Millisecond)

	if _, found, err := cache.Get(ctx, "temp"); err != nil || found {
		t.Fatalf("Get after expiry: found=%v err=%v, want found=false err=nil", found, err)
	}
}

func TestCache_DeleteMultiple(t *testing.T) {
	ctx := context.Background()
	cache := newCacheForTest(t)

	for _, k := range []string{"a", "b", "c"} {
		if err := cache.Set(ctx, k, []byte(k), 0); err != nil {
			t.Fatalf("Set %q: %v", k, err)
		}
	}
	if err := cache.Delete(ctx, "a", "b", "c"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	for _, k := range []string{"a", "b", "c"} {
		if _, found, err := cache.Get(ctx, k); err != nil || found {
			t.Fatalf("Get %q after Delete: found=%v err=%v", k, found, err)
		}
	}
}
