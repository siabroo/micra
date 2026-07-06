package core

import "context"

// Component is a long-running unit of lifecycle. See the design spec
// §6.1 for the full contract.
//
// State model:
//
//	registered ──Init──► initialized ──Start──► running ──Stop──► stopped
//	             │                                │
//	             └───── (skip if no Init) ────────┘
//
// Init errors abort registration order; Start errors trigger fail-fast.
// Stop is called for every Component that entered "running" (and only
// those).
type Component interface {
	// Name returns a unique identifier within an App. Used for:
	//   - the "component" log tag attached to ctx in Start
	//   - error messages: "component %q failed: %w"
	//   - duplicate detection on Register.
	// Conventions: lowercase, hyphen-separated, descriptive ("grpc",
	// "pgxpool", "worker-emails", "metrics-http").
	Name() string

	// Start runs the component until ctx is cancelled or the component
	// fails. It blocks. Returns:
	//   - nil when ctx was cancelled (normal shutdown signal from App)
	//   - non-nil error if the component failed of its own accord
	//     (irrecoverable internal error, recovered panic)
	//
	// App calls Start exactly once. By the time Start is invoked, every
	// earlier-registered Initializer has already completed successfully.
	Start(ctx context.Context) error

	// Stop releases resources held by Start. App calls Stop sequentially
	// in reverse-of-Register order. Implementations must wait for
	// Start's goroutine to actually exit (closed listener, drained
	// pool) before returning.
	//
	// The ctx passed to Stop has its own deadline (App.WithStopTimeout,
	// default 30s). Implementations must respect it.
	//
	// Stop is invoked only for Components that entered "running" state.
	Stop(ctx context.Context) error
}

// Initializer is an optional interface for Components that need a
// synchronous setup phase before Start blocks. App calls Init in
// registration order, synchronously: an error from Init aborts startup
// (fail-fast) without launching any further Component's Start.
//
// Init runs to completion before any later Component's Init begins, and
// before any Start goroutine is launched. This guarantees that, by the
// time grpcserver.Start runs, pgxpool.Init has already returned — and
// the gRPC handler can safely call pool.DB().
//
// Components that have no synchronous setup phase do not implement
// Initializer at all; App treats them as if Init returned nil
// immediately.
type Initializer interface {
	Init(ctx context.Context) error
}
