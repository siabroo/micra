package grpcclient_test

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
	"google.golang.org/grpc/test/bufconn"

	"github.com/siabroo/micra/components/grpcclient"
	"github.com/siabroo/micra/core"
)

func TestClient_HappyPath_DialsAndExposesConn(t *testing.T) {
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }

	c := grpcclient.New(
		grpcclient.WithName("buf"),
		grpcclient.WithTarget("passthrough:///bufnet"),
		grpcclient.WithDialOptions(
			grpc.WithContextDialer(dialer),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)

	if _, ok := any(c).(core.Initializer); !ok {
		t.Fatal("Client does not implement core.Initializer")
	}
	if _, ok := any(c).(core.Component); !ok {
		t.Fatal("Client does not implement core.Component")
	}

	if err := c.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	cli := healthpb.NewHealthClient(c.Conn())
	cctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := cli.Check(cctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health Check: %v", err)
	}
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("status: %v", resp.GetStatus())
	}

	startCtx, startCancel := context.WithCancel(context.Background())
	startDone := make(chan error, 1)
	go func() { startDone <- c.Start(startCtx) }()
	time.Sleep(50 * time.Millisecond)
	startCancel()
	select {
	case err := <-startDone:
		if err != nil {
			t.Errorf("Start returned %v after ctx cancel, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return")
	}

	if err := c.Stop(context.Background()); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestClient_Init_FailsWithoutTarget(t *testing.T) {
	c := grpcclient.New(grpcclient.WithName("x"))
	if err := c.Init(context.Background()); err == nil {
		t.Fatal("Init succeeded without WithTarget, want error")
	}
}

func TestClient_Conn_PanicsBeforeInit(t *testing.T) {
	c := grpcclient.New(grpcclient.WithTarget("x"))
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Conn() before Init did not panic")
		}
	}()
	_ = c.Conn()
}

// TestClient_WithRoundRobin_DistributesAcrossBackends proves the option
// installs the round_robin policy: with two resolved backends, RPCs land on
// both. Under the default pick_first policy all RPCs would pin to one.
func TestClient_WithRoundRobin_DistributesAcrossBackends(t *testing.T) {
	newBackend := func() (*bufconn.Listener, *int32) {
		lis := bufconn.Listen(1 << 16)
		var count int32
		s := grpc.NewServer(grpc.ChainUnaryInterceptor(
			func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
				atomic.AddInt32(&count, 1)
				return h(ctx, req)
			}))
		healthpb.RegisterHealthServer(s, health.NewServer())
		go func() { _ = s.Serve(lis) }()
		t.Cleanup(s.Stop)
		return lis, &count
	}

	lis1, c1 := newBackend()
	lis2, c2 := newBackend()

	r := manual.NewBuilderWithScheme("rrtest")
	r.InitialState(resolver.State{Addresses: []resolver.Address{
		{Addr: "backend1"},
		{Addr: "backend2"},
	}})

	dialer := func(_ context.Context, addr string) (net.Conn, error) {
		switch addr {
		case "backend1":
			return lis1.Dial()
		case "backend2":
			return lis2.Dial()
		}
		return nil, fmt.Errorf("unknown backend %q", addr)
	}

	c := grpcclient.New(
		grpcclient.WithName("rr"),
		grpcclient.WithTarget(r.Scheme()+":///backends"),
		grpcclient.WithRoundRobin(),
		grpcclient.WithDialOptions(
			grpc.WithResolvers(r),
			grpc.WithContextDialer(dialer),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	if err := c.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = c.Stop(context.Background()) }()

	// Let both subconns reach READY — round_robin only spreads across
	// ready backends.
	time.Sleep(200 * time.Millisecond)

	cli := healthpb.NewHealthClient(c.Conn())
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, err := cli.Check(ctx, &healthpb.HealthCheckRequest{}, grpc.WaitForReady(true))
		cancel()
		if err != nil {
			t.Fatalf("Check %d: %v", i, err)
		}
	}

	if atomic.LoadInt32(c1) == 0 || atomic.LoadInt32(c2) == 0 {
		t.Errorf("round_robin did not distribute: backend1=%d backend2=%d",
			atomic.LoadInt32(c1), atomic.LoadInt32(c2))
	}
}
