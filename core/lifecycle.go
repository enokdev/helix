package core

import "context"

// Lifecycle is implemented by components that need to hook into the
// application start/stop sequence.
//
// OnStart is called after all dependencies are resolved, before the
// HTTP server begins accepting connections.
//
// OnStop is called on SIGTERM/SIGINT, in reverse dependency order,
// allowing components to flush buffers and release resources.
// The provided context carries the remaining shutdown budget; implementations
// should honour ctx.Done() to avoid goroutine leaks.
type Lifecycle interface {
	OnStart() error
	OnStop(ctx context.Context) error
}
