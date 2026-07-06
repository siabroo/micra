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
| `WithUnaryInterceptors(...)` | no | Chain order of arguments. |
| `WithStreamInterceptors(...)` | no | Chain order of arguments. |
