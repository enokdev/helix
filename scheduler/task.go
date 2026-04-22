package scheduler

import (
	"log/slog"
	"sync/atomic"
)

// WrapError wraps a func() error as func(), logging non-nil errors via slog.
// The job name is included as a structured slog field for observability.
func WrapError(name string, fn func() error) func() {
	return func() {
		if err := fn(); err != nil {
			slog.Default().Error("scheduler: job error", "job", name, "error", err)
		}
	}
}

// WrapSkipIfBusy wraps fn so concurrent invocations are skipped.
func WrapSkipIfBusy(fn func()) func() {
	var running atomic.Bool
	return func() {
		if !running.CompareAndSwap(false, true) {
			return
		}
		defer running.Store(false)
		fn()
	}
}
