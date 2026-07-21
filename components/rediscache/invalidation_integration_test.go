//go:build integration

package rediscache_test

import (
	"context"
	"sync"
	"testing"
	"time"

	micraredis "github.com/siabroo/micra/components/rediscache"
)

func newClientForTest(t *testing.T) *micraredis.Client {
	t.Helper()
	ctx := context.Background()
	dsn := startRedis(t)
	c := micraredis.New(micraredis.WithDSN(dsn))
	if err := c.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = c.Stop(ctx) })
	return c
}

func TestPubSub_PublishReceived(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := newClientForTest(t)
	rdb := c.Redis()

	const channel = "invalidations"
	pub := micraredis.NewPublisher(rdb, channel)
	sub := micraredis.NewSubscriber(rdb, channel)

	var (
		mu       sync.Mutex
		received [][]string
	)
	done := make(chan struct{}, 1)
	if err := sub.Subscribe(ctx, func(keys []string) {
		mu.Lock()
		received = append(received, keys)
		mu.Unlock()
		select {
		case done <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	want := []string{"user:1", "user:2"}
	if err := pub.Publish(ctx, want...); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not receive message within 3s")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("received %d messages, want 1", len(received))
	}
	if len(received[0]) != len(want) {
		t.Fatalf("received keys %v, want %v", received[0], want)
	}
	for i := range want {
		if received[0][i] != want[i] {
			t.Errorf("key[%d] = %q, want %q", i, received[0][i], want[i])
		}
	}
}

func TestPubSub_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	c := newClientForTest(t)
	rdb := c.Redis()

	const channel = "invalidations-cancel"
	sub := micraredis.NewSubscriber(rdb, channel)

	if err := sub.Subscribe(ctx, func(_ []string) {}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Cancelling the context must shut the subscriber goroutine down cleanly.
	cancel()
	// Give the goroutine a moment to close the underlying subscription.
	time.Sleep(200 * time.Millisecond)
}
