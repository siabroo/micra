package loggerslog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/siabroo/micra/core"
)

func TestNew_ImplementsCoreLogger(t *testing.T) {
	var _ core.Logger = New(slog.Default())
}

func TestLogger_LevelMethodsRouteToSlog(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	l := New(slog.New(h))

	l.Debug("dbg", "k", 1)
	l.Info("inf", "k", 2)
	l.Warn("wrn", "k", 3)
	l.Error("err", "k", 4)

	lines := splitNonEmpty(buf.String())
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4: %q", len(lines), buf.String())
	}
	for i, want := range []struct {
		level string
		msg   string
	}{
		{"DEBUG", "dbg"}, {"INFO", "inf"}, {"WARN", "wrn"}, {"ERROR", "err"},
	} {
		var got map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &got); err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if got["level"] != want.level {
			t.Errorf("line %d: level = %v, want %v", i, got["level"], want.level)
		}
		if got["msg"] != want.msg {
			t.Errorf("line %d: msg = %v, want %v", i, got["msg"], want.msg)
		}
	}
}

func TestLogger_WithAttachesAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, nil)
	base := New(slog.New(h))
	child := base.With("service", "x")
	child.Info("hello")

	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &got); err != nil {
		t.Fatalf("json: %v (%q)", err, buf.String())
	}
	if got["service"] != "x" {
		t.Errorf("service = %v, want x", got["service"])
	}
}

func TestLogger_Enabled(t *testing.T) {
	h := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	l := New(slog.New(h))
	if l.Enabled(core.LevelInfo) {
		t.Error("Enabled(Info) = true, want false at WARN threshold")
	}
	if !l.Enabled(core.LevelWarn) {
		t.Error("Enabled(Warn) = false, want true at WARN threshold")
	}
	if !l.Enabled(core.LevelError) {
		t.Error("Enabled(Error) = false, want true at WARN threshold")
	}
}

func splitNonEmpty(s string) []string {
	parts := strings.Split(s, "\n")
	out := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}
