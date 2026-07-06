package core

import "testing"

func TestParseLevel_KnownNames(t *testing.T) {
	cases := []struct {
		in   string
		want Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{" Info ", LevelInfo},
		{"", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
	}
	for _, c := range cases {
		got, err := ParseLevel(c.in)
		if err != nil {
			t.Errorf("ParseLevel(%q) returned err %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ParseLevel(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseLevel_Unknown_ReturnsErrorAndInfo(t *testing.T) {
	got, err := ParseLevel("trace")
	if err == nil {
		t.Fatal("expected error for unknown level")
	}
	if got != LevelInfo {
		t.Errorf("got %d on unknown level, want LevelInfo (%d) as fallback", got, LevelInfo)
	}
}
