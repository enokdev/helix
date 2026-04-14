package core

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
