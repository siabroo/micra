package core

import (
	"context"
	"testing"
)

// fakeComponent is a Component implementation used by core tests. The
// channels let tests assert on call ordering and inject errors.
type fakeComponent struct {
	name       string
	initErr    error
	startErr   error
	stopErr    error
	startBlock chan struct{} // closed by test to unblock Start
	initCalled bool
	startCalled bool
	stopCalled  bool
}

func (f *fakeComponent) Name() string { return f.name }

func (f *fakeComponent) Init(ctx context.Context) error {
	f.initCalled = true
	return f.initErr
}

func (f *fakeComponent) Start(ctx context.Context) error {
	f.startCalled = true
	if f.startErr != nil {
		return f.startErr
	}
	if f.startBlock != nil {
		select {
		case <-f.startBlock:
		case <-ctx.Done():
			return nil
		}
	} else {
		<-ctx.Done()
	}
	return nil
}

func (f *fakeComponent) Stop(ctx context.Context) error {
	f.stopCalled = true
	return f.stopErr
}

// Compile-time interface assertions — if interfaces change shape, this
// file fails to compile.
var _ Component = (*fakeComponent)(nil)
var _ Initializer = (*fakeComponent)(nil)

func TestComponent_InterfaceShape(t *testing.T) {
	c := &fakeComponent{name: "test"}
	if c.Name() != "test" {
		t.Errorf("Name() = %q, want %q", c.Name(), "test")
	}
}
