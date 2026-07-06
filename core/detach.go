package core

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

// DetachTimeout is the deadline applied to fn inside Detach. Detach is
// intended for short fire-and-forget side effects (audit records,
// analytics events, metrics emission) — anything that needs longer
// belongs in a different mechanism.
const DetachTimeout = 10 * time.Second

// Detach runs fn in a new goroutine with a context derived from
// parent that:
//
//   - Inherits every value from parent (logger with requestId/method,
//     trace/span context, outgoing gRPC metadata).
//   - Does NOT propagate parent's cancellation: the goroutine survives
//     the return of the handler that scheduled it.
//   - Has a 10-second deadline (DetachTimeout) so a hung downstream
//     does not leak the goroutine past process shutdown.
//
// The logger derived from the new ctx is tagged with "detached"=true,
// so log lines emitted from fn are filterable apart from request-scoped
// ones in log aggregators. When AddSource is enabled on the underlying
// logger, each log line additionally carries the file:line where the
// log call was made — there is no need to name the detached task.
//
// Panics inside fn are recovered and logged as Error via the parent
// logger.
//
// Detach is for fire-and-forget. For long-lived background work or
// queued jobs, use a different primitive.
func Detach(parent context.Context, fn func(context.Context)) {
	parentLog := LoggerFrom(parent)
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), DetachTimeout)
		defer cancel()

		log := parentLog.With("detached", true)
		ctx = ContextWithLogger(ctx, log)

		defer func() {
			if r := recover(); r != nil {
				log.Error("detached task panicked",
					"panic", fmt.Sprint(r),
					"stack", string(debug.Stack()))
			}
		}()
		fn(ctx)
	}()
}
