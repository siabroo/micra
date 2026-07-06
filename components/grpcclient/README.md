# components/grpcclient

A `*grpc.ClientConn` as a micra `core.Component` + `core.Initializer`.
Symmetric to `components/grpcserver`. No OTel dependency.

```go
import (
    "github.com/siabroo/micra/components/grpcclient"
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

cli := grpcclient.New(
    grpcclient.WithName("auth"),
    grpcclient.WithTarget("dns:///auth:50051"),
    grpcclient.WithUnaryInterceptors(otelgrpc.UnaryClientInterceptor()),
)

// in main:
app.Register(cli)
// in handlers:
authClient := authpb.NewAuthClient(cli.Conn())
```

`grpc.NewClient` does no I/O at construction time — connection errors
surface from the first RPC, not from `Init`. That's standard gRPC v1.62+
behaviour and the right shape for fast Init.

## Options

| Option | Required | Purpose |
|---|---|---|
| `WithTarget(string)` | yes | Dial target (host:port, dns:///..., passthrough:///...). |
| `WithName(string)` | no | Component name. Default "grpc-client". |
| `WithDialOptions(...grpc.DialOption)` | no | Raw dial opts (credentials, keepalive, etc.). |
| `WithRoundRobin()` | no | Set the LB policy to round_robin (default is pick_first). |
| `WithUnaryInterceptors(...)` | no | Chain order of arguments. |
| `WithStreamInterceptors(...)` | no | Chain order of arguments. |

## Load balancing across pods (Kubernetes)

gRPC's default `pick_first` policy opens one long-lived HTTP/2 connection and
pins every RPC to a single backend pod — so it neither spreads load nor
follows a rolling update. To balance client-side, point the target at a
**headless Service** (so DNS returns pod IPs, not one ClusterIP) and enable
round_robin:

```go
grpcclient.New(
    grpcclient.WithTarget("dns:///auth-headless.prod.svc.cluster.local:50051"),
    grpcclient.WithRoundRobin(),
)
```

The server side pairs with `grpcserver`'s graceful `Stop`, which flips the
health status to `NOT_SERVING` before draining and sends GOAWAY so clients
re-resolve onto the new pods.
