package core

// Scope defines the instantiation strategy for a registered component.
type Scope string

const (
	// ScopeSingleton returns the same instance on every Resolve call (default).
	ScopeSingleton Scope = "singleton"
	// ScopePrototype creates a new instance on every Resolve call.
	ScopePrototype Scope = "prototype"
)

// ComponentRegistration holds the metadata for a registered component.
// It is used internally by Resolver implementations.
type ComponentRegistration struct {
	// Component is the registered value (pointer to struct).
	Component any
	// Scope controls how instances are created. Defaults to ScopeSingleton.
	Scope Scope
	// Lazy defers instantiation until the first Resolve call.
	Lazy bool
}
