package core

// Lifecycle is implemented by components that need to hook into the
// application start/stop sequence.
//
// OnStart is called after all dependencies are resolved, before the
// HTTP server begins accepting connections.
//
// OnStop is called on SIGTERM/SIGINT, in reverse dependency order,
// allowing components to flush buffers and release resources.
type Lifecycle interface {
	OnStart() error
	OnStop() error
}
