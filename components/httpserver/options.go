package httpserver

import (
	"net/http"
	"time"
)

type config struct {
	name          string
	addr          string
	handler       http.Handler
	readTimeout   time.Duration
	writeTimeout  time.Duration
	idleTimeout   time.Duration
	shutdownGrace time.Duration
}

// Option configures Server via New.
type Option func(*config)

func defaults() config {
	return config{
		name:          "http",
		readTimeout:   10 * time.Second,
		writeTimeout:  10 * time.Second,
		idleTimeout:   60 * time.Second,
		shutdownGrace: 10 * time.Second,
	}
}

// WithName overrides the default component name "http".
func WithName(name string) Option { return func(c *config) { c.name = name } }

// WithAddr is required. The listen address (host:port).
func WithAddr(addr string) Option { return func(c *config) { c.addr = addr } }

// WithHandler is required. The HTTP handler.
func WithHandler(h http.Handler) Option { return func(c *config) { c.handler = h } }

// WithReadTimeout overrides default 10s.
func WithReadTimeout(d time.Duration) Option { return func(c *config) { c.readTimeout = d } }

// WithWriteTimeout overrides default 10s.
func WithWriteTimeout(d time.Duration) Option { return func(c *config) { c.writeTimeout = d } }

// WithIdleTimeout overrides default 60s.
func WithIdleTimeout(d time.Duration) Option { return func(c *config) { c.idleTimeout = d } }

// WithShutdownGrace overrides the default 10s deadline used by Stop
// when the user-supplied stopCtx allows more.
func WithShutdownGrace(d time.Duration) Option { return func(c *config) { c.shutdownGrace = d } }
