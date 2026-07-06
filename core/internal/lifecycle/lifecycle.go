// Package lifecycle drives a list of components through the
// Init → Start → Stop sequence described in the spec §9.1.
//
// It is intentionally decoupled from the public core.Component
// interface: it takes Items containing function pointers for Init
// (optional), Start, and Stop. The App layer in core adapts
// Component values into Items. This decoupling means lifecycle is
// trivially testable without the App machinery.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// ErrStopTimeout signals that one or more Stop calls did not return
// before their deadline. core.ErrStopTimeout is an alias for this.
var ErrStopTimeout = errors.New("stop timeout exceeded")

// Item is the unit lifecycle.Run consumes: per-component function
// pointers. App constructs the Init field as nil for Components that
// don't implement core.Initializer.
type Item struct {
	Name  string
	Init  func(ctx context.Context) error // nil if no Init phase
	Start func(ctx context.Context) error
	Stop  func(ctx context.Context) error
}

// Run drives items through Init, Start, and Stop phases as described
// in spec §9.1.
//
// On return, all Start goroutines for components that entered "running"
// state have either returned or been bounded by stopTimeout.
func Run(ctx context.Context, items []Item, stopTimeout time.Duration) error {
	// ---------- Phase 1: Init (sequential, registration order) ----------
	for i, item := range items {
		if item.Init == nil {
			continue
		}
		if err := safeCall(func() error { return item.Init(ctx) }); err != nil {
			return fmt.Errorf("component %q init: %w", items[i].Name, err)
		}
	}

	// ---------- Phase 2: Start (concurrent) ----------
	internalCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type startResult struct {
		idx int
		err error
	}
	startResults := make(chan startResult, len(items))

	// Track which components actually entered Start so we know which to Stop.
	running := make([]bool, len(items))
	var runningMu sync.Mutex

	var startWG sync.WaitGroup
	for i := range items {
		i := i
		startWG.Add(1)
		runningMu.Lock()
		running[i] = true
		runningMu.Unlock()
		go func() {
			defer startWG.Done()
			err := safeCall(func() error { return items[i].Start(internalCtx) })
			startResults <- startResult{idx: i, err: err}
		}()
	}

	// ---------- Phase 3: Wait for shutdown trigger ----------
	var triggerErr error
	go func() {
		// Drain the first Start error from a running component;
		// if Start returned nil after ctx cancel, ignore — it's expected.
		for res := range startResults {
			if res.err != nil && triggerErr == nil {
				triggerErr = fmt.Errorf("component %q start: %w", items[res.idx].Name, res.err)
				cancel()
			}
		}
	}()

	<-internalCtx.Done()
	// internalCtx is now cancelled — either because parent ctx was
	// cancelled, or because a Start errored and the goroutine above
	// cancelled it.

	// If internalCtx was cancelled by parent (not by a Start error),
	// capture parent's err.
	if triggerErr == nil {
		if pe := ctx.Err(); pe != nil {
			triggerErr = pe
		}
	}

	// ---------- Phase 4: Stop (sequential, reverse Register order) ----------
	var stopErrs []error
	for i := len(items) - 1; i >= 0; i-- {
		runningMu.Lock()
		wasRunning := running[i]
		runningMu.Unlock()
		if !wasRunning {
			continue
		}
		stopCtx, stopCancel := context.WithTimeout(context.Background(), stopTimeout)
		done := make(chan error, 1)
		go func(idx int) {
			done <- safeCall(func() error { return items[idx].Stop(stopCtx) })
		}(i)
		select {
		case err := <-done:
			if err != nil {
				stopErrs = append(stopErrs,
					fmt.Errorf("component %q stop: %w", items[i].Name, err))
			}
		case <-stopCtx.Done():
			stopErrs = append(stopErrs,
				fmt.Errorf("component %q stop: %w", items[i].Name, ErrStopTimeout))
		}
		stopCancel()
	}

	// ---------- Phase 5: Drain remaining Start goroutines ----------
	// They should all be exiting now that internalCtx is cancelled and
	// each Stop has signalled its component. Bound this drain too.
	doneAll := make(chan struct{})
	go func() {
		startWG.Wait()
		close(doneAll)
	}()
	select {
	case <-doneAll:
	case <-time.After(stopTimeout):
		stopErrs = append(stopErrs,
			fmt.Errorf("start goroutines did not exit before deadline: %w", ErrStopTimeout))
	}
	close(startResults)

	// ---------- Aggregate ----------
	if triggerErr == nil && len(stopErrs) == 0 {
		return nil
	}
	all := make([]error, 0, 1+len(stopErrs))
	if triggerErr != nil {
		all = append(all, triggerErr)
	}
	all = append(all, stopErrs...)
	return errors.Join(all...)
}

// safeCall runs fn and recovers panics, converting them to an error
// with a stack trace included.
func safeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}
