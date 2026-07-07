package interceptors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/baggage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/siabroo/micra/core"
)

func TestRequestID_PropagatesIncoming(t *testing.T) {
	md := metadata.New(map[string]string{"x-request-id": "supplied-id"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	called := false
	_, _ = RequestID()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(c context.Context, _ any) (any, error) {
			called = true
			// outgoing metadata should carry the same request-id
			om, _ := metadata.FromOutgoingContext(c)
			if got := om.Get("x-request-id"); len(got) == 0 || got[0] != "supplied-id" {
				t.Errorf("outgoing x-request-id = %v, want [supplied-id]", got)
			}
			return nil, nil
		})
	if !called {
		t.Fatal("handler was not invoked")
	}
}

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	ctx := context.Background()
	captured := ""
	_, _ = RequestID()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(c context.Context, _ any) (any, error) {
			om, _ := metadata.FromOutgoingContext(c)
			ids := om.Get("x-request-id")
			if len(ids) > 0 {
				captured = ids[0]
			}
			return nil, nil
		})
	if captured == "" {
		t.Fatal("no x-request-id generated")
	}
}

func TestRequestID_TagsLoggerInContext(t *testing.T) {
	rec := &recordingLogger{}
	ctx := core.ContextWithLogger(context.Background(), rec)

	_, _ = RequestID()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(c context.Context, _ any) (any, error) {
			// inside the handler, LoggerFrom returns the tagged logger
			l := core.LoggerFrom(c)
			if l == rec {
				t.Error("logger in ctx was not replaced with a tagged variant")
			}
			return nil, nil
		})

	if len(rec.withCalls) == 0 {
		t.Fatal("expected With to be called on the base logger")
	}
	first := rec.withCalls[0]
	if !containsKey(first, "requestId") || !containsKey(first, "method") {
		t.Errorf("With args missing requestId/method: %v", first)
	}
}

func TestRPCLog_AlwaysLogsRPCEnd(t *testing.T) {
	rec := &recordingLogger{enabled: false}
	ctx := core.ContextWithLogger(context.Background(), rec)
	_, _ = RPCLog(Config{PayloadLogging: false, PayloadMaxBytes: 200})(
		ctx, "req-body", &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(c context.Context, _ any) (any, error) { return "resp-body", nil })
	if len(rec.infoCalls) == 0 {
		t.Fatal("expected at least one Info call (rpc.end)")
	}
	last := rec.infoCalls[len(rec.infoCalls)-1]
	if last.msg != "rpc.end" {
		t.Errorf("last Info msg = %q, want rpc.end", last.msg)
	}
}

func TestRPCLog_SkipsPayloadWhenDisabled(t *testing.T) {
	rec := &recordingLogger{enabled: true}
	ctx := core.ContextWithLogger(context.Background(), rec)
	_, _ = RPCLog(Config{PayloadLogging: false, PayloadMaxBytes: 200})(
		ctx, "req-body", &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(c context.Context, _ any) (any, error) { return "resp-body", nil })
	for _, c := range rec.debugCalls {
		if strings.Contains(c.msg, "rpc.start") {
			t.Errorf("expected no rpc.start at debug, got: %+v", c)
		}
	}
}

func TestRPCLog_SkipsPayloadWhenLoggerDisabled(t *testing.T) {
	rec := &recordingLogger{enabled: false}
	ctx := core.ContextWithLogger(context.Background(), rec)
	_, _ = RPCLog(Config{PayloadLogging: true, PayloadMaxBytes: 200})(
		ctx, "req-body", &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(c context.Context, _ any) (any, error) { return "resp-body", nil })
	if len(rec.debugCalls) > 0 {
		t.Errorf("expected no debug calls when logger disabled; got %d", len(rec.debugCalls))
	}
}

func TestRPCLog_EmitsPayloadWhenAllowed(t *testing.T) {
	rec := &recordingLogger{enabled: true}
	ctx := core.ContextWithLogger(context.Background(), rec)
	_, _ = RPCLog(Config{PayloadLogging: true, PayloadMaxBytes: 200})(
		ctx, "req-body", &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(c context.Context, _ any) (any, error) { return "resp-body", nil })
	if len(rec.debugCalls) == 0 {
		t.Fatal("expected rpc.start at debug")
	}
}

func TestRecovery_ConvertsPanicToInternal(t *testing.T) {
	_, err := Recovery()(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(context.Context, any) (any, error) {
			panic("boom")
		})
	if err == nil {
		t.Fatal("expected non-nil error after panic")
	}
}

// TestRecovery_ReturnsGenericMessage asserts that the unary recovery interceptor
// does NOT leak the panic value to the caller; the client should only see a
// fixed "internal error" string.
func TestRecovery_ReturnsGenericMessage(t *testing.T) {
	_, err := Recovery()(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(context.Context, any) (any, error) {
			panic("sensitive-internal-detail")
		})
	if err == nil {
		t.Fatal("expected non-nil error after panic")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Errorf("code = %v, want Internal", st.Code())
	}
	if strings.Contains(st.Message(), "sensitive-internal-detail") {
		t.Errorf("panic value leaked to client: %q", st.Message())
	}
	const wantMsg = "internal error"
	if st.Message() != wantMsg {
		t.Errorf("message = %q, want %q", st.Message(), wantMsg)
	}
}

// mockServerStream is a minimal grpc.ServerStream for use in tests.
type mockServerStream struct {
	ctx context.Context
}

func (m *mockServerStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(metadata.MD)       {}
func (m *mockServerStream) Context() context.Context     { return m.ctx }
func (m *mockServerStream) SendMsg(any) error            { return nil }
func (m *mockServerStream) RecvMsg(any) error            { return nil }

// TestStreamRecovery_ConvertsPanicToInternal asserts that a panic in a
// streaming handler is caught and converted to codes.Internal rather than
// crashing the process.
func TestStreamRecovery_ConvertsPanicToInternal(t *testing.T) {
	ss := &mockServerStream{ctx: context.Background()}
	err := StreamRecovery()(nil, ss, &grpc.StreamServerInfo{FullMethod: "/svc/Stream"},
		func(_ any, _ grpc.ServerStream) error {
			panic("stream-panic")
		})
	if err == nil {
		t.Fatal("expected non-nil error after stream panic")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Errorf("code = %v, want Internal", st.Code())
	}
}

// TestStreamRecovery_ReturnsGenericMessage asserts the stream recovery
// interceptor does NOT expose the panic value to the caller.
func TestStreamRecovery_ReturnsGenericMessage(t *testing.T) {
	ss := &mockServerStream{ctx: context.Background()}
	err := StreamRecovery()(nil, ss, &grpc.StreamServerInfo{FullMethod: "/svc/Stream"},
		func(_ any, _ grpc.ServerStream) error {
			panic("secret-internal-xyz")
		})
	st, _ := status.FromError(err)
	if strings.Contains(st.Message(), "secret-internal-xyz") {
		t.Errorf("panic detail leaked to stream client: %q", st.Message())
	}
	const wantMsg = "internal error"
	if st.Message() != wantMsg {
		t.Errorf("message = %q, want %q", st.Message(), wantMsg)
	}
}

// TestRPCLog_RedactsAuthorizationHeader asserts that when payload logging is
// active, sensitive metadata keys (e.g. authorization) are replaced with
// [REDACTED] and never appear verbatim in log output.
func TestRPCLog_RedactsAuthorizationHeader(t *testing.T) {
	rec := &recordingLogger{enabled: true}
	md := metadata.New(map[string]string{
		"authorization": "Bearer secrettoken123",
		"x-request-id": "req-abc",
	})
	ctx := metadata.NewIncomingContext(
		core.ContextWithLogger(context.Background(), rec),
		md,
	)
	_, _ = RPCLog(Config{PayloadLogging: true, PayloadMaxBytes: 200})(
		ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/svc/M"},
		func(c context.Context, _ any) (any, error) { return "resp", nil })

	found := false
	for _, c := range rec.debugCalls {
		if c.msg != "rpc.start" {
			continue
		}
		found = true
		for i := 0; i+1 < len(c.args); i += 2 {
			k, ok := c.args[i].(string)
			if !ok || k != "metadata" {
				continue
			}
			val := fmt.Sprintf("%v", c.args[i+1])
			if strings.Contains(val, "secrettoken123") {
				t.Errorf("authorization token leaked verbatim in log: %v", c.args[i+1])
			}
			if !strings.Contains(val, "REDACTED") {
				t.Errorf("expected [REDACTED] in metadata log, got: %v", c.args[i+1])
			}
			// Non-sensitive key must not be redacted.
			if !strings.Contains(val, "req-abc") {
				t.Errorf("non-sensitive key x-request-id was unexpectedly redacted: %v", c.args[i+1])
			}
		}
	}
	if !found {
		t.Fatal("no rpc.start debug call found")
	}
}

func TestRequestID_EnrichesSpanAndLoggerWithCorrelationIDs(t *testing.T) {
	// Recording span so IsRecording() is true and SpanContext is valid.
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	ctx, span := tp.Tracer("test").Start(context.Background(), "op")

	// session.id baggage (pass-through source).
	mem, err := baggage.NewMember("session.id", "sess-123")
	if err != nil {
		t.Fatalf("NewMember: %v", err)
	}
	bag, err := baggage.New(mem)
	if err != nil {
		t.Fatalf("New baggage: %v", err)
	}
	ctx = baggage.ContextWithBaggage(ctx, bag)

	rec := &recordingLogger{}
	ctx = core.ContextWithLogger(ctx, rec)

	_, _ = RequestID()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(context.Context, any) (any, error) { return nil, nil })
	span.End()

	// Logger tags: traceId, spanId, sessionId present; sessionId value correct.
	if len(rec.withCalls) == 0 {
		t.Fatal("expected With to be called")
	}
	first := rec.withCalls[0]
	for _, k := range []string{"requestId", "method", "traceId", "spanId", "traceSampled", "sessionId"} {
		if !containsKey(first, k) {
			t.Errorf("With args missing %q: %v", k, first)
		}
	}
	if got := valueForKey(first, "sessionId"); got != "sess-123" {
		t.Errorf("sessionId = %v, want sess-123", got)
	}
	// The default SDK provider samples every span, so the real flag is true.
	if got := valueForKey(first, "traceSampled"); got != true {
		t.Errorf("traceSampled = %v, want true", got)
	}

	// Span attributes: request.id present, session.id == sess-123.
	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected 1 ended span, got %d", len(ended))
	}
	attrs := map[string]string{}
	for _, a := range ended[0].Attributes() {
		attrs[string(a.Key)] = a.Value.AsString()
	}
	if attrs["request.id"] == "" {
		t.Error("span missing request.id attribute")
	}
	if attrs["session.id"] != "sess-123" {
		t.Errorf("span session.id = %q, want sess-123", attrs["session.id"])
	}
}

// valueForKey returns the value following key in a flat k,v,... slice.
func valueForKey(kv []any, key string) any {
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i] == key {
			return kv[i+1]
		}
	}
	return nil
}

func TestRequestID_NoSessionBaggage_OmitsSessionFields(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	rec := &recordingLogger{}
	ctx = core.ContextWithLogger(ctx, rec)

	_, _ = RequestID()(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/svc/Method"},
		func(context.Context, any) (any, error) { return nil, nil })
	span.End()

	// codex #8: guard indices so a regression fails cleanly, not with a panic.
	if len(rec.withCalls) == 0 {
		t.Fatal("expected With to be called")
	}
	first := rec.withCalls[0]
	if containsKey(first, "sessionId") {
		t.Errorf("sessionId must be omitted when no session baggage: %v", first)
	}
	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected 1 ended span, got %d", len(ended))
	}
	for _, a := range ended[0].Attributes() {
		if string(a.Key) == "session.id" {
			t.Error("span must not have session.id when no baggage")
		}
	}
}

// helpers ---------------------------------------------------------------

type loggerCall struct {
	msg  string
	args []any
}

type recordingLogger struct {
	enabled    bool
	withCalls  [][]any
	infoCalls  []loggerCall
	debugCalls []loggerCall
}

func (r *recordingLogger) Debug(msg string, args ...any) {
	r.debugCalls = append(r.debugCalls, loggerCall{msg, args})
}
func (r *recordingLogger) Info(msg string, args ...any) {
	r.infoCalls = append(r.infoCalls, loggerCall{msg, args})
}
func (r *recordingLogger) Warn(string, ...any)  {}
func (r *recordingLogger) Error(string, ...any) {}
func (r *recordingLogger) With(args ...any) core.Logger {
	cp := make([]any, len(args))
	copy(cp, args)
	r.withCalls = append(r.withCalls, cp)
	// Return a distinct logger so callers can detect that With was applied.
	return &recordingLogger{enabled: r.enabled}
}
func (r *recordingLogger) Enabled(core.Level) bool { return r.enabled }

func containsKey(args []any, key string) bool {
	for i := 0; i+1 < len(args); i += 2 {
		if k, ok := args[i].(string); ok && k == key {
			return true
		}
	}
	return false
}

var _ = errors.Is // silence unused import in some builds
