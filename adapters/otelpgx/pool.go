// Package otelpgx wraps a *pgxpool.Pool with two cross-cutting
// concerns: sqlcommenter prefix injection (so Cloud SQL Insights and
// any Postgres slow-query log link the query to a trace id) and OTel
// span emission per query (so each query shows up as a child span of
// the request).
//
// Optionality: this module depends only on the OTel API
// (go.opentelemetry.io/otel*). It does NOT pull the OTel SDK. A
// service that imports adapters/otelpgx but does not initialise a
// TracerProvider gets the OTel default (NoopTracerProvider): zero-cost
// spans and no sqlcommenter comment is emitted (the SpanContext is
// invalid). Wiring the SDK is the service's job (see
// adapters/otelinit).
package otelpgx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is the public interface repository code holds. Both *otelpgx.Pool
// and *otelpgx.Tx satisfy it, so a repository accepts either a long-lived
// pool or a transaction-scoped object without caring which is in flight.
// The raw *pgxpool.Pool also satisfies DBTX, so tests can fall back to
// the unwrapped pool when OTel is irrelevant.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// poolBackend is the internal seam Pool wraps. It extends DBTX with
// BeginTx so the wrapper can start transactions; unexported because
// repositories should not start transactions through DBTX (they get a
// Tx handed to them by the service layer).
type poolBackend interface {
	DBTX
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// Pool wraps a pgx pool backend and prepends sqlcommenter on every call.
type Pool struct {
	inner poolBackend
	cfg   config
}

// WrapPool returns a Pool that intercepts queries on the given pgxpool
// pool. Apply functional options for application name, route extractor,
// and tracer provider override.
func WrapPool(p *pgxpool.Pool, opts ...Option) *Pool {
	return wrapInner(p, opts...)
}

// wrapInner is the testable seam — accepts any poolBackend.
func wrapInner(inner poolBackend, opts ...Option) *Pool {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}
	return &Pool{inner: inner, cfg: cfg}
}

// Query opens a "db.query" span, prepends sqlcommenter to sql, and
// delegates to the inner pool. The returned Rows are unchanged.
func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	sql, ctx, end := p.cfg.before(ctx, sql, "db.query")
	rows, err := p.inner.Query(ctx, sql, args...)
	end(err)
	return rows, err
}

// QueryRow opens a "db.query" span, prepends sqlcommenter, delegates.
// QueryRow defers errors to Scan; the span ends here on the wire-level
// call so the trace duration reflects time spent reaching Postgres.
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	sql, ctx, end := p.cfg.before(ctx, sql, "db.query")
	row := p.inner.QueryRow(ctx, sql, args...)
	end(nil)
	return row
}

// Exec opens a "db.exec" span, prepends sqlcommenter, delegates.
func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	sql, ctx, end := p.cfg.before(ctx, sql, "db.exec")
	tag, err := p.inner.Exec(ctx, sql, args...)
	end(err)
	return tag, err
}

// SendBatch returns the underlying BatchResults unchanged; per-query
// comment injection inside a batch would require wrapping
// pgx.BatchResults and is deferred to a follow-up (see spec §11).
func (p *Pool) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return p.inner.SendBatch(ctx, b)
}

// BeginTx starts a transaction and returns a *Tx wrapper. Statements
// run through the wrapper inherit Pool's sqlcommenter prefix and
// span emission. Pass the wrapper to repository code via the DBTX
// interface so it accepts either Pool or Tx transparently.
func (p *Pool) BeginTx(ctx context.Context, opts pgx.TxOptions) (*Tx, error) {
	rawTx, err := p.inner.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{inner: rawTx, cfg: p.cfg}, nil
}
