package scheduler

import (
	"context"
	"fmt"

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
	OnStop() error
}

// Compile-time assertions
var _ core.Lifecycle = (Scheduler)(nil)

type adapterWrapper struct {
	inner *internal.CronAdapter
}

func (w *adapterWrapper) Register(job Job) error {
	if job.Fn == nil {
		return fmt.Errorf("%w: job %q has nil Fn", ErrInvalidCron, job.Name)
	}
	if err := w.inner.RegisterRaw(job.Name, job.Expr, job.Fn); err != nil {
		return fmt.Errorf("scheduler: register %q: %w: %w", job.Name, ErrInvalidCron, err)
	}
	return nil
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

func (w *adapterWrapper) OnStop() error {
	return w.inner.OnStop()
}

var (
	_ Scheduler      = (*adapterWrapper)(nil)
	_ core.Lifecycle = (*adapterWrapper)(nil)
)

// NewScheduler returns a new Scheduler backed by robfig/cron v3.
func NewScheduler() Scheduler {
	return &adapterWrapper{
		inner: internal.NewCronAdapter(),
	}
}
