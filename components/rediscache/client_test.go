package rediscache

import (
	"context"
	"testing"
	"time"

	"github.com/siabroo/micra/core"
)

func TestNew_Defaults(t *testing.T) {
	c := New()
	if c.cfg.name != "rediscache" {
		t.Errorf("name = %q, want %q", c.cfg.name, "rediscache")
	}
	if c.cfg.addr != "localhost:6379" {
		t.Errorf("addr = %q, want %q", c.cfg.addr, "localhost:6379")
	}
	if c.cfg.dialTimeout != 10*time.Second {
		t.Errorf("dialTimeout = %v, want 10s", c.cfg.dialTimeout)
	}
	if c.cfg.pingTimeout != 5*time.Second {
		t.Errorf("pingTimeout = %v, want 5s", c.cfg.pingTimeout)
	}
}

func TestOptions_Override(t *testing.T) {
	c := New(
		WithName("cache"),
		WithAddr("redis:6380"),
		WithPassword("secret"),
		WithDB(3),
		WithDialTimeout(2*time.Second),
		WithPingTimeout(1*time.Second),
	)
	if c.cfg.name != "cache" {
		t.Errorf("name = %q, want %q", c.cfg.name, "cache")
	}
	if c.cfg.addr != "redis:6380" {
		t.Errorf("addr = %q, want %q", c.cfg.addr, "redis:6380")
	}
	if c.cfg.password != "secret" {
		t.Errorf("password = %q, want %q", c.cfg.password, "secret")
	}
	if c.cfg.db != 3 {
		t.Errorf("db = %d, want 3", c.cfg.db)
	}
	if c.cfg.dialTimeout != 2*time.Second {
		t.Errorf("dialTimeout = %v, want 2s", c.cfg.dialTimeout)
	}
	if c.cfg.pingTimeout != 1*time.Second {
		t.Errorf("pingTimeout = %v, want 1s", c.cfg.pingTimeout)
	}
}

func TestWithDSN(t *testing.T) {
	c := New(WithDSN("redis://localhost:6379/1"))
	if c.cfg.dsn != "redis://localhost:6379/1" {
		t.Errorf("dsn = %q, want %q", c.cfg.dsn, "redis://localhost:6379/1")
	}
}

func TestInit_EmptyAddr_ReturnsError(t *testing.T) {
	c := New(WithAddr(""))
	if err := c.Init(context.Background()); err == nil {
		t.Fatal("Init succeeded with empty addr, want error")
	}
}

func TestInit_BadDSN_ReturnsError(t *testing.T) {
	c := New(WithDSN("not-a-valid-url"))
	if err := c.Init(context.Background()); err == nil {
		t.Fatal("Init succeeded with bad DSN, want error")
	}
}

func TestRedis_PanicsBeforeInit(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Redis() did not panic before Init")
		}
	}()
	New().Redis()
}

func TestInterfaceAssertions(t *testing.T) {
	c := New()
	if _, ok := any(c).(core.Component); !ok {
		t.Fatal("Client does not implement core.Component")
	}
	if _, ok := any(c).(core.Initializer); !ok {
		t.Fatal("Client does not implement core.Initializer")
	}
}
