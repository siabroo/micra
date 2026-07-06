package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/siabroo/micra/core/internal/lifecycle"
	"github.com/siabroo/micra/core/internal/signals"
)

// App owns a graph of Components and runs their lifecycle.
type App struct {
	cfg appConfig

	mu         sync.Mutex
	components []Component
	names      map[string]struct{}
	started    bool
}

type appConfig struct {
	name        string
	version     string
	rawLogger   Logger
	logger      Logger // rawLogger.With("service", name, "version", version)
	stopTimeout time.Duration
	signals     []os.Signal
	noSignals   bool
}

// Option configures App via New.
type Option func(*appConfig)

// WithName sets the service name. When non-empty, added as the
// "service" tag on the logger and exposed via App.Name().
func WithName(name string) Option { return func(c *appConfig) { c.name = name } }

// WithVersion sets the service version (typically git commit hash).
// When non-empty, added as the "version" tag on the logger and
// exposed via App.Version().
func WithVersion(version string) Option { return func(c *appConfig) { c.version = version } }

// WithLogger sets the base logger. App tags it with service+version
// internally and propagates the tagged logger via ctx to every
// Component's Start.
func WithLogger(l Logger) Option { return func(c *appConfig) { c.rawLogger = l } }

// WithStopTimeout sets the deadline used for the ctx passed to each
// Component's Stop.
func WithStopTimeout(d time.Duration) Option {
	return func(c *appConfig) { c.stopTimeout = d }
}

// WithSignals sets which OS signals trigger shutdown.
// Default: SIGINT, SIGTERM. At least one signal must be passed; use
// WithoutSignals to disable.
func WithSignals(sigs ...os.Signal) Option {
	return func(c *appConfig) {
		if len(sigs) > 0 {
			c.signals = sigs
			c.noSignals = false
		}
	}
}

// WithoutSignals disables the signal trap entirely.
func WithoutSignals() Option { return func(c *appConfig) { c.noSignals = true } }

// New creates an App with the given options.
func New(opts ...Option) *App {
	cfg := appConfig{
		stopTimeout: 30 * time.Second,
		signals:     []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.rawLogger == nil {
		cfg.rawLogger = NewNoOpLogger()
	}
	tagged := cfg.rawLogger
	var tags []any
	if cfg.name != "" {
		tags = append(tags, "service", cfg.name)
	}
	if cfg.version != "" {
		tags = append(tags, "version", cfg.version)
	}
	if len(tags) > 0 {
		tagged = tagged.With(tags...)
	}
	cfg.logger = tagged
	return &App{cfg: cfg, names: make(map[string]struct{})}
}

// Name returns the configured service name.
func (a *App) Name() string { return a.cfg.name }

// Version returns the configured service version.
func (a *App) Version() string { return a.cfg.version }

// Logger returns the App's base logger (tagged with service and
// version).
func (a *App) Logger() Logger { return a.cfg.logger }

// Register adds a Component to the App. Returns ErrDuplicateComponent
// if a Component with this Name is already registered, or
// ErrAppAlreadyStarted if the App has already been Run.
func (a *App) Register(c Component) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started {
		return ErrAppAlreadyStarted
	}
	if _, dup := a.names[c.Name()]; dup {
		return fmt.Errorf("%w: %q", ErrDuplicateComponent, c.Name())
	}
	a.names[c.Name()] = struct{}{}
	a.components = append(a.components, c)
	return nil
}

// Run drives the App through Init → Start, blocks on signal/error,
// then Stop. See spec §6.3 and §9.1.
//
// Returns nil on a clean signal-driven shutdown (SIGINT/SIGTERM or
// parent ctx cancellation). Non-nil results indicate a Start failure,
// a Stop failure, or ErrStopTimeout leakage. Errors solely consisting
// of context.Canceled (the cancel signal micra used to drive teardown)
// are stripped so daemons can return process exit code 0 on clean
// shutdown.
func (a *App) Run(ctx context.Context) error {
	if err := a.markStarted(); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ContextWithLogger(ctx, a.cfg.logger))
	defer cancel()
	stop := a.installSignals(runCtx, cancel)
	defer stop()
	items := a.buildItems()
	return withoutCanceled(lifecycle.Run(runCtx, items, a.cfg.stopTimeout))
}

// RunOnce drives the App through Init → Start → fn → Stop. See spec
// §6.4 and §9.2.
//
// fn runs after every registered Component has had Start launched (in
// parallel goroutines, as with Run). When fn returns, the App shuts
// down regardless of whether fn returned an error. The error returned
// from RunOnce is fn's error (if any) joined with any errors that
// occurred during Component shutdown. A clean shutdown (fn returns nil,
// all Stops succeed) yields a nil return.
func (a *App) RunOnce(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := a.markStarted(); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ContextWithLogger(ctx, a.cfg.logger))
	defer cancel()
	stop := a.installSignals(runCtx, cancel)
	defer stop()

	// We add a synthetic item at the end whose Start runs fn and then
	// returns whatever fn returned. If fn returns nil, the synthetic
	// Start returns nil — lifecycle.Run will then unblock only when the
	// parent ctx is cancelled, so we cancel it ourselves. If fn returns
	// non-nil, lifecycle.Run treats it as a Start error and fail-fasts.
	//
	// We use a sentinel-style "ok" return path: when fn returns nil, the
	// synthetic Start calls cancel() to drive shutdown. In that case
	// lifecycle.Run will see ctx.Err() == context.Canceled and wrap it
	// into triggerErr. We strip context.Canceled from the joined return
	// here because it's not a real error — it's the signal we sent to
	// initiate teardown.
	items := a.buildItems()
	var (
		fnErr   error
		fnMu    sync.Mutex
		fnDone  bool
	)
	items = append(items, lifecycle.Item{
		Name: "__runonce_fn__",
		Start: func(ctx context.Context) error {
			err := fn(ctx)
			fnMu.Lock()
			fnErr = err
			fnDone = true
			fnMu.Unlock()
			// fn done — drive shutdown. We return nil here so that
			// lifecycle.Run does not race to capture our error vs the
			// resulting ctx cancellation. RunOnce reads fnErr below.
			cancel()
			return nil
		},
		Stop: func(context.Context) error { return nil },
	})
	lifecycleErr := lifecycle.Run(runCtx, items, a.cfg.stopTimeout)

	fnMu.Lock()
	captured := fnErr
	done := fnDone
	fnMu.Unlock()

	// lifecycle's error will include context.Canceled because we cancel
	// the parent ctx to drive shutdown; strip it from the aggregate.
	// Keep ErrStopTimeout and any other genuine shutdown errors.
	lifecycleErr = withoutCanceled(lifecycleErr)

	if done && captured != nil {
		if lifecycleErr != nil {
			return errors.Join(captured, lifecycleErr)
		}
		return captured
	}
	return lifecycleErr
}

// withoutCanceled returns nil if err is (or unwraps to) only
// context.Canceled; otherwise returns err unchanged. Used by RunOnce
// to suppress the self-induced cancel signal.
func withoutCanceled(err error) error {
	// errors.Join returns an *joinError implementing Unwrap() []error.
	type unwrapper interface{ Unwrap() []error }
	if u, ok := err.(unwrapper); ok {
		var kept []error
		for _, e := range u.Unwrap() {
			if !errors.Is(e, context.Canceled) {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			return nil
		}
		return errors.Join(kept...)
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (a *App) markStarted() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started {
		return ErrAppAlreadyStarted
	}
	a.started = true
	return nil
}

func (a *App) installSignals(ctx context.Context, cancel context.CancelFunc) func() {
	if a.cfg.noSignals {
		return func() {}
	}
	return signals.Trap(ctx, cancel, a.cfg.signals...)
}

func (a *App) buildItems() []lifecycle.Item {
	a.mu.Lock()
	cs := append([]Component(nil), a.components...)
	a.mu.Unlock()
	items := make([]lifecycle.Item, 0, len(cs))
	for _, c := range cs {
		c := c
		item := lifecycle.Item{Name: c.Name(), Start: c.Start, Stop: c.Stop}
		if init, ok := c.(Initializer); ok {
			item.Init = init.Init
		}
		items = append(items, item)
	}
	return items
}
