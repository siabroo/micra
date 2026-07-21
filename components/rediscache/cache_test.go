package rediscache

import (
	"testing"
	"time"
)

func TestCacheDefaults(t *testing.T) {
	c := NewCache(nil)
	if c.cfg.prefix != "" {
		t.Errorf("prefix = %q, want empty", c.cfg.prefix)
	}
	if c.cfg.defaultTTL != 0 {
		t.Errorf("defaultTTL = %v, want 0", c.cfg.defaultTTL)
	}
}

func TestCacheOptions(t *testing.T) {
	c := NewCache(nil,
		WithPrefix("app"),
		WithDefaultTTL(30*time.Second),
	)
	if c.cfg.prefix != "app" {
		t.Errorf("prefix = %q, want %q", c.cfg.prefix, "app")
	}
	if c.cfg.defaultTTL != 30*time.Second {
		t.Errorf("defaultTTL = %v, want 30s", c.cfg.defaultTTL)
	}
}

func TestNamespacedKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{"empty prefix", "", "user:1", "user:1"},
		{"with prefix", "app", "user:1", "app:user:1"},
		{"prefix trailing colon ignored", "app", "k", "app:k"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCache(nil, WithPrefix(tt.prefix))
			if got := c.namespaced(tt.key); got != tt.want {
				t.Errorf("namespaced(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestNamespacedRoundTrip(t *testing.T) {
	c := NewCache(nil, WithPrefix("ns"))
	key := "some:key"
	full := c.namespaced(key)
	if full != "ns:"+key {
		t.Fatalf("namespaced = %q, want %q", full, "ns:"+key)
	}
}

func TestEffectiveTTL(t *testing.T) {
	c := NewCache(nil, WithDefaultTTL(10*time.Second))
	if got := c.effectiveTTL(0); got != 10*time.Second {
		t.Errorf("effectiveTTL(0) = %v, want default 10s", got)
	}
	if got := c.effectiveTTL(2 * time.Second); got != 2*time.Second {
		t.Errorf("effectiveTTL(2s) = %v, want 2s", got)
	}
}
