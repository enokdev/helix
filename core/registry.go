package core

import "reflect"

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

// NewComponentRegistration creates a ComponentRegistration with safe defaults
// (ScopeSingleton, Lazy = false). Use this instead of a struct literal to avoid
// the zero-value trap where Scope would be "" instead of ScopeSingleton.
func NewComponentRegistration(component any) ComponentRegistration {
	return ComponentRegistration{
		Component: component,
		Scope:     ScopeSingleton,
	}
}

func normalizeComponentRegistration(input any) (ComponentRegistration, reflect.Type, error) {
	registration, ok := input.(ComponentRegistration)
	if !ok {
		registration = NewComponentRegistration(input)
	}
	if registration.Scope == "" {
		registration.Scope = ScopeSingleton
	}

	componentValue := reflect.ValueOf(registration.Component)
	if !isRegistrableComponent(componentValue) {
		return ComponentRegistration{}, nil, ErrUnresolvable
	}

	if !registration.Scope.isValid() {
		return ComponentRegistration{}, nil, ErrUnresolvable
	}

	return registration, componentValue.Type(), nil
}

func (s Scope) isValid() bool {
	switch s {
	case ScopeSingleton, ScopePrototype:
		return true
	default:
		return false
	}
}
