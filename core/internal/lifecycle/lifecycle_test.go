package lifecycle

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// trackingComponent records phase transitions in the shared phases
// slice so tests can assert ordering across multiple components.
type trackingComponent struct {
	name string
	mu   *sync.Mutex
	log  *[]string
	initErr  error
	startErr error
	stopErr  error
	startBlock chan struct{} // optional — Start blocks on it instead of ctx.Done
}

func (t *trackingComponent) Name() string { return t.name }

func (t *trackingComponent) Init(ctx context.Context) error {
	t.mu.Lock()
	*t.log = append(*t.log, t.name+":init")
	t.mu.Unlock()
	return t.initErr
}

func (t *trackingComponent) Start(ctx context.Context) error {
	t.mu.Lock()
	*t.log = append(*t.log, t.name+":start")
	t.mu.Unlock()
	if t.startErr != nil {
		return t.startErr
	}
	if t.startBlock != nil {
		select {
		case <-t.startBlock:
		case <-ctx.Done():
		}
	} else {
		<-ctx.Done()
	}
	return nil
}

func (t *trackingComponent) Stop(ctx context.Context) error {
	t.mu.Lock()
	*t.log = append(*t.log, t.name+":stop")
	t.mu.Unlock()
	return t.stopErr
}

func TestRun_HappyPath_StartsAndStopsInOrder(t *testing.T) {
	var mu sync.Mutex
	var log []string
	a := &trackingComponent{name: "a", mu: &mu, log: &log}
	b := &trackingComponent{name: "b", mu: &mu, log: &log}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Run(ctx, []Item{
		{Name: "a", Init: a.Init, Start: a.Start, Stop: a.Stop},
		{Name: "b", Init: b.Init, Start: b.Start, Stop: b.Stop},
	}, 1*time.Second)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v, want nil or context.Canceled", err)
	}

	mu.Lock()
	got := append([]string(nil), log...)
	mu.Unlock()

	// Init order: a, b (sequential).
	// Start order: not strictly defined relative to each other (goroutines),
	// but both must appear AFTER both inits.
	// Stop order: b, a (reverse sequential).
	if got[0] != "a:init" || got[1] != "b:init" {
		t.Errorf("init order wrong: %v", got)
	}
	// Find positions of stops; b:stop must come before a:stop.
	bStop, aStop := -1, -1
	for i, evt := range got {
		if evt == "b:stop" {
			bStop = i
		}
		if evt == "a:stop" {
			aStop = i
		}
	}
	if bStop == -1 || aStop == -1 || bStop >= aStop {
		t.Errorf("stop order wrong: %v (expected b:stop before a:stop)", got)
	}
}

func TestRun_InitError_AbortsBeforeAnyStart(t *testing.T) {
	var mu sync.Mutex
	var log []string
	a := &trackingComponent{name: "a", mu: &mu, log: &log}
	bErr := errors.New("b init failed")
	b := &trackingComponent{name: "b", mu: &mu, log: &log, initErr: bErr}
	c := &trackingComponent{name: "c", mu: &mu, log: &log}

	err := Run(context.Background(), []Item{
		{Name: "a", Init: a.Init, Start: a.Start, Stop: a.Stop},
		{Name: "b", Init: b.Init, Start: b.Start, Stop: b.Stop},
		{Name: "c", Init: c.Init, Start: c.Start, Stop: c.Stop},
	}, 1*time.Second)

	if !errors.Is(err, bErr) {
		t.Fatalf("Run returned %v, want %v", err, bErr)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, evt := range log {
		if evt == "c:init" || evt == "a:start" || evt == "b:start" || evt == "c:start" {
			t.Errorf("post-failure event observed: %v in log %v", evt, log)
		}
		if evt == "a:stop" || evt == "b:stop" || evt == "c:stop" {
			t.Errorf("stop should not run when Init aborts: %v in log %v", evt, log)
		}
	}
}

func TestRun_StartError_FailsFastAndStopsRunning(t *testing.T) {
	var mu sync.Mutex
	var log []string
	a := &trackingComponent{name: "a", mu: &mu, log: &log}
	bErr := errors.New("b start failed")
	b := &trackingComponent{name: "b", mu: &mu, log: &log, startErr: bErr}

	err := Run(context.Background(), []Item{
		{Name: "a", Init: a.Init, Start: a.Start, Stop: a.Stop},
		{Name: "b", Init: b.Init, Start: b.Start, Stop: b.Stop},
	}, 1*time.Second)

	if !errors.Is(err, bErr) {
		t.Fatalf("Run returned %v, want %v", err, bErr)
	}

	mu.Lock()
	defer mu.Unlock()
	// a is running → must be stopped. b returned error → not stopped.
	stopped := map[string]bool{}
	for _, evt := range log {
		if len(evt) > 5 && evt[len(evt)-5:] == ":stop" {
			stopped[evt[:len(evt)-5]] = true
		}
	}
	if !stopped["a"] {
		t.Errorf("a should be stopped, log: %v", log)
	}
}

func TestRun_NoInit_SkipsInitPhase(t *testing.T) {
	var calls int32
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Run(ctx, []Item{
		{
			Name: "x",
			// Init left nil — coordinator must skip Init phase for this item.
			Start: func(ctx context.Context) error {
				atomic.AddInt32(&calls, 1)
				<-ctx.Done()
				return nil
			},
			Stop: func(ctx context.Context) error { return nil },
		},
	}, 1*time.Second)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v, want nil or context.Canceled", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("Start was called %d times, want 1", got)
	}
}

func TestRun_StopTimeoutLeaksAndContinues(t *testing.T) {
	hangStop := make(chan struct{}) // never closed — Stop hangs forever
	stopCalled := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := Run(ctx, []Item{
		{
			Name: "hanger",
			Start: func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			},
			Stop: func(ctx context.Context) error {
				stopCalled <- struct{}{}
				select {
				case <-hangStop:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		},
	}, 50*time.Millisecond) // tiny stop timeout

	// err must include ErrStopTimeout in the chain.
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrStopTimeout) {
		t.Errorf("err = %v, want chain to include ErrStopTimeout", err)
	}
	close(hangStop) // let the hung Stop goroutine exit
}
