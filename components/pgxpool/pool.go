// Package pgxpool exposes a *github.com/jackc/pgx/v5/pgxpool.Pool as
// a micra core.Component + core.Initializer. Init does pgxpool.New
// plus Ping (fail-fast if either fails). DB() is safe to call from
// any later-registered Component's Start once Init has returned.
package pgxpool

import (
	"context"
	"errors"
	"fmt"

	pgxpoolv5 "github.com/jackc/pgx/v5/pgxpool"

	"github.com/siabroo/micra/core"
)

// Pool wraps a *pgxpoolv5.Pool with micra lifecycle.
type Pool struct {
	cfg  config
	pool *pgxpoolv5.Pool
}

// New constructs a Pool. WithDSN is required; calling New + Init with
// an empty DSN returns an error from Init.
func New(opts ...Option) *Pool {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}
	return &Pool{cfg: cfg}
}

// Name implements core.Component.
func (p *Pool) Name() string { return p.cfg.name }

// Init implements core.Initializer: open the pool and Ping it.
func (p *Pool) Init(ctx context.Context) error {
	if p.cfg.dsn == "" {
		return errors.New("pgxpool: DSN is empty (use WithDSN)")
	}
	pcfg, err := pgxpoolv5.ParseConfig(p.cfg.dsn)
	if err != nil {
		return fmt.Errorf("parse DSN: %w", err)
	}
	if p.cfg.maxConns > 0 {
		pcfg.MaxConns = p.cfg.maxConns
	}
	if p.cfg.minConns > 0 {
		pcfg.MinConns = p.cfg.minConns
	}
	if p.cfg.maxConnLifetime > 0 {
		pcfg.MaxConnLifetime = p.cfg.maxConnLifetime
	}
	if p.cfg.maxConnIdleTime > 0 {
		pcfg.MaxConnIdleTime = p.cfg.maxConnIdleTime
	}

	connectCtx, cancel := context.WithTimeout(ctx, p.cfg.connectTimeout)
	defer cancel()
	pool, err := pgxpoolv5.NewWithConfig(connectCtx, pcfg)
	if err != nil {
		return fmt.Errorf("new pool: %w", err)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, p.cfg.pingTimeout)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return fmt.Errorf("ping: %w", err)
	}

	p.pool = pool
	core.LoggerFrom(ctx).Info("db pool ready",
		"name", p.cfg.name,
		"max_conns", pcfg.MaxConns,
	)
	return nil
}

// Start implements core.Component: block until ctx cancelled.
func (p *Pool) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Stop implements core.Component: close the pool, bounded by ctx.
func (p *Pool) Stop(ctx context.Context) error {
	if p.pool == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		p.pool.Close()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		core.LoggerFrom(ctx).Warn("pgxpool stop exceeded deadline; in-flight queries may be lost",
			"name", p.cfg.name)
		return ctx.Err()
	}
}

// DB returns the underlying pool. Safe after Init returns nil. Panics
// if called before Init.
func (p *Pool) DB() *pgxpoolv5.Pool {
	if p.pool == nil {
		panic("pgxpool: DB() called before Init")
	}
	return p.pool
}
