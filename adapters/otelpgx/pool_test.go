package otelpgx

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
)

// fakeInner records the SQL it received for each call. It satisfies
// the same subset of methods otelpgx.Pool exposes, so we can substitute
// it via an interface without spinning up Postgres in unit tests.
type fakeInner struct {
	lastSQL string
	err     error
}

func (f *fakeInner) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.lastSQL = sql
	return nil, f.err
}
func (f *fakeInner) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	f.lastSQL = sql
	return errRow{f.err}
}
func (f *fakeInner) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.lastSQL = sql
	return pgconn.CommandTag{}, f.err
}
func (f *fakeInner) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil // batch tests covered separately
}
func (f *fakeInner) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	return nil, f.err // BeginTx tests use a dedicated fakeTx (see Task A4)
}

type errRow struct{ err error }

func (e errRow) Scan(dest ...any) error { return e.err }

func TestPool_Query_PrependsCommentWithApplicationAndTraceparent(t *testing.T) {
	inner := &fakeInner{}
	pool := wrapInner(inner, WithApplication("auth-go"))

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    mustTraceID("4bf92f3577b34da6a3ce929d0e0e4736"),
		SpanID:     mustSpanID("00f067aa0ba902b7"),
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	_, _ = pool.Query(ctx, "SELECT 1")

	if !strings.HasPrefix(inner.lastSQL, "/*application='auth-go',traceparent='00-") {
		t.Fatalf("missing comment prefix: %q", inner.lastSQL)
	}
	if !strings.HasSuffix(inner.lastSQL, "*/SELECT 1") {
		t.Fatalf("missing SQL suffix: %q", inner.lastSQL)
	}
}

func TestPool_Query_NoCommentWhenNoSpan(t *testing.T) {
	inner := &fakeInner{}
	pool := wrapInner(inner)

	_, _ = pool.Query(context.Background(), "SELECT 1")

	if inner.lastSQL != "SELECT 1" {
		t.Fatalf("unexpected SQL: %q", inner.lastSQL)
	}
}

func TestPool_QueryRow_PrependsComment(t *testing.T) {
	inner := &fakeInner{}
	pool := wrapInner(inner, WithApplication("svc"))

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: mustTraceID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		SpanID:  mustSpanID("bbbbbbbbbbbbbbbb"),
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	_ = pool.QueryRow(ctx, "SELECT name FROM t WHERE id=$1", 7).Scan(new(string))

	if !strings.Contains(inner.lastSQL, "application='svc'") {
		t.Fatalf("comment missing: %q", inner.lastSQL)
	}
}

func TestPool_Exec_PrependsComment_AndReadsRouteBaggage(t *testing.T) {
	inner := &fakeInner{}
	pool := wrapInner(inner)

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    mustTraceID("dddddddddddddddddddddddddddddddd"),
		SpanID:     mustSpanID("eeeeeeeeeeeeeeee"),
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	m, _ := baggage.NewMember("micra.route", "/svc.S/M")
	bg, _ := baggage.New(m)
	ctx = baggage.ContextWithBaggage(ctx, bg)

	_, _ = pool.Exec(ctx, "DELETE FROM t WHERE id=$1", 7)

	if !strings.Contains(inner.lastSQL, "route='%2Fsvc.S%2FM'") {
		t.Fatalf("route missing: %q", inner.lastSQL)
	}
}

func mustTraceID(s string) trace.TraceID {
	id, err := trace.TraceIDFromHex(s)
	if err != nil {
		panic(err)
	}
	return id
}
func mustSpanID(s string) trace.SpanID {
	id, err := trace.SpanIDFromHex(s)
	if err != nil {
		panic(err)
	}
	return id
}
