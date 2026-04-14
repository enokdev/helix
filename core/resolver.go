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
