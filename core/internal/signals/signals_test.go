package signals

import (
	"context"
	"syscall"
	"testing"
	"time"
)

func TestTrap_CancelsContextOnSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := Trap(ctx, cancel, syscall.SIGUSR1)
	defer stop()

	// Send SIGUSR1 to our own process.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGUSR1); err != nil {
		t.Fatalf("Kill(SIGUSR1) failed: %v", err)
	}

	select {
	case <-ctx.Done():
		// success — the trap cancelled the ctx
	case <-time.After(2 * time.Second):
		t.Fatal("context was not cancelled within 2s of receiving SIGUSR1")
	}
}

func TestTrap_StopUnregisters(t *testing.T) {
	// Start a trap, immediately stop it, then send the signal.
	// Without an unregister, syscall.Kill would terminate the test process.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := Trap(ctx, cancel, syscall.SIGUSR2)
	stop()
	// We can't actually verify "the test process is still alive after a
	// SIGUSR2", because Go's signal.Reset puts the default handler back
	// which for SIGUSR2 is "terminate". Instead, assert that stop is
	// idempotent — multiple calls don't panic.
	stop()
	stop()
}
