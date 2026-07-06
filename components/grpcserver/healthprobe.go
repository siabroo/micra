package grpcserver

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// HealthProbe dials addr, calls grpc.health.v1.Health/Check, and
// returns 0 if the server reports SERVING, 1 otherwise.
//
// Designed as the body of a -healthcheck binary mode for distroless
// Docker images, which cannot host an external probe tool. The
// canonical usage at the top of main is:
//
//	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
//	    os.Exit(grpcserver.HealthProbe("127.0.0.1:" + port))
//	}
//
// The probe uses insecure transport credentials — appropriate for
// loopback inside a container. It does not verify a specific service
// name; the empty-string ("overall") status is what micra's Server
// sets to SERVING after the listener is open, which is the right
// semantic for "container is ready to take traffic".
func HealthProbe(addr string) int {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return 1
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil || resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		return 1
	}
	return 0
}
