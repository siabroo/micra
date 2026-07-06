package loggerslog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/siabroo/micra/core"
)

func TestNewSimple_DefaultsToJSONAndInfo(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimple(Config{Output: &buf})
	l.Debug("dbg")
	l.Info("inf")

	out := buf.String()
	if strings.Contains(out, "dbg") {
		t.Errorf("debug should be filtered at default LevelInfo: %s", out)
	}
	if !strings.Contains(out, "inf") {
		t.Errorf("info should be emitted: %s", out)
	}
	// First non-empty line must parse as JSON.
	line := firstLine(out)
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("default output is not JSON: %v (%q)", err, line)
	}
	if got["msg"] != "inf" {
		t.Errorf("msg = %v, want inf", got["msg"])
	}
}

func TestNewSimple_FormatText(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimple(Config{Output: &buf, Format: FormatText})
	l.Info("hello", "k", "v")

	out := buf.String()
	if !strings.Contains(out, "msg=hello") {
		t.Errorf("expected key=value text format, got %q", out)
	}
	if !strings.Contains(out, "k=v") {
		t.Errorf("expected attr key=v in text output, got %q", out)
	}
	// Text handler must not emit ANSI escape codes — micra deliberately
	// does not provide a coloured handler.
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("text output contains ANSI escape: %q", out)
	}
}

func TestNewSimple_SourceAddsCallerInfo(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimple(Config{Output: &buf, Source: true})
	l.Info("with source")

	line := firstLine(buf.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, line)
	}
	src, ok := got["source"].(map[string]any)
	if !ok {
		t.Fatalf("expected source object in log, got %v", got)
	}
	file, _ := src["file"].(string)
	if file == "" {
		t.Errorf("source missing file: %v", src)
	}
	if _, ok := src["line"]; !ok {
		t.Errorf("source missing line: %v", src)
	}
	// Source must point at the test file (the caller), not at the
	// adapter's logger.go — otherwise AddSource is useless to consumers.
	if strings.HasSuffix(file, "logger.go") {
		t.Errorf("source points at the wrapper (logger.go) instead of caller: %v", src)
	}
	if !strings.HasSuffix(file, "easy_test.go") {
		t.Errorf("source should point at this test file, got file=%q", file)
	}
}

func TestNewSimple_SourceOffByDefault(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimple(Config{Output: &buf})
	l.Info("no source")

	line := firstLine(buf.String())
	var got map[string]any
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if _, present := got["source"]; present {
		t.Errorf("source should be absent when Source=false, got %v", got)
	}
}

func TestNewSimple_FormatPrettyAlias(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimple(Config{Output: &buf, Format: "pretty"})
	l.Info("hi")
	if !strings.Contains(buf.String(), "msg=hi") {
		t.Errorf("pretty alias should produce text output: %q", buf.String())
	}
}

func TestNewSimple_LevelFilters(t *testing.T) {
	var buf bytes.Buffer
	l := NewSimple(Config{Output: &buf, Level: core.LevelWarn})
	l.Info("i")
	l.Warn("w")
	if strings.Contains(buf.String(), "\"i\"") || strings.Contains(buf.String(), "msg\":\"i\"") {
		t.Errorf("info should be filtered at warn threshold: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "\"w\"") {
		t.Errorf("warn should be emitted: %q", buf.String())
	}
}

func firstLine(s string) string {
	for _, p := range strings.Split(s, "\n") {
		if strings.TrimSpace(p) != "" {
			return p
		}
	}
	return ""
}
