package core

import "fmt"

// Container is the Helix dependency injection container.
// It delegates all registration and resolution to a pluggable Resolver.
// Use NewContainer with WithResolver to configure a concrete resolver.
type Container struct {
	resolver Resolver
}

// Register adds a component to the container's resolver registry.
// Returns ErrUnresolvable if no Resolver has been configured.
func (c *Container) Register(component any) error {
	if c.resolver == nil {
		return fmt.Errorf("core: register %T: %w", component, ErrUnresolvable)
	}
	return c.resolver.Register(component)
}

// Resolve populates target with the registered component matching its type.
// Returns ErrUnresolvable if no Resolver has been configured.
func (c *Container) Resolve(target any) error {
	if c.resolver == nil {
		return fmt.Errorf("core: resolve %T: %w", target, ErrUnresolvable)
	}
	return c.resolver.Resolve(target)
}

// NewContainer creates a new Container and applies the provided options.
// Without WithResolver, Register and Resolve will return ErrUnresolvable.
func NewContainer(opts ...Option) *Container {
	c := &Container{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
