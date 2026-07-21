//go:build integration

package rediscache_test

import (
	"context"
	"testing"
	"time"

	micraredis "github.com/siabroo/micra/components/rediscache"
	"github.com/siabroo/micra/core"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func startRedis(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	rc, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() {
		if err := rc.Terminate(ctx); err != nil {
			t.Logf("terminate: %v", err)
		}
	})
	addr, err := rc.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return addr
}

func TestClient_HappyPath(t *testing.T) {
	ctx := context.Background()
	dsn := startRedis(t)

	c := micraredis.New(micraredis.WithDSN(dsn))
	if err := c.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	rdb := c.Redis()
	if rdb == nil {
		t.Fatal("Redis returned nil after Init")
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("PING after Init: %v", err)
	}

	startCtx, startCancel := context.WithCancel(ctx)
	startDone := make(chan error, 1)
	go func() { startDone <- c.Start(startCtx) }()
	time.Sleep(100 * time.Millisecond)
	startCancel()
	select {
	case err := <-startDone:
		if err != nil {
			t.Errorf("Start returned %v after ctx cancel, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return within 2s after ctx cancel")
	}

	stopCtx, stopCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stopCancel()
	if err := c.Stop(stopCtx); err != nil {
		t.Errorf("Stop: %v", err)
	}

	if err := rdb.Ping(ctx).Err(); err == nil {
		t.Error("PING after Stop succeeded, expected error from closed client")
	}
}

func TestClient_InitFails_WhenUnreachable(t *testing.T) {
	c := micraredis.New(
		micraredis.WithAddr("127.0.0.1:1"),
		micraredis.WithDialTimeout(500*time.Millisecond),
		micraredis.WithPingTimeout(500*time.Millisecond),
	)
	if err := c.Init(context.Background()); err == nil {
		t.Fatal("Init succeeded against unreachable Redis, want error")
	}
}

var _ core.Component = (*micraredis.Client)(nil)
