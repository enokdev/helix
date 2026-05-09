package core

import (
	"fmt"
	"reflect"
)

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
	// ResolveAs limits interface resolution to the listed interface types.
	ResolveAs []reflect.Type
	// ExcludeFrom removes this component from the listed interface resolutions.
	ExcludeFrom []reflect.Type
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
	if registration.Scope == ScopePrototype && registration.Lazy {
		return ComponentRegistration{}, nil, fmt.Errorf("ScopePrototype cannot be combined with Lazy: %w", ErrUnresolvable)
	}
	if err := validateInterfaceTypeList("ResolveAs", registration.ResolveAs); err != nil {
		return ComponentRegistration{}, nil, err
	}
	if err := validateInterfaceTypeList("ExcludeFrom", registration.ExcludeFrom); err != nil {
		return ComponentRegistration{}, nil, err
	}
	registration.ResolveAs = append([]reflect.Type(nil), registration.ResolveAs...)
	registration.ExcludeFrom = append([]reflect.Type(nil), registration.ExcludeFrom...)

	return registration, componentValue.Type(), nil
}

func validateInterfaceTypeList(name string, types []reflect.Type) error {
	for _, typ := range types {
		if typ == nil || typ.Kind() != reflect.Interface {
			return fmt.Errorf("%s must contain interface types only: %w", name, ErrUnresolvable)
		}
	}
	return nil
}

func (s Scope) isValid() bool {
	switch s {
	case ScopeSingleton, ScopePrototype:
		return true
	default:
		return false
	}
}
