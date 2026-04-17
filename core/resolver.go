package core

// DependencyGraph represents the resolved component dependency graph.
// Nodes holds the type names of all registered components.
// Edges maps each component type name to its list of dependencies.
type DependencyGraph struct {
	Nodes []string
	Edges map[string][]string
}

// Resolver is the abstraction layer for dependency resolution strategies.
// Two implementations are provided:
//   - ReflectResolver: runtime reflection, zero configuration required (default)
//   - WireResolver:    compile-time code generation, zero reflection in production (opt-in)
type Resolver interface {
	Register(component any) error
	Resolve(target any) error
	Graph() DependencyGraph
}

// LifecycleCandidate holds a resolved non-lazy singleton component that
// implements the Lifecycle interface.
type LifecycleCandidate struct {
	Name     string
	Instance Lifecycle
}

// LifecycleResolver is an optional capability that a Resolver may implement
// to enable Container.Start() and Container.Shutdown(). Resolvers that do not
// implement this interface cause Start() to return a clear error.
// ReflectResolver satisfies this interface by default.
type LifecycleResolver interface {
	// LifecycleCandidates returns all non-lazy singleton components implementing
	// Lifecycle, in registration order. Dependency ordering is applied by the caller.
	LifecycleCandidates() ([]LifecycleCandidate, error)
}
