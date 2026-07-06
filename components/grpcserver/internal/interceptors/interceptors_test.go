package interceptors

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

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
