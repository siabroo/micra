//go:build integration

package rediscache_test

import (
	"context"
	"sync"
	"sync/atomic"
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

func TestPubSub_SkipsMalformedMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := newClientForTest(t)
	rdb := c.Redis()

	const channel = "invalidations-malformed"
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

	// A malformed payload must be skipped without killing the consumer loop.
	if err := rdb.Publish(ctx, channel, "not-json").Err(); err != nil {
		t.Fatalf("Publish malformed: %v", err)
	}
	// A subsequent valid message must still be delivered.
	want := []string{"user:1"}
	if err := pub.Publish(ctx, want...); err != nil {
		t.Fatalf("Publish valid: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("valid message not received after malformed one within 3s")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("handler fired %d times, want 1 (malformed message must be skipped)", len(received))
	}
	if len(received[0]) != 1 || received[0][0] != "user:1" {
		t.Fatalf("received keys %v, want %v", received[0], want)
	}
}

func TestPubSub_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	c := newClientForTest(t)
	rdb := c.Redis()

	const channel = "invalidations-cancel"
	pub := micraredis.NewPublisher(rdb, channel)
	sub := micraredis.NewSubscriber(rdb, channel)

	var calls int32
	fired := make(chan struct{}, 1)
	if err := sub.Subscribe(ctx, func(_ []string) {
		atomic.AddInt32(&calls, 1)
		select {
		case fired <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Confirm the subscriber is live before cancelling.
	if err := pub.Publish(ctx, "before"); err != nil {
		t.Fatalf("Publish before cancel: %v", err)
	}
	select {
	case <-fired:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not receive pre-cancel message within 3s")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls before cancel = %d, want 1", got)
	}

	// Cancelling the context must shut the subscriber goroutine down cleanly so
	// no further messages are delivered.
	cancel()
	// Give the goroutine a moment to close the underlying subscription.
	time.Sleep(200 * time.Millisecond)

	// Publish after cancellation on a fresh context; the handler must not fire.
	if err := pub.Publish(context.Background(), "after"); err != nil {
		t.Fatalf("Publish after cancel: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("handler fired %d times after cancel, want it to stay at 1", got)
	}
}
