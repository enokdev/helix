package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"sync"
	"time"
)

const reloadIntervalKey = "helix.config.reload-interval"

var knownConfigKeys = []string{
	reloadIntervalKey,
	"helix.logging.level",
	"helix.logging.levels.web",
	"helix.starters.observability.tracing.enabled",
	"helix.starters.observability.tracing.exporter",
	"helix.starters.observability.tracing.endpoint",
	"helix.starters.observability.tracing.service-name",
}

// Reloadable is implemented by components that need to react after a
// successful configuration reload.
type Reloadable interface {
	OnConfigReload()
}

// Reloader reloads configuration on demand or from explicit background triggers.
// Callers must hold RLock while reading the decoded config struct to avoid data
// races with concurrent background reloads.
type Reloader interface {
	Reload() error
	Start(ctx context.Context) error
	// RLock acquires a shared read lock. Hold it while reading the config target
	// to prevent races with a concurrent background reload.
	RLock()
	// RUnlock releases the read lock acquired by RLock.
	RUnlock()
}

// ReloadOption configures a Reloader.
type ReloadOption func(*reloader) error

type signalSource func(context.Context) (<-chan os.Signal, func(), error)

type reloadTicker interface {
	C() <-chan time.Time
	Stop()
}

type realReloadTicker struct {
	ticker *time.Ticker
}

func (t realReloadTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t realReloadTicker) Stop() {
	t.ticker.Stop()
}

type tickerFactory func(time.Duration) reloadTicker

type reloader struct {
	mu               sync.RWMutex
	loader           Loader
	target           any
	reloadables      []Reloadable
	interval         time.Duration
	intervalExplicit bool
	logger           *slog.Logger
	signalSource     signalSource
	tickerFactory    tickerFactory
}

// NewReloader creates an opt-in configuration reloader.
func NewReloader(loader Loader, target any, opts ...ReloadOption) (Reloader, error) {
	if loader == nil {
		return nil, fmt.Errorf("config: create reloader: %w", ErrInvalidConfig)
	}
	if !isValidDecodeTarget(target) {
		return nil, fmt.Errorf("config: create reloader target %T: %w", target, ErrInvalidConfig)
	}

	r := &reloader{
		loader:        loader,
		target:        target,
		logger:        slog.Default(),
		signalSource:  defaultReloadSignalSource,
		tickerFactory: newRealReloadTicker,
	}
	for _, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("config: configure reloader: %w", ErrInvalidConfig)
		}
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// WithReloadables registers callbacks invoked after each successful reload.
func WithReloadables(reloadables ...Reloadable) ReloadOption {
	return func(r *reloader) error {
		if slices.ContainsFunc(reloadables, isNilReloadable) {
			return fmt.Errorf("config: configure reloadable: %w", ErrInvalidConfig)
		}
		r.reloadables = append(r.reloadables, reloadables...)
		return nil
	}
}

// isNilReloadable returns true for both untyped nil and typed-nil interface values.
func isNilReloadable(r Reloadable) bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(r)
	kind := v.Kind()
	return (kind == reflect.Pointer || kind == reflect.Interface ||
		kind == reflect.Chan || kind == reflect.Func ||
		kind == reflect.Map || kind == reflect.Slice) && v.IsNil()
}

// WithReloadInterval configures the polling interval. Non-positive intervals disable polling.
func WithReloadInterval(interval time.Duration) ReloadOption {
	return func(r *reloader) error {
		r.interval = interval
		r.intervalExplicit = true
		return nil
	}
}

// WithReloadLogger configures the logger used for background reload errors.
func WithReloadLogger(logger *slog.Logger) ReloadOption {
	return func(r *reloader) error {
		if logger == nil {
			return fmt.Errorf("config: configure reload logger: %w", ErrInvalidConfig)
		}
		r.logger = logger
		return nil
	}
}

func withReloadSignalSource(source signalSource) ReloadOption {
	return func(r *reloader) error {
		if source == nil {
			return fmt.Errorf("config: configure reload signal source: %w", ErrInvalidConfig)
		}
		r.signalSource = source
		return nil
	}
}

func withReloadTickerFactory(factory tickerFactory) ReloadOption {
	return func(r *reloader) error {
		if factory == nil {
			return fmt.Errorf("config: configure reload ticker: %w", ErrInvalidConfig)
		}
		r.tickerFactory = factory
		return nil
	}
}

func newRealReloadTicker(interval time.Duration) reloadTicker {
	return realReloadTicker{ticker: time.NewTicker(interval)}
}

func (r *reloader) RLock() {
	r.mu.RLock()
}

func (r *reloader) RUnlock() {
	r.mu.RUnlock()
}

func (r *reloader) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.loader.Load(r.target); err != nil {
		return err
	}
	for _, reloadable := range r.reloadables {
		reloadable.OnConfigReload()
	}
	return nil
}

func (r *reloader) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("config: start reloader: %w", ErrInvalidConfig)
	}

	interval, err := r.resolveInterval()
	if err != nil {
		return err
	}

	signals, stopSignals, err := r.signalSource(ctx)
	if err != nil {
		return fmt.Errorf("config: start reload signal handler: %w", err)
	}
	if stopSignals == nil {
		stopSignals = func() {}
	}
	defer stopSignals()

	var ticker reloadTicker
	var ticks <-chan time.Time
	if interval > 0 {
		ticker = r.tickerFactory(interval)
		if ticker == nil {
			return fmt.Errorf("config: start reload ticker: %w", ErrInvalidConfig)
		}
		defer ticker.Stop()
		ticks = ticker.C()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-signals:
			r.reloadAndLog()
		case <-ticks:
			r.reloadAndLog()
		}
	}
}

func (r *reloader) reloadAndLog() {
	if err := r.Reload(); err != nil {
		r.logger.Error("configuration reload failed", "error", err)
	}
}

func (r *reloader) resolveInterval() (time.Duration, error) {
	if r.intervalExplicit {
		if r.interval <= 0 {
			return 0, nil
		}
		return r.interval, nil
	}

	value, ok := r.loader.Lookup(reloadIntervalKey)
	if !ok {
		return 0, nil
	}

	interval, err := parseReloadInterval(value)
	if err != nil {
		return 0, fmt.Errorf("config: parse %s: %v: %w", reloadIntervalKey, err, ErrInvalidConfig)
	}
	if interval <= 0 {
		return 0, nil
	}
	return interval, nil
}

func parseReloadInterval(value any) (time.Duration, error) {
	switch typed := value.(type) {
	case time.Duration:
		return typed, nil
	case string:
		return time.ParseDuration(typed)
	default:
		return 0, fmt.Errorf("unsupported interval type %T: use a string like \"30s\"", value)
	}
}
