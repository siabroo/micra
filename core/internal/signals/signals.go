// Package signals is internal helper for the SIGINT/SIGTERM trap that
// core.App installs in Run and RunOnce. Extracted here so it can be
// tested independently of App lifecycle logic.
package signals

import (
	"context"
	"os"
	"os/signal"
	"sync"
)

// Trap registers a handler that calls cancel the first time any of the
// given signals is received on the process. Returns a stop function
// that unregisters the handler. The stop function is safe to call
// multiple times.
//
// If sigs is empty, Trap is a no-op and stop is a no-op.
func Trap(ctx context.Context, cancel context.CancelFunc, sigs ...os.Signal) (stop func()) {
	if len(sigs) == 0 {
		return func() {}
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)

	done := make(chan struct{})
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
		close(done)
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			signal.Stop(ch)
			// Wait for the goroutine to exit so callers don't race with
			// it on subsequent test runs.
			cancel()
			<-done
		})
	}
}
