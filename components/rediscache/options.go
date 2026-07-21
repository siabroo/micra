package rediscache

import "time"

type config struct {
	name        string
	addr        string
	password    string
	db          int
	dsn         string
	dsnSet      bool
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
// When set, it takes precedence over WithAddr/WithPassword/WithDB. Passing an
// empty string is an error surfaced by Init (it does not silently fall back to
// WithAddr), so WithDSN(os.Getenv("REDIS_URL")) fails fast when the variable is
// unset instead of connecting to the default address. When a positive
// WithDialTimeout is set (the default is 10s), it overrides any dial_timeout
// encoded in the DSN query string.
func WithDSN(dsn string) Option { return func(c *config) { c.dsn = dsn; c.dsnSet = true } }

// WithDialTimeout bounds the initial connection dial in Init. A positive value
// overrides any dial_timeout encoded in a WithDSN URL; a zero value leaves the
// DSN value (or the go-redis default) in place.
func WithDialTimeout(d time.Duration) Option { return func(c *config) { c.dialTimeout = d } }

// WithPingTimeout bounds the PING call in Init.
func WithPingTimeout(d time.Duration) Option { return func(c *config) { c.pingTimeout = d } }
