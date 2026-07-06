package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestRun_TriggerErrRace reproduces the data race on triggerErr between the
// drain goroutine (which writes triggerErr when a Start returns an error) and
// the main goroutine (which reads/writes triggerErr right after unblocking from
// internalCtx.Done()). When shutdown is triggered by parent-context
// cancellation while a component's Start simultaneously returns an error, both
// goroutines touch triggerErr with no happens-before relationship.
//
// Under `go test -race` this fails (race detected) before the fix.
func TestRun_TriggerErrRace(t *testing.T) {
	boom := errors.New("boom on cancel")
	for i := 0; i < 200; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		// Component that returns an error the instant ctx is cancelled.
		start := func(ctx context.Context) error {
			<-ctx.Done()
			return boom
		}
		stop := func(ctx context.Context) error { return nil }

		go cancel() // parent cancel races with the Start error result

		_ = Run(ctx, []Item{
			{Name: "racer", Start: start, Stop: stop},
		}, 1*time.Second)
	}
}

// TestRun_SendOnClosedChannel reproduces the send-on-closed-channel crash.
// A component's Start goroutine ignores context cancellation and stays blocked
// past stopTimeout, so Phase 5 times out and closes startResults. When the
// Start goroutine is later released and performs its channel send, the old code
// panics with "send on closed channel" in a goroutine with no recover, killing
// the process.
//
// After the fix, Run returns cleanly and releasing the goroutine causes no panic.
func TestRun_SendOnClosedChannel(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})

	start := func(ctx context.Context) error {
		close(started)
		<-release // ignores ctx cancellation entirely
		return nil
	}
	stop := func(ctx context.Context) error { return nil }

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	err := Run(ctx, []Item{
		{Name: "stuck", Start: start, Stop: stop},
	}, 50*time.Millisecond)

	if err == nil || !errors.Is(err, ErrStopTimeout) {
		t.Fatalf("Run err = %v, want chain including ErrStopTimeout", err)
	}

	// Release the stuck Start goroutine; it will now perform its channel send.
	// Before the fix this is a send on a closed channel -> panic -> crash.
	close(release)
	// Give the goroutine time to attempt the send.
	time.Sleep(100 * time.Millisecond)
}
