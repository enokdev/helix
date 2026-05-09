package internal

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
)

// EntryID is the internal handle returned by robfig/cron for a registered job.
type EntryID = cron.EntryID

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
func (a *CronAdapter) RegisterRaw(name, expr string, fn func()) (EntryID, error) {
	if fn == nil {
		return 0, fmt.Errorf("scheduler: job %q: fn must not be nil", name)
	}
	id, err := a.cron.AddFunc(expr, fn)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Remove deletes a previously registered cron entry.
func (a *CronAdapter) Remove(id EntryID) {
	a.cron.Remove(id)
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
// Calling Stop(ctx) before OnStop(ctx) is safe: robfig/cron.Stop() is idempotent
// and returns an already-done context when called a second time.
func (a *CronAdapter) OnStop(ctx context.Context) error {
	stopCtx := a.cron.Stop()
	select {
	case <-stopCtx.Done():
	case <-ctx.Done():
		return fmt.Errorf("scheduler: shutdown timed out: %w", ctx.Err())
	}
	return nil
}
