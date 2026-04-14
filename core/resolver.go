package core

// DependencyGraph represents the resolved component dependency graph.
// Nodes holds the type names of all registered components.
// Edges maps each component type name to its list of dependencies.
type DependencyGraph struct {
	Nodes []string
	Edges map[string][]string
}

// Resolver is the abstraction layer for dependency resolution strategies.
// Two implementations are planned:
//   - ReflectResolver: runtime reflection (default, Phase 1)
//   - WireResolver:    compile-time codegen (opt-in, Phase 4)
type Resolver interface {
	Register(component any) error
	Resolve(target any) error
	Graph() DependencyGraph
}
