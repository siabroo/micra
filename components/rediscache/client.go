// Package rediscache exposes a *github.com/redis/go-redis/v9.Client as a
// micra core.Component + core.Initializer, plus an L2 distributed Cache and
// pub/sub key-invalidation helpers.
//
// The package provides an L2 (shared, Redis-backed) cache only — there is no
// in-memory L1 tier. Multiple service instances synchronise their local state
// by broadcasting key-invalidation events over a pub/sub channel (see
// Publisher and Subscriber).
//
// Init dials Redis and issues a PING (fail-fast if either fails). Redis() is
// safe to call from any later-registered Component's Start once Init has
// returned.
package rediscache

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/siabroo/micra/core"
)

// Client wraps a *redis.Client with micra lifecycle.
type Client struct {
	cfg config
	rdb *redis.Client
}

// New constructs a Client. Either WithAddr (default "localhost:6379") or
// WithDSN must resolve to a non-empty target; an empty target returns an error
// from Init.
func New(opts ...Option) *Client {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}
	return &Client{cfg: cfg}
}

// Name implements core.Component.
func (c *Client) Name() string { return c.cfg.name }

// Init implements core.Initializer: open the client and PING it.
func (c *Client) Init(ctx context.Context) error {
	var opt *redis.Options
	if c.cfg.dsnSet {
		if c.cfg.dsn == "" {
			return errors.New("rediscache: DSN is empty (WithDSN given an empty string)")
		}
		parsed, err := redis.ParseURL(c.cfg.dsn)
		if err != nil {
			return fmt.Errorf("parse DSN: %w", err)
		}
		opt = parsed
	} else {
		if c.cfg.addr == "" {
			return errors.New("rediscache: addr is empty (use WithAddr or WithDSN)")
		}
		opt = &redis.Options{
			Addr:     c.cfg.addr,
			Password: c.cfg.password,
			DB:       c.cfg.db,
		}
	}
	if c.cfg.dialTimeout > 0 {
		opt.DialTimeout = c.cfg.dialTimeout
	}

	rdb := redis.NewClient(opt)

	// A non-positive ping timeout would make context.WithTimeout produce an
	// already-expired context, failing every Init; fall back to the default.
	pingTimeout := c.cfg.pingTimeout
	if pingTimeout <= 0 {
		pingTimeout = defaultPingTimeout
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return fmt.Errorf("ping: %w", err)
	}

	c.rdb = rdb
	core.LoggerFrom(ctx).Info("redis client ready",
		"name", c.cfg.name,
		"addr", opt.Addr,
		"db", opt.DB,
	)
	return nil
}

// Start implements core.Component: block until ctx cancelled.
func (c *Client) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Stop implements core.Component: close the client, bounded by ctx.
func (c *Client) Stop(ctx context.Context) error {
	if c.rdb == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- c.rdb.Close() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		core.LoggerFrom(ctx).Warn("rediscache stop exceeded deadline",
			"name", c.cfg.name)
		return ctx.Err()
	}
}

// Redis returns the underlying client. Safe after Init returns nil. Panics if
// called before Init.
func (c *Client) Redis() *redis.Client {
	if c.rdb == nil {
		panic("rediscache: Redis() called before Init")
	}
	return c.rdb
}
