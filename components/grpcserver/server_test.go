package grpcserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/siabroo/micra/components/grpcserver"
	"github.com/siabroo/micra/core"
)

func TestServer_ImplementsComponentAndInitializer(t *testing.T) {
	s := grpcserver.New(grpcserver.WithAddr(":0"), grpcserver.WithRegister(func(*grpc.Server) {}))
	if _, ok := any(s).(core.Component); !ok {
		t.Error("Server does not implement core.Component")
	}
	if _, ok := any(s).(core.Initializer); !ok {
		t.Error("Server does not implement core.Initializer")
	}
}

func TestServer_HealthCheck_Serving(t *testing.T) {
	// Use a bufconn for hermetic test; replace Listen via a Listener option.
	bufLis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { bufLis.Close() })

	srv := grpcserver.New(
		grpcserver.WithListener(bufLis),
		grpcserver.WithRegister(func(*grpc.Server) {}),
		grpcserver.WithReflection(false),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	startDone := make(chan error, 1)
	go func() { startDone <- srv.Start(ctx) }()

	// Dial via bufconn.
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return bufLis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer conn.Close()

	check, err := grpc_health_v1.NewHealthClient(conn).Check(ctx,
		&grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if check.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Errorf("status = %v, want SERVING", check.Status)
	}

	cancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := srv.Stop(stopCtx); err != nil {
		t.Errorf("Stop: %v", err)
	}
	select {
	case err := <-startDone:
		if err != nil {
			t.Errorf("Start returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return within 2s of Stop")
	}
}
