# Correlation context design

**Date:** 2026-07-06
**Status:** Approved (design), pending implementation
**Repos touched:** `github.com/siabroo/micra` (library) + `nestjs-one` (gateway edge)

## Goal

Make logs and traces linkable, at three time scales, so an operator can pivot
between signals and follow a whole user session:

| Key | Where it lands | Scale | Origin |
|---|---|---|---|
| `request.id` | span attribute + log field `requestId` (already logged) | one request | micra (read `x-request-id` or generate) |
| `traceId` / `spanId` | log fields (new) | one request | the active `SpanContext` |
| `session.id` | span attribute + log field `sessionId` | one session | **pass-through** from the edge via OTel Baggage |

`traceId`/`spanId` are added to **logs only** (so a log line points at its exact
trace); they are NOT set as span attributes — the span already *is* the trace.

## Decisions (locked)

- **session_id is pass-through only.** Nobody mints it. If it is absent, no
  `session.id` attribute and no `sessionId` log field are written. The frontend
  (future) will own generating/holding it.
- **session_id propagates via OTel Baggage**, not a bespoke metadata header.
  Baggage is set once at the edge and rides every downstream hop automatically
  (micra's `otelinit` already installs the composite `TraceContext + Baggage`
  propagator); this matches "context that flows across the whole session".
  (`request.id` keeps its existing `x-request-id` metadata-header mechanism —
  unchanged.)
- **Extend the existing internal `RequestID` interceptor** (broaden its role to
  "correlation") rather than add a second interceptor — it already owns the
  correlation-id concern, computes `rid`, and tags the ctx logger.
- **Unary scope for this increment** — matches the current `RequestID`/`RPCLog`
  coverage. Streaming correlation is a separate follow-up.
- **Cardinality:** these ids live in traces and logs only, **never as metric
  labels** (already the case — metric labels are `job`/`rpc_method`/status).

## Data flow

```
frontend (future):  x-session-id header/cookie
  → api-gateway (Node):  read x-session-id → OTel Baggage(session.id)   [edge; pass-through: absent → no-op]
      → gRPC  (W3C baggage propagator, already installed)  — auto-propagates to every hop
          → micra grpcserver interceptor (unary):
              - span.SetAttributes( request.id=rid [, session.id=<baggage>] )
              - logger.With( traceId, spanId [, sessionId=<baggage>] )      // requestId already tagged
```

## Phase 1 — micra (`components/grpcserver`)

### Interceptor change (`internal/interceptors/requestid.go`)

Extend `RequestID()` so that, after it computes `rid` and before calling the
handler:

1. **Span attributes** — on the active span (`trace.SpanFromContext(ctx)`), when
   it is recording: set `request.id = rid`; and, if a `session.id` baggage member
   is present and non-empty, set `session.id = <value>`.
2. **Logger tags** — extend the existing `base.With("requestId", rid, "method", …)`
   to also include `traceId` and `spanId` from `trace.SpanContextFromContext(ctx)`
   when the context is valid, and `sessionId` when the `session.id` baggage member
   is present. Absent/empty values are omitted (no empty fields).
3. **Guards** — if the span is not recording, skip `SetAttributes`. If the
   `SpanContext` is not valid, omit `traceId`/`spanId`. If no `session.id`
   baggage, omit `session.id`/`sessionId`. All no-op-safe.

`RPCLog` needs no change — it logs through the ctx logger, which now carries the
extra fields.

### Reading baggage

```go
import "go.opentelemetry.io/otel/baggage"
sid := baggage.FromContext(ctx).Member("session.id").Value()  // "" if absent
```

### Dependencies

Promote to direct requires in `components/grpcserver/go.mod` (otel is already in
the module graph as indirect): `go.opentelemetry.io/otel/trace`,
`go.opentelemetry.io/otel/baggage`, `go.opentelemetry.io/otel/attribute`.

### Release

Additive, backward-compatible → minor `components/grpcserver@v0.4.0`.

## Phase 2 — gateway edge (`nestjs-one`, `services/api-gateway`)

Add a small NestJS interceptor/middleware that reads the `x-session-id` request
header and, if present, sets OTel Baggage `session.id` on the active context so
the OTel-instrumented gRPC client propagates it downstream. Pass-through: no
header → no baggage, no mint, no cookie (the frontend owns that later).

Then bump the gateway/services to `grpcserver@v0.4.0` so the services enrich
spans/logs from the propagated baggage.

## Naming & conventions

- Span attributes (dotted): `request.id`, `session.id`.
- Log fields (camelCase, matching existing `requestId`/`method`): `traceId`,
  `spanId`, `sessionId`.
- No new public API on `grpcserver` — `RequestID` is an internal interceptor
  wired by `grpcserver.New`.

## Testing

- **micra unit (TDD):** run the interceptor with (a) a recording span + a ctx
  carrying `session.id` baggage → assert the span has `request.id` and
  `session.id` attributes and the ctx logger emits `traceId`/`spanId`/`sessionId`;
  (b) no session baggage → assert no `session.id`/`sessionId` appears. Also a
  non-recording-span path (no panic, no attributes).
- **Live end-to-end:** a gRPC call through the gateway carrying `x-session-id` →
  the trace span shows `session.id`, the logs show `sessionId` + `traceId`, and
  Jaeger tag search `session.id=<value>` finds the trace. A call without the
  header shows `requestId` + `traceId` but no `sessionId`.

## Cloud portability

This is the exact enabler for GCP/AWS native log-trace correlation: once logs
carry `traceId`/`spanId` and the span carries `session.id`, moving to the
`googlecloud`/ADOT collector exporters maps them onto Cloud Logging's
`trace`/`spanId` fields (and span attributes), giving automatic correlation — no
code change in the services.

## Out of scope

- Minting session_id (frontend responsibility).
- Frontend session cookie handling.
- Streaming-RPC correlation (follow-up).
- Loki / Grafana derived-field pivot (separate deploy increment).
- Putting any correlation id on metrics.

## Rollout order

1. **micra:** extend `RequestID` + tests → PR → release `components/grpcserver@v0.4.0`.
2. **monorepo:** gateway `x-session-id → baggage` interceptor; bump services to
   `grpcserver@v0.4.0` → PR.
