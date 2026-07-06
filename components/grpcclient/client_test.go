package grpcclient_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/siabroo/micra/components/grpcclient"
	"github.com/siabroo/micra/core"
)

func TestClient_HappyPath_DialsAndExposesConn(t *testing.T) {
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go srv.Serve(lis)
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
