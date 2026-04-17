package core

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Container is the Helix dependency injection container.
// It delegates all registration and resolution to a pluggable Resolver.
// Use NewContainer with WithResolver to configure a concrete resolver.
type Container struct {
	mu              sync.Mutex
	resolver        Resolver
	valueLookup     func(key string) (any, bool)
	logger          *slog.Logger
	shutdownTimeout time.Duration
	lifecycle       lifecycleState
}

// Register adds a component to the container's resolver registry.
// Returns ErrUnresolvable if no Resolver has been configured or component is nil.
func (c *Container) Register(component any) error {
	if component == nil {
		return fmt.Errorf("core: register: %w", ErrUnresolvable)
	}
	if c.resolver == nil {
		return fmt.Errorf("core: register %T: %w", component, ErrUnresolvable)
	}
	return c.resolver.Register(component)
}

// Resolve populates target with the registered component matching its type.
// Returns ErrUnresolvable if no Resolver has been configured or target is nil.
func (c *Container) Resolve(target any) error {
	if target == nil {
		return fmt.Errorf("core: resolve: %w", ErrUnresolvable)
	}
	if c.resolver == nil {
		return fmt.Errorf("core: resolve %T: %w", target, ErrUnresolvable)
	}
	return c.resolver.Resolve(target)
}

// NewContainer creates a new Container and applies the provided options.
// Without WithResolver, Register and Resolve will return ErrUnresolvable.
func NewContainer(opts ...Option) *Container {
	c := &Container{
		logger:          slog.Default(),
		shutdownTimeout: DefaultShutdownTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Start resolves lifecycle-aware singleton components and invokes OnStart in
// dependency order. Lazy components are intentionally ignored until a future
// bootstrap phase decides to resolve them explicitly.
func (c *Container) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.resolver == nil {
		return fmt.Errorf("core: start: %w", ErrUnresolvable)
	}
	if c.lifecycle.hasStarted {
		return nil
	}

	components, err := c.resolveLifecycleComponents()
	if err != nil {
		return fmt.Errorf("core: start: %w", err)
	}

	started := make([]startedLifecycle, 0, len(components))
	for _, component := range components {
		if err := component.instance.OnStart(); err != nil {
			startErr := fmt.Errorf("core: start %s: %w", component.name, err)
			stopErr := c.stopStartedComponents(started)
			c.lifecycle = lifecycleState{}
			if stopErr != nil {
				return errors.Join(startErr, fmt.Errorf("core: cleanup after start failure: %w", stopErr))
			}
			return startErr
		}
		started = append(started, component)
	}

	c.lifecycle = lifecycleState{hasStarted: true, started: started}
	return nil
}

// Shutdown stops only the lifecycle components that successfully completed
// OnStart, in reverse startup order.
func (c *Container) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.lifecycle.hasStarted {
		return nil
	}

	err := c.stopStartedComponents(c.lifecycle.started)
	c.lifecycle = lifecycleState{}
	if err != nil {
		return fmt.Errorf("core: shutdown: %w", err)
	}
	return nil
}
