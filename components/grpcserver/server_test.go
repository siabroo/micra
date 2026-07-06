package grpcserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
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
	t.Cleanup(func() { _ = bufLis.Close() })

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
	defer func() { _ = conn.Close() }()

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

// TestServer_Stop_FlipsHealthToNotServing asserts Stop flips the health
// status to NOT_SERVING *before* draining, so a readiness probe fails and
// the pod is pulled from the Service EndpointSlice ahead of GracefulStop.
// The transition is observed over an open Health/Watch stream.
func TestServer_Stop_FlipsHealthToNotServing(t *testing.T) {
	bufLis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = bufLis.Close() })

	srv := grpcserver.New(
		grpcserver.WithListener(bufLis),
		grpcserver.WithRegister(func(*grpc.Server) {}),
		grpcserver.WithReflection(false),
	)

	srvCtx, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()
	if err := srv.Init(srvCtx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	startDone := make(chan error, 1)
	go func() { startDone <- srv.Start(srvCtx) }()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return bufLis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() { _ = conn.Close() }()

	watchCtx, watchCancel := context.WithCancel(context.Background())
	defer watchCancel()
	stream, err := grpc_health_v1.NewHealthClient(conn).Watch(watchCtx,
		&grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	first, err := stream.Recv()
	if err != nil {
		t.Fatalf("first Recv: %v", err)
	}
	if first.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("first status = %v, want SERVING", first.Status)
	}

	// Begin graceful shutdown. The open Watch stream keeps GracefulStop
	// blocked, giving us a window to observe the pre-drain flip.
	srvCancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	stopDone := make(chan error, 1)
	go func() { stopDone <- srv.Stop(stopCtx) }()

	got, err := stream.Recv()
	if err != nil {
		t.Fatalf("second Recv (expected NOT_SERVING flip): %v", err)
	}
	if got.Status != grpc_health_v1.HealthCheckResponse_NOT_SERVING {
		t.Errorf("status after Stop = %v, want NOT_SERVING", got.Status)
	}

	// End the streaming RPC so GracefulStop can complete promptly.
	watchCancel()
	select {
	case err := <-stopDone:
		if err != nil {
			t.Errorf("Stop returned %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return")
	}
	select {
	case <-startDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return")
	}
}

// TestServer_ReflectionOffByDefault asserts that a server created with no
// explicit WithReflection option does NOT register the gRPC reflection service.
// Reflection is a reconnaissance tool and must require explicit opt-in.
func TestServer_ReflectionOffByDefault(t *testing.T) {
	bufLis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = bufLis.Close() })

	// No WithReflection option → must default to OFF.
	srv := grpcserver.New(
		grpcserver.WithListener(bufLis),
		grpcserver.WithRegister(func(*grpc.Server) {}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	go func() { _ = srv.Start(ctx) }()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return bufLis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Attempt to list services via reflection — must fail with Unimplemented.
	refClient := grpc_reflection_v1.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		t.Fatalf("ServerReflectionInfo open: %v", err)
	}
	_ = stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	})
	_, recvErr := stream.Recv()
	if recvErr == nil {
		t.Fatal("reflection responded successfully — default must be OFF (reflection: false)")
	}
	if code := status.Code(recvErr); code != codes.Unimplemented {
		t.Errorf("expected codes.Unimplemented from disabled reflection, got %v", code)
	}
}

// TestServer_ReflectionEnabledWithOption asserts that WithReflection(true)
// makes the reflection service reachable.
func TestServer_ReflectionEnabledWithOption(t *testing.T) {
	bufLis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = bufLis.Close() })

	srv := grpcserver.New(
		grpcserver.WithListener(bufLis),
		grpcserver.WithRegister(func(*grpc.Server) {}),
		grpcserver.WithReflection(true),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	go func() { _ = srv.Start(ctx) }()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return bufLis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer func() { _ = conn.Close() }()

	refClient := grpc_reflection_v1.NewServerReflectionClient(conn)
	stream, err := refClient.ServerReflectionInfo(ctx)
	if err != nil {
		t.Fatalf("ServerReflectionInfo open: %v", err)
	}
	if sendErr := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	}); sendErr != nil {
		t.Fatalf("Send: %v", sendErr)
	}
	resp, recvErr := stream.Recv()
	if recvErr != nil {
		t.Fatalf("reflection service unreachable with WithReflection(true): %v", recvErr)
	}
	if resp == nil {
		t.Fatal("nil response from reflection service")
	}
}
