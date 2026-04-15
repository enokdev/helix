package core

import (
	"log/slog"
	"time"
)

// DefaultShutdownTimeout is the lifecycle shutdown budget used when no custom
// timeout is configured by the application bootstrap.
const DefaultShutdownTimeout = 30 * time.Second

// Option is a functional option for configuring a Container.
type Option func(*Container)

// WithResolver sets the Resolver implementation used by the Container.
// Without this option, Register and Resolve return ErrUnresolvable.
// Panics if r is nil.
func WithResolver(r Resolver) Option {
	if r == nil {
		panic("helix: WithResolver: resolver must not be nil")
	}
	return func(c *Container) {
		c.resolver = r
	}
}

// WithShutdownTimeout overrides the lifecycle shutdown budget.
func WithShutdownTimeout(timeout time.Duration) Option {
	if timeout <= 0 {
		panic("helix: WithShutdownTimeout: timeout must be positive")
	}
	return func(c *Container) {
		c.shutdownTimeout = timeout
	}
}

// WithLogger overrides the logger used for lifecycle shutdown errors.
func WithLogger(logger *slog.Logger) Option {
	if logger == nil {
		panic("helix: WithLogger: logger must not be nil")
	}
	return func(c *Container) {
		c.logger = logger
	}
}
