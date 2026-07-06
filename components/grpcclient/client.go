// Package grpcclient exposes a *grpc.ClientConn as a micra
// core.Component + core.Initializer. Mirrors components/grpcserver:
// Init builds the connection (lazy under the hood — grpc.NewClient
// does no I/O until first RPC), Stop closes it. The component has no
// OTel dependency; services that want OTel pass
// otelgrpc.UnaryClientInterceptor() via WithUnaryInterceptors.
package grpcclient

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"

	"github.com/siabroo/micra/core"
)

// Client wraps a *grpc.ClientConn with micra lifecycle.
type Client struct {
	cfg  config
	conn *grpc.ClientConn
}

// New constructs a Client. WithTarget is required.
func New(opts ...Option) *Client {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}
	return &Client{cfg: cfg}
}

// Name implements core.Component.
func (c *Client) Name() string { return c.cfg.name }

// Init implements core.Initializer: build the *grpc.ClientConn.
// grpc.NewClient is non-blocking; connection errors surface from the
// first RPC, not here.
func (c *Client) Init(ctx context.Context) error {
	if c.cfg.target == "" {
		return errors.New("grpcclient: WithTarget is required")
	}
	dialOpts := append([]grpc.DialOption{}, c.cfg.dialOpts...)
	if len(c.cfg.unaryICs) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(c.cfg.unaryICs...))
	}
	if len(c.cfg.streamICs) > 0 {
		dialOpts = append(dialOpts, grpc.WithChainStreamInterceptor(c.cfg.streamICs...))
	}
	conn, err := grpc.NewClient(c.cfg.target, dialOpts...)
	if err != nil {
		return fmt.Errorf("grpcclient: NewClient %q: %w", c.cfg.target, err)
	}
	c.conn = conn
	core.LoggerFrom(ctx).Info("grpc client ready",
		"name", c.cfg.name,
		"target", c.cfg.target,
	)
	return nil
}

// Start implements core.Component: block until ctx cancelled. The
// connection itself manages its own goroutines.
func (c *Client) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Stop implements core.Component: close the connection.
func (c *Client) Stop(ctx context.Context) error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Conn returns the underlying connection. Panics if called before Init.
func (c *Client) Conn() *grpc.ClientConn {
	if c.conn == nil {
		panic("grpcclient: Conn() called before Init")
	}
	return c.conn
}
