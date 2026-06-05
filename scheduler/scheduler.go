package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/enokdev/helix/core"
	"github.com/enokdev/helix/scheduler/internal"
)

// Scheduler manages cron job registration and execution.
type Scheduler interface {
	// Register adds a job to the scheduler. Returns ErrInvalidCron if the expression is invalid.
	Register(job Job) error
	// Start begins the background cron runner (non-blocking).
	Start()
	// Stop halts the scheduler, waiting for all running jobs to complete.
	Stop(ctx context.Context)
	// OnStart implements core.Lifecycle — starts the scheduler on application start.
	OnStart() error
	// OnStop implements core.Lifecycle — stops the scheduler on application shutdown.
	// Calling Stop(ctx) before OnStop(ctx) is safe: robfig/cron.Stop() is idempotent.
	OnStop(ctx context.Context) error
}

// Compile-time assertions
var _ core.Lifecycle = (Scheduler)(nil)

const defaultDistributedLockTTL = time.Minute

type Option func(*adapterWrapper)

type adapterWrapper struct {
	mu      sync.Mutex
	inner   *internal.CronAdapter
	byName  map[string]registeredJob
	lock    DistributedLock
	lockTTL time.Duration
}

type registeredJob struct {
	id internal.EntryID
	fn func()
}

func (w *adapterWrapper) Register(job Job) error {
	name := strings.TrimSpace(job.Name)
	if name == "" {
		return fmt.Errorf("%w: job name must not be empty", ErrInvalidJob)
	}
	if name != job.Name {
		return fmt.Errorf("%w: job name %q must not contain surrounding whitespace", ErrInvalidJob, job.Name)
	}
	if job.Fn == nil {
		return fmt.Errorf("%w: job %q has nil Fn", ErrInvalidJob, job.Name)
	}

	fn := job.Fn
	if !job.AllowConcurrent {
		fn = WrapSkipIfBusy(fn)
	}
	fn = wrapDistributedLock(w.lock, w.lockTTL, name, fn)
	fn = recoverPanic(name, fn)

	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.byName[name]; exists {
		return fmt.Errorf("%w: job %q already registered", ErrDuplicateJob, name)
	}

	id, err := w.inner.RegisterRaw(name, job.Expr, fn)
	if err != nil {
		return fmt.Errorf("scheduler: register %q: %w: %w", job.Name, ErrInvalidCron, err)
	}
	w.byName[name] = registeredJob{id: id, fn: fn}
	return nil
}

func (w *adapterWrapper) Unregister(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%w: job name must not be empty", ErrInvalidJob)
	}
	if trimmed != name {
		return fmt.Errorf("%w: job name %q must not contain surrounding whitespace", ErrInvalidJob, name)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	job, ok := w.byName[trimmed]
	if !ok {
		return fmt.Errorf("%w: job %q", ErrJobNotFound, trimmed)
	}
	w.inner.Remove(job.id)
	delete(w.byName, trimmed)
	return nil
}

func recoverPanic(name string, fn func()) func() {
	return func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Default().Error("scheduler: job panic", "job", name, "panic", recovered)
			}
		}()
		fn()
	}
}

func wrapDistributedLock(lock DistributedLock, ttl time.Duration, name string, fn func()) func() {
	if lock == nil {
		return fn
	}

	return func() {
		ctx := context.Background()
		acquired, err := lock.Acquire(ctx, name, ttl)
		if err != nil {
			slog.Default().Error("scheduler: distributed lock acquire failed", "job", name, "error", err)
			return
		}
		if !acquired {
			slog.Default().Debug("scheduler: distributed lock busy", "job", name)
			return
		}
		defer func() {
			if err := lock.Release(ctx, name); err != nil {
				slog.Default().Error("scheduler: distributed lock release failed", "job", name, "error", err)
			}
		}()
		fn()
	}
}

func (w *adapterWrapper) Start() {
	w.inner.Start()
}

func (w *adapterWrapper) Stop(ctx context.Context) {
	w.inner.Stop(ctx)
}

func (w *adapterWrapper) OnStart() error {
	return w.inner.OnStart()
}

func (w *adapterWrapper) OnStop(ctx context.Context) error {
	return w.inner.OnStop(ctx)
}

var (
	_ Scheduler      = (*adapterWrapper)(nil)
	_ core.Lifecycle = (*adapterWrapper)(nil)
)

// WithDistributedLock configures a lock used to suppress duplicate job execution
// across scheduler instances. Passing nil restores the single-instance default.
func WithDistributedLock(lock DistributedLock) Option {
	return func(w *adapterWrapper) {
		if lock == nil {
			w.lock = &NoOpLock{}
			return
		}
		w.lock = lock
	}
}

// WithDistributedLockTTL overrides the TTL passed to DistributedLock.Acquire.
// Non-positive values restore the default TTL.
func WithDistributedLockTTL(ttl time.Duration) Option {
	return func(w *adapterWrapper) {
		if ttl <= 0 {
			w.lockTTL = defaultDistributedLockTTL
			return
		}
		w.lockTTL = ttl
	}
}

// NewScheduler returns a new Scheduler backed by robfig/cron v3.
func NewScheduler(opts ...Option) Scheduler {
	scheduler := &adapterWrapper{
		inner:   internal.NewCronAdapter(),
		byName:  make(map[string]registeredJob),
		lock:    &NoOpLock{},
		lockTTL: defaultDistributedLockTTL,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(scheduler)
		}
	}
	return scheduler
}
