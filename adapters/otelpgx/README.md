# adapters/otelpgx

Wrap a `*pgxpool.Pool` so every query emits an OTel CLIENT span and
carries a Google sqlcommenter prefix linking it back to the parent
trace.

```go
import (
    micrapool "github.com/siabroo/micra/components/pgxpool"
    "github.com/siabroo/micra/adapters/otelpgx"
)

raw := micrapool.New(micrapool.WithDSN(dsn))
// ... raw.Init(ctx) ...
db := otelpgx.WrapPool(raw.DB(), otelpgx.WithApplication("auth-go"))
// Pass db to repositories; they accept otelpgx.DBTX so the raw
// pool also works in tests where OTel is irrelevant.
```

## What lands in Postgres

```sql
/*application='auth-go',route='%2Fauth.Auth%2FSignIn',traceparent='00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01'*/
SELECT id, email FROM users WHERE id = $1
```

GCP Cloud SQL Insights parses `traceparent` and links to Cloud Trace.
On any cloud, the comment is greppable in `pg_stat_activity.query`
and slow-query logs.

If the context has no valid OTel SpanContext (NoopTracerProvider or
unsampled), the comment is omitted entirely. The query goes out
unchanged — zero-cost for opt-out callers.

## Route baggage

`route` is read from OTel baggage key `micra.route` by default. Set it
at the request boundary so when service A → B → DB, B reports A's route
(SQL Insights attributes the query correctly).

gRPC server example. Note the **set-if-absent** check: when A calls B,
A's baggage already carries `micra.route=/a.A/CallB`. B must NOT
overwrite it — otherwise SQL Insights reports B's local method for the
DB query that A originated.

```go
import "go.opentelemetry.io/otel/baggage"

func RouteBaggage(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
    bg := baggage.FromContext(ctx)
    if bg.Member("micra.route").Value() == "" {
        m, _ := baggage.NewMember("micra.route", info.FullMethod)
        bg, _ = bg.SetMember(m)
        ctx = baggage.ContextWithBaggage(ctx, bg)
    }
    return h(ctx, req)
}

// then:
grpcserver.WithUnaryInterceptors(otelgrpc.UnaryServerInterceptor(), RouteBaggage)
```

`otelgrpc.UnaryServerInterceptor()` is the legacy API; the modern
recommendation is `grpc.StatsHandler(otelgrpc.NewServerHandler())`
passed via `WithGRPCServerOptions`. Both work — interceptor form fits
micra's existing chain so the example uses it.

HTTP example: wrap your handler and stamp `r.URL.Path` similarly.

## Transactions

`Pool.BeginTx(ctx, opts)` returns an `*otelpgx.Tx` whose Query/QueryRow/
Exec methods carry the same sqlcommenter prefix and span emission as
the Pool itself. Commit/Rollback delegate plainly. Repositories that
accept the `DBTX` interface work uniformly with either Pool or Tx —
the service layer chooses which to hand them per call.

```go
tx, err := db.BeginTx(ctx, pgx.TxOptions{})
if err != nil { return err }
defer tx.Rollback(ctx)

if err := repo.UpsertUser(ctx, tx, u); err != nil { return err } // tx satisfies DBTX
if err := repo.InsertAudit(ctx, tx, evt); err != nil { return err }
return tx.Commit(ctx)
```

## Override the extractor

If your service uses a different baggage convention or a different
ctx-stored value, pass `otelpgx.WithRouteFromContext(fn)`.

## Options

| Option | Default | Purpose |
|---|---|---|
| `WithApplication(string)` | "" (omitted) | Static service name in the comment. |
| `WithRouteFromContext(fn)` | reads baggage `micra.route` | Override extractor. |
| `WithTracerProvider(tp)` | `otel.GetTracerProvider()` | Override for tests. |

## What this module does NOT do

- Configure the OTel SDK or exporter (use `adapters/otelinit`).
- Set the W3C propagator (use `adapters/otelinit`).
- Log queries. The premise: queries are observable via traces, not
  logs. Adding a `WithQueryLogging` option would re-introduce the cost
  this design exists to avoid.
