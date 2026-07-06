# adapters/otelinit

Install the three OTel globals every traced micra service needs:

1. `otel.SetTracerProvider(...)` — the SDK or noop provider you built.
2. `otel.SetTextMapPropagator(...)` — W3C TraceContext + Baggage so
   `traceparent` and `baggage` propagate over gRPC and HTTP headers.
3. A sampler — defaults to `ParentBased(TraceIDRatioBased(0.01))`.

The service builds its own exporter (OTLP, Cloud Trace, X-Ray, etc.)
and the SDK `TracerProvider` that wraps it; `otelinit` just installs
them globally.

## Example: OTLP exporter

```go
import (
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/resource"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

    "github.com/siabroo/micra/adapters/otelinit"
)

func main() {
    ctx := context.Background()

    exp, _ := otlptracegrpc.New(ctx)
    res, _ := resource.New(ctx,
        resource.WithAttributes(semconv.ServiceName("auth-go")),
    )
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(res),
    )

    shutdown, err := otelinit.Setup(ctx, otelinit.WithTracerProvider(tp))
    if err != nil { /* fail-fast */ }
    defer shutdown(ctx)

    // ... rest of main: micra App, components, etc.
}
```

## Why it must run before anything else

Without `propagation.TraceContext{}` installed, `otelgrpc` and
`otelhttp` will neither extract nor inject `traceparent`. The trace
will not span more than one service. Call `Setup` before constructing
any `grpcserver`, `grpcclient`, or HTTP client.
