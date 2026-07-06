//go:build integration

package pgxpool_test

import (
	"context"
	"testing"
	"time"

	micrapool "github.com/siabroo/micra/components/pgxpool"
	"github.com/siabroo/micra/core"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPool_HappyPath(t *testing.T) {
	ctx := context.Background()

	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("test"),
		tcpostgres.WithUsername("u"),
		tcpostgres.WithPassword("p"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	defer func() {
		if err := pgC.Terminate(ctx); err != nil {
			t.Logf("terminate: %v", err)
		}
	}()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}

	p := micrapool.New(micrapool.WithDSN(dsn))
	if _, ok := any(p).(core.Initializer); !ok {
		t.Fatal("Pool does not implement core.Initializer")
	}
	if _, ok := any(p).(core.Component); !ok {
		t.Fatal("Pool does not implement core.Component")
	}

	if err := p.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	pool := p.DB()
	if pool == nil {
		t.Fatal("DB returned nil after Init")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var one int
	if err := pool.QueryRow(queryCtx, "SELECT 1").Scan(&one); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if one != 1 {
		t.Errorf("got %d, want 1", one)
	}

	startCtx, startCancel := context.WithCancel(ctx)
	startDone := make(chan error, 1)
	go func() { startDone <- p.Start(startCtx) }()
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
	if err := p.Stop(stopCtx); err != nil {
		t.Errorf("Stop: %v", err)
	}

	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err == nil {
		t.Error("query after Stop succeeded, expected error from closed pool")
	}
}

func TestPool_InitFails_WhenBadDSN(t *testing.T) {
	p := micrapool.New(
		micrapool.WithDSN("postgres://nobody:nope@127.0.0.1:1/none?sslmode=disable"),
		micrapool.WithConnectTimeout(500*time.Millisecond),
		micrapool.WithPingTimeout(500*time.Millisecond),
	)
	err := p.Init(context.Background())
	if err == nil {
		t.Fatal("Init succeeded with bad DSN, want error")
	}
}
