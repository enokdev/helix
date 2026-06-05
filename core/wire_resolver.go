package core

import (
	"fmt"
	"reflect"
	"sync"
)

var (
	_ Resolver          = (*WireResolver)(nil)
	_ LifecycleResolver = (*WireResolver)(nil)
)

// WireResolver stores pre-wired singleton instances generated at compile time.
type WireResolver struct {
	mu                sync.RWMutex
	instances         map[reflect.Type]any
	registrations     map[reflect.Type]ComponentRegistration
	registrationOrder []reflect.Type
}

// NewWireResolver creates a resolver for compile-time generated DI wiring.
func NewWireResolver() *WireResolver {
	return &WireResolver{
		instances:     make(map[reflect.Type]any),
		registrations: make(map[reflect.Type]ComponentRegistration),
	}
}

// Register stores a pre-wired component instance by its concrete pointer type.
func (r *WireResolver) Register(component any) error {
	registration, componentType, err := normalizeComponentRegistration(component)
	if err != nil {
		return fmt.Errorf("core: register %T: %w", component, err)
	}
	if registration.Scope != ScopeSingleton || registration.Lazy {
		return fmt.Errorf("core: register %T: wire resolver accepts eager singletons only: %w", component, ErrUnresolvable)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.instances[componentType]; exists {
		return fmt.Errorf("core: register %T: %w", registration.Component, ErrAlreadyRegistered)
	}
	if err := r.checkInterfaceConflicts(componentType, registration); err != nil {
		return fmt.Errorf("core: register %T: %w", registration.Component, err)
	}

	r.registrationOrder = append(r.registrationOrder, componentType)
	r.instances[componentType] = registration.Component
	r.registrations[componentType] = registration
	return nil
}

// Unregister removes a pre-wired component instance by concrete type.
func (r *WireResolver) Unregister(component any) error {
	_, componentType, err := normalizeComponentRegistration(component)
	if err != nil {
		return fmt.Errorf("core: unregister %T: %w", component, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.instances[componentType]; !exists {
		return fmt.Errorf("core: unregister %T: %w", component, ErrNotFound)
	}
	delete(r.instances, componentType)
	delete(r.registrations, componentType)
	r.registrationOrder = removeWireRegistrationType(r.registrationOrder, componentType)
	return nil
}

func removeWireRegistrationType(types []reflect.Type, target reflect.Type) []reflect.Type {
	for i, typ := range types {
		if typ == target {
			return append(types[:i], types[i+1:]...)
		}
	}
	return types
}

// Resolve assigns a registered pre-wired instance to target.
func (r *WireResolver) Resolve(target any) error {
	targetValue := reflect.ValueOf(target)
	if !isResolvableTarget(targetValue) {
		return fmt.Errorf("core: resolve %T: %w", target, ErrUnresolvable)
	}

	requestedType := targetValue.Elem().Type()
	component, err := r.lookup(requestedType)
	if err != nil {
		return fmt.Errorf("core: resolve %s: %w", requestedType, err)
	}

	targetValue.Elem().Set(reflect.ValueOf(component))
	return nil
}

// Graph returns a flat dependency graph for generated wiring.
func (r *WireResolver) Graph() DependencyGraph {
	r.mu.RLock()
	defer r.mu.RUnlock()

	graph := DependencyGraph{
		Nodes: make([]string, 0, len(r.registrationOrder)),
		Edges: make(map[string][]string, len(r.registrationOrder)),
	}
	for _, componentType := range r.registrationOrder {
		name := componentType.String()
		graph.Nodes = append(graph.Nodes, name)
		graph.Edges[name] = nil
	}
	return graph
}

// LifecycleCandidates returns registered components implementing Lifecycle.
func (r *WireResolver) LifecycleCandidates() ([]LifecycleCandidate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	candidates := make([]LifecycleCandidate, 0)
	for _, componentType := range r.registrationOrder {
		component := r.instances[componentType]
		instance, ok := component.(Lifecycle)
		if !ok {
			continue
		}
		candidates = append(candidates, LifecycleCandidate{
			Name:     componentType.String(),
			Instance: instance,
		})
	}
	return candidates, nil
}

func (r *WireResolver) lookup(requestedType reflect.Type) (any, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if component, ok := r.instances[requestedType]; ok {
		return component, nil
	}

	if requestedType.Kind() != reflect.Interface {
		return nil, ErrNotFound
	}

	var (
		match any
		found bool
	)
	for _, componentType := range r.registrationOrder {
		registration := r.registrations[componentType]
		if !registrationMatchesRequestedType(componentType, registration, requestedType) {
			continue
		}
		if found {
			return nil, fmt.Errorf("core: lookup %s: multiple registrations assignable: %w", requestedType, ErrUnresolvable)
		}
		match = r.instances[componentType]
		found = true
	}
	if !found {
		return nil, ErrNotFound
	}
	return match, nil
}

func (r *WireResolver) checkInterfaceConflicts(componentType reflect.Type, registration ComponentRegistration) error {
	for _, existingType := range r.registrationOrder {
		existingRegistration := r.registrations[existingType]

		for _, iface := range explicitRegistrationInterfaces(registration) {
			if registrationExposesInterface(existingType, existingRegistration, iface) {
				return fmt.Errorf("interface %s already provided by %s: %w", iface, existingType, ErrAlreadyRegistered)
			}
		}
		for _, iface := range explicitRegistrationInterfaces(existingRegistration) {
			if registrationExposesInterface(componentType, registration, iface) {
				return fmt.Errorf("interface %s already provided by %s: %w", iface, existingType, ErrAlreadyRegistered)
			}
		}
	}
	return nil
}

func explicitRegistrationInterfaces(registration ComponentRegistration) []reflect.Type {
	return registration.ResolveAs
}

func registrationExposesInterface(componentType reflect.Type, registration ComponentRegistration, iface reflect.Type) bool {
	return registrationMatchesRequestedType(componentType, registration, iface)
}
