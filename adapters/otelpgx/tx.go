package otelpgx

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Tx wraps a pgx.Tx returned by Pool.BeginTx. Every Query/QueryRow/
// Exec inside the transaction is sqlcommenter-prepended and spanned,
// matching Pool's behaviour. Commit and Rollback delegate to the
// inner tx without instrumentation — those are control-plane, not
// data-plane queries.
type Tx struct {
	inner pgx.Tx
	cfg   config
}

// Query opens a "db.query" span, prepends sqlcommenter, delegates.
func (t *Tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	sql, ctx, end := t.cfg.before(ctx, sql, "db.query")
	rows, err := t.inner.Query(ctx, sql, args...)
	end(err)
	return rows, err
}

// QueryRow opens a "db.query" span, prepends sqlcommenter, delegates.
func (t *Tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	sql, ctx, end := t.cfg.before(ctx, sql, "db.query")
	row := t.inner.QueryRow(ctx, sql, args...)
	end(nil)
	return row
}

// Exec opens a "db.exec" span, prepends sqlcommenter, delegates.
func (t *Tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	sql, ctx, end := t.cfg.before(ctx, sql, "db.exec")
	tag, err := t.inner.Exec(ctx, sql, args...)
	end(err)
	return tag, err
}

// SendBatch passthrough; see Pool.SendBatch for the same v0 note.
func (t *Tx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return t.inner.SendBatch(ctx, b)
}

// Commit delegates to the inner transaction.
func (t *Tx) Commit(ctx context.Context) error { return t.inner.Commit(ctx) }

// Rollback delegates to the inner transaction.
func (t *Tx) Rollback(ctx context.Context) error { return t.inner.Rollback(ctx) }
