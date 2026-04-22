package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// CronAdapter isolates the robfig/cron/v3 dependency.
type CronAdapter struct {
	cron *cron.Cron
}

// NewCronAdapter creates a new CronAdapter backed by robfig/cron v3.
func NewCronAdapter() *CronAdapter {
	return &CronAdapter{
		cron: cron.New(),
	}
}

// RegisterRaw registers a cron function directly.
func (a *CronAdapter) RegisterRaw(name, expr string, fn func()) error {
	if fn == nil {
		return fmt.Errorf("scheduler: job %q: fn must not be nil", name)
	}
	if _, err := a.cron.AddFunc(expr, fn); err != nil {
		return err
	}
	return nil
}

// Start begins the background cron runner (non-blocking).
func (a *CronAdapter) Start() {
	a.cron.Start()
}

// Stop halts the scheduler, waiting for all running jobs to complete.
func (a *CronAdapter) Stop(ctx context.Context) {
	stopCtx := a.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-ctx.Done():
	}
}

// OnStart implements core.Lifecycle — starts the scheduler on application start.
func (a *CronAdapter) OnStart() error {
	a.Start()
	return nil
}

// OnStop implements core.Lifecycle — stops the scheduler on application shutdown.
// Returns an error if jobs do not complete within 30 seconds.
func (a *CronAdapter) OnStop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stopCtx := a.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-ctx.Done():
		return fmt.Errorf("scheduler: shutdown timed out: %w", ctx.Err())
	}
	return nil
}
