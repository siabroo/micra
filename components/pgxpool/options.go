package pgxpool

import "time"

type config struct {
	name            string
	dsn             string
	maxConns        int32
	minConns        int32
	maxConnLifetime time.Duration
	maxConnIdleTime time.Duration
	connectTimeout  time.Duration
	pingTimeout     time.Duration
}

// Option configures Pool via New.
type Option func(*config)

func defaults() config {
	return config{
		name:           "pgxpool",
		connectTimeout: 10 * time.Second,
		pingTimeout:    5 * time.Second,
	}
}

// WithName overrides the default component name "pgxpool".
func WithName(name string) Option { return func(c *config) { c.name = name } }

// WithDSN is required. Sets the connection string (libpq URL form).
func WithDSN(dsn string) Option { return func(c *config) { c.dsn = dsn } }

// WithMaxConns sets pgxpool MaxConns.
func WithMaxConns(n int32) Option { return func(c *config) { c.maxConns = n } }

// WithMinConns sets pgxpool MinConns.
func WithMinConns(n int32) Option { return func(c *config) { c.minConns = n } }

// WithMaxConnLifetime sets the per-conn lifetime.
func WithMaxConnLifetime(d time.Duration) Option {
	return func(c *config) { c.maxConnLifetime = d }
}

// WithMaxConnIdleTime sets per-conn idle timeout.
func WithMaxConnIdleTime(d time.Duration) Option {
	return func(c *config) { c.maxConnIdleTime = d }
}

// WithConnectTimeout bounds the pgxpool.New call in Init.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *config) { c.connectTimeout = d }
}

// WithPingTimeout bounds the pool.Ping call in Init.
func WithPingTimeout(d time.Duration) Option {
	return func(c *config) { c.pingTimeout = d }
}
