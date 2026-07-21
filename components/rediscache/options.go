package rediscache

import "time"

type config struct {
	name        string
	addr        string
	password    string
	db          int
	dsn         string
	dialTimeout time.Duration
	pingTimeout time.Duration
}

// Option configures Client via New.
type Option func(*config)

func defaults() config {
	return config{
		name:        "rediscache",
		addr:        "localhost:6379",
		dialTimeout: 10 * time.Second,
		pingTimeout: 5 * time.Second,
	}
}

// WithName overrides the default component name "rediscache".
func WithName(name string) Option { return func(c *config) { c.name = name } }

// WithAddr sets the Redis host:port address. Defaults to "localhost:6379".
// Ignored when WithDSN is provided.
func WithAddr(addr string) Option { return func(c *config) { c.addr = addr } }

// WithPassword sets the Redis auth password. Ignored when WithDSN is provided.
func WithPassword(password string) Option { return func(c *config) { c.password = password } }

// WithDB selects the Redis logical database index. Ignored when WithDSN is
// provided.
func WithDB(db int) Option { return func(c *config) { c.db = db } }

// WithDSN sets a Redis connection URL (e.g. "redis://user:pass@host:6379/0").
// When set, it takes precedence over WithAddr/WithPassword/WithDB. Note that the
// dial timeout is always taken from WithDialTimeout (default 10s), so any
// dial_timeout encoded in the DSN query string is ignored.
func WithDSN(dsn string) Option { return func(c *config) { c.dsn = dsn } }

// WithDialTimeout bounds the initial connection dial in Init. It always applies,
// overriding any dial_timeout encoded in a WithDSN URL.
func WithDialTimeout(d time.Duration) Option { return func(c *config) { c.dialTimeout = d } }

// WithPingTimeout bounds the PING call in Init.
func WithPingTimeout(d time.Duration) Option { return func(c *config) { c.pingTimeout = d } }
