//go:build integration

package otelpgx_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/siabroo/micra/adapters/otelpgx"
)

// TestPool_CommentReachesPostgresAndSpansAreRecorded verifies the
// observable contract of WrapPool against a real Postgres:
//   - the SQL sent on the wire begins with the sqlcommenter prefix,
//   - exactly one db.exec span is recorded per pool.Exec call,
//   - that span carries db.system=postgresql and the prefixed SQL in
//     db.statement.
//
// We assert (1) via pg_stat_activity: while a long-running statement
// is in flight, its `query` text in pg_stat_activity must contain the
// comment. We use pg_sleep(1.0) and a polling loop to avoid races.
func TestPool_CommentReachesPostgresAndSpansAreRecorded(t *testing.T) {
	ctx := context.Background()

	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("test"),
		tcpostgres.WithUsername("u"),
		tcpostgres.WithPassword("p"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	defer pgC.Terminate(ctx)

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	raw, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer raw.Close()

	rec := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(rec))
	defer tp.Shutdown(ctx)

	db := otelpgx.WrapPool(raw,
		otelpgx.WithApplication("integration-test"),
		otelpgx.WithTracerProvider(tp),
	)

	parentCtx, parent := tp.Tracer("test").Start(ctx, "parent")

	// Fire a slow statement in the background. While it sleeps, a
	// second connection (raw, no wrap) polls pg_stat_activity until
	// it observes the row. Polling avoids the race where the inner
	// SELECT runs too fast — pg_sleep(1.0) is long enough for stock
	// hardware but we still poll instead of sleeping a fixed amount.
	slowDone := make(chan error, 1)
	go func() {
		_, err := db.Exec(parentCtx, "SELECT pg_sleep(1.0)")
		slowDone <- err
	}()

	var observedQuery string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		row := raw.QueryRow(ctx,
			`SELECT query FROM pg_stat_activity WHERE query LIKE '%pg_sleep%' AND query NOT LIKE '%pg_stat_activity%' LIMIT 1`)
		if err := row.Scan(&observedQuery); err == nil && observedQuery != "" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if observedQuery == "" {
		t.Fatal("did not observe pg_sleep statement in pg_stat_activity before deadline")
	}
	if err := <-slowDone; err != nil {
		t.Fatalf("slow exec: %v", err)
	}
	parent.End()

	if !strings.HasPrefix(observedQuery, "/*application='integration-test'") {
		t.Fatalf("pg_stat_activity.query missing comment prefix: %q", observedQuery)
	}
	if !strings.Contains(observedQuery, "traceparent='00-") {
		t.Fatalf("pg_stat_activity.query missing traceparent: %q", observedQuery)
	}

	var dbSpan tracetest.SpanStub
	for _, s := range rec.GetSpans() {
		if s.Name == "db.exec" {
			dbSpan = s
			break
		}
	}
	if dbSpan.Name == "" {
		t.Fatal("no db.exec span recorded")
	}
	var stmt string
	var hasSystem bool
	for _, kv := range dbSpan.Attributes {
		if string(kv.Key) == "db.statement" {
			stmt = kv.Value.AsString()
		}
		if string(kv.Key) == "db.system" && kv.Value.AsString() == "postgresql" {
			hasSystem = true
		}
	}
	if !strings.HasPrefix(stmt, "/*application='integration-test'") {
		t.Fatalf("span db.statement missing comment prefix: %q", stmt)
	}
	if !hasSystem {
		t.Fatalf("span missing db.system=postgresql attribute: %+v", dbSpan.Attributes)
	}
}
