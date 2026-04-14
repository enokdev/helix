package core

// Option is a functional option for configuring a Container.
type Option func(*Container)

// WithResolver sets the Resolver implementation used by the Container.
// Without this option, Register and Resolve return ErrUnresolvable.
func WithResolver(r Resolver) Option {
	return func(c *Container) {
		c.resolver = r
	}
}
