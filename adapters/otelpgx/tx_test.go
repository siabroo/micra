package otelpgx

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/trace"
)

// fakeTx records SQL sent through the transaction and counts commit /
// rollback calls. Satisfies pgx.Tx for the subset Tx delegates to.
type fakeTx struct {
	lastSQL   string
	commits   int
	rollbacks int
	err       error
}

func (f *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) { return nil, errors.New("nested begin not used") }
func (f *fakeTx) BeginFunc(ctx context.Context, fn func(pgx.Tx) error) error {
	return errors.New("BeginFunc not used")
}
func (f *fakeTx) Commit(ctx context.Context) error   { f.commits++; return f.err }
func (f *fakeTx) Rollback(ctx context.Context) error { f.rollbacks++; return f.err }
func (f *fakeTx) CopyFrom(ctx context.Context, table pgx.Identifier, cols []string, src pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("CopyFrom not used")
}
func (f *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (f *fakeTx) LargeObjects() pgx.LargeObjects                                { return pgx.LargeObjects{} }
func (f *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("Prepare not used")
}
func (f *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastSQL = sql
	return pgconn.CommandTag{}, f.err
}
func (f *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastSQL = sql
	return nil, f.err
}
func (f *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	f.lastSQL = sql
	return errRow{f.err}
}
func (f *fakeTx) Conn() *pgx.Conn { return nil }

// txFakeInner extends fakeInner so BeginTx returns a fakeTx instead of
// the nil/zero-value default. Lets Pool.BeginTx be tested end-to-end.
type txFakeInner struct {
	fakeInner
	tx *fakeTx
}

func (t *txFakeInner) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	return t.tx, nil
}

func TestPool_BeginTx_ReturnsWrappedTxThatPrependsComment(t *testing.T) {
	inner := &txFakeInner{tx: &fakeTx{}}
	pool := wrapInner(inner, WithApplication("auth-go"))

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    mustTraceID("4bf92f3577b34da6a3ce929d0e0e4736"),
		SpanID:     mustSpanID("00f067aa0ba902b7"),
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}

	_, _ = tx.Exec(ctx, "DELETE FROM t WHERE id=$1", 7)

	if !strings.HasPrefix(inner.tx.lastSQL, "/*application='auth-go'") {
		t.Fatalf("tx.Exec missing comment prefix: %q", inner.tx.lastSQL)
	}
	if !strings.HasSuffix(inner.tx.lastSQL, "*/DELETE FROM t WHERE id=$1") {
		t.Fatalf("tx.Exec missing SQL suffix: %q", inner.tx.lastSQL)
	}
}

func TestTx_Commit_DelegatesAndDoesNotInstrument(t *testing.T) {
	inner := &txFakeInner{tx: &fakeTx{}}
	pool := wrapInner(inner)
	tx, _ := pool.BeginTx(context.Background(), pgx.TxOptions{})

	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if inner.tx.commits != 1 {
		t.Fatalf("commits = %d, want 1", inner.tx.commits)
	}
}

func TestTx_Rollback_Delegates(t *testing.T) {
	inner := &txFakeInner{tx: &fakeTx{}}
	pool := wrapInner(inner)
	tx, _ := pool.BeginTx(context.Background(), pgx.TxOptions{})

	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if inner.tx.rollbacks != 1 {
		t.Fatalf("rollbacks = %d, want 1", inner.tx.rollbacks)
	}
}

func TestTx_QueryRow_PrependsComment(t *testing.T) {
	inner := &txFakeInner{tx: &fakeTx{}}
	pool := wrapInner(inner, WithApplication("svc"))

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: mustTraceID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanID:  mustSpanID("bbbbbbbbbbbbbbbb"),
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	tx, _ := pool.BeginTx(ctx, pgx.TxOptions{})

	_ = tx.QueryRow(ctx, "SELECT 1").Scan(new(int))

	if !strings.Contains(inner.tx.lastSQL, "application='svc'") {
		t.Fatalf("tx.QueryRow missing comment: %q", inner.tx.lastSQL)
	}
}
