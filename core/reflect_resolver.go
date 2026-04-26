package core

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"
)

// Compile-time check that ReflectResolver satisfies the Resolver interface.
var _ Resolver = (*ReflectResolver)(nil)

// Compile-time check that ReflectResolver satisfies the LifecycleResolver interface.
var _ LifecycleResolver = (*ReflectResolver)(nil)

// ReflectResolver resolves dependencies using Go reflection at runtime.
// It is the default Helix resolver mode and stores singleton instances by type.
type ReflectResolver struct {
	mu                sync.RWMutex
	registrations     map[reflect.Type]ComponentRegistration
	registrationOrder []reflect.Type
	singletons        map[reflect.Type]reflect.Value
	graph             DependencyGraph
	valueLookup       func(key string) (any, bool)
}

type resolutionState struct {
	stack     []reflect.Type
	positions map[reflect.Type]int
}

// NewReflectResolver creates a reflection-based resolver with initialized maps.
func NewReflectResolver() *ReflectResolver {
	return &ReflectResolver{
		registrations: make(map[reflect.Type]ComponentRegistration),
		singletons:    make(map[reflect.Type]reflect.Value),
		graph: DependencyGraph{
			Edges: make(map[string][]string),
		},
		valueLookup: func(string) (any, bool) {
			return nil, false
		},
	}
}

func (r *ReflectResolver) setValueLookup(lookup func(key string) (any, bool)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.valueLookup = lookup
}

// Register stores a component registration keyed by its concrete pointer type.
func (r *ReflectResolver) Register(component any) error {
	registration, componentType, err := normalizeComponentRegistration(component)
	if err != nil {
		return fmt.Errorf("core: register %T: %w", component, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, exists := r.registrations[componentType]
	r.registrations[componentType] = registration
	delete(r.singletons, componentType)

	typeName := componentType.String()
	if !exists {
		r.registrationOrder = append(r.registrationOrder, componentType)
		r.graph.Nodes = append(r.graph.Nodes, typeName)
	}
	r.graph.Edges[typeName] = nil

	return nil
}

// Resolve finds the registered component matching target's element type.
func (r *ReflectResolver) Resolve(target any) error {
	targetValue := reflect.ValueOf(target)
	if !isResolvableTarget(targetValue) {
		return fmt.Errorf("core: resolve %T: %w", target, ErrUnresolvable)
	}

	requestedType := targetValue.Elem().Type()

	// Fast path: RLock for cached singletons.
	r.mu.RLock()
	if val, ok := r.singletons[requestedType]; ok {
		targetValue.Elem().Set(val)
		r.mu.RUnlock()
		return nil
	}
	r.mu.RUnlock()

	// Slow path: Lock for materialization.
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check after acquiring write lock.
	if val, ok := r.singletons[requestedType]; ok {
		targetValue.Elem().Set(val)
		return nil
	}

	resolvedValue, err := r.resolveByType(requestedType)
	if err != nil {
		return fmt.Errorf("core: resolve %s: %w", requestedType, err)
	}

	targetValue.Elem().Set(resolvedValue)
	return nil
}

// Graph returns a defensive copy of the current dependency graph.
func (r *ReflectResolver) Graph() DependencyGraph {
	r.mu.RLock()
	defer r.mu.RUnlock()

	graph := DependencyGraph{
		Nodes: append([]string(nil), r.graph.Nodes...),
		Edges: make(map[string][]string, len(r.graph.Edges)),
	}
	for node, deps := range r.graph.Edges {
		graph.Edges[node] = append([]string(nil), deps...)
	}
	return graph
}

// LifecycleCandidates returns all non-lazy singleton components implementing
// Lifecycle, in registration order. Uses reflect.Type as the seen-set key to
// prevent collisions between types that share a short string representation.
func (r *ReflectResolver) LifecycleCandidates() ([]LifecycleCandidate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := make(map[reflect.Type]struct{}, len(r.registrationOrder))
	var candidates []LifecycleCandidate

	for _, registrationType := range r.registrationOrder {
		registration := r.registrations[registrationType]
		if registration.Scope != ScopeSingleton || registration.Lazy {
			continue
		}
		if !registrationType.Implements(lifecycleType) {
			continue
		}
		if _, dup := seen[registrationType]; dup {
			continue
		}
		seen[registrationType] = struct{}{}

		value, err := r.resolveRegistration(registrationType, registration, newResolutionState())
		if err != nil {
			return nil, fmt.Errorf("core: resolve lifecycle %s: %w", registrationType, err)
		}

		instance, ok := value.Interface().(Lifecycle)
		if !ok {
			continue
		}

		candidates = append(candidates, LifecycleCandidate{
			Name:     registrationType.String(),
			Instance: instance,
		})
	}

	return candidates, nil
}

func (r *ReflectResolver) resolveByType(requestedType reflect.Type) (reflect.Value, error) {
	state := newResolutionState()
	return r.resolveByTypeWithState(requestedType, state)
}
func (r *ReflectResolver) resolveAllAssignable(targetType reflect.Type) ([]reflect.Value, error) {
	if targetType == nil || (targetType.Kind() != reflect.Interface && targetType.Kind() != reflect.Ptr) {
		return nil, ErrUnresolvable
	}

	r.mu.RLock()
	order := append([]reflect.Type(nil), r.registrationOrder...)
	r.mu.RUnlock()

	values := make([]reflect.Value, 0)
	for _, registrationType := range order {
		if !registrationType.AssignableTo(targetType) {
			continue
		}

		r.mu.Lock()
		registration := r.registrations[registrationType]
		val, err := r.resolveRegistration(registrationType, registration, newResolutionState())
		r.mu.Unlock()

		if err != nil {
			return nil, fmt.Errorf("core: resolve all %s: %w", targetType, err)
		}
		values = append(values, val)
	}

	return values, nil
}

func (r *ReflectResolver) resolveByTypeWithState(requestedType reflect.Type, state *resolutionState) (reflect.Value, error) {
	registrationType, registration, err := r.lookupRegistration(requestedType)
	if err != nil {
		return reflect.Value{}, err
	}

	return r.resolveRegistration(registrationType, registration, state)
}

func (r *ReflectResolver) resolveRegistration(registrationType reflect.Type, registration ComponentRegistration, state *resolutionState) (reflect.Value, error) {
	if registration.Scope == ScopeSingleton {
		if singleton, ok := r.singletons[registrationType]; ok {
			return singleton, nil
		}
	}
	if cyclePath, ok := state.detectCycle(registrationType); ok {
		return reflect.Value{}, &CyclicDepError{Path: cyclePath}
	}

	instance := materializeRegistrationInstance(registrationType, registration)
	if !isRegistrableComponent(instance) {
		return reflect.Value{}, ErrUnresolvable
	}

	state.push(registrationType)
	defer state.pop()

	if err := r.injectFields(registrationType, instance, state); err != nil {
		return reflect.Value{}, err
	}

	if registration.Scope == ScopeSingleton {
		r.singletons[registrationType] = instance
	}

	return instance, nil
}

func materializeRegistrationInstance(registrationType reflect.Type, registration ComponentRegistration) reflect.Value {
	if registration.Scope != ScopePrototype {
		return reflect.ValueOf(registration.Component)
	}
	// Prototype: allocate a fresh zero-value instance; inject:"true" and value:"..."
	// fields are populated by the caller via injectFields.
	return reflect.New(registrationType.Elem())
}

func (r *ReflectResolver) injectFields(ownerType reflect.Type, instance reflect.Value, state *resolutionState) error {
	structValue := instance.Elem()
	structType := structValue.Type()

	for i := 0; i < structValue.NumField(); i++ {
		fieldValue := structValue.Field(i)
		fieldType := structType.Field(i)

		if fieldType.Tag.Get("inject") == "true" {
			if !fieldType.IsExported() || !fieldValue.CanSet() {
				return fmt.Errorf("core: resolve %s field %s: %w", ownerType, fieldType.Name, ErrUnresolvable)
			}

			dependencyValue, dependencyType, err := r.resolveFieldDependency(fieldType.Type, state)
			if err != nil {
				return fmt.Errorf("core: resolve %s field %s: %w", ownerType, fieldType.Name, err)
			}
			if dependencyType != nil {
				r.appendGraphEdge(ownerType.String(), dependencyType.String())
			}

			fieldValue.Set(dependencyValue)
		} else if valueKey := fieldType.Tag.Get("value"); valueKey != "" {
			if !fieldType.IsExported() || !fieldValue.CanSet() {
				return fmt.Errorf("core: resolve %s field %s: %w", ownerType, fieldType.Name, ErrUnresolvable)
			}

			// Release lock during external callback to prevent deadlocks.
			r.mu.Unlock()
			rawValue, ok := r.valueLookup(valueKey)
			r.mu.Lock()

			if !ok {
				return fmt.Errorf("core: resolve %s field %s value %q: %w", ownerType, fieldType.Name, valueKey, ErrNotFound)
			}

			convertedValue, err := convertScalarValue(rawValue, fieldType.Type)
			if err != nil {
				return fmt.Errorf("core: resolve %s field %s value %q: %w", ownerType, fieldType.Name, valueKey, err)
			}

			fieldValue.Set(convertedValue)
		}
	}

	return nil
}

func (r *ReflectResolver) resolveFieldDependency(fieldType reflect.Type, state *resolutionState) (reflect.Value, reflect.Type, error) {
	registrationType, registration, err := r.lookupRegistration(fieldType)
	if err != nil {
		return reflect.Value{}, nil, err
	}

	value, err := r.resolveRegistration(registrationType, registration, state)
	if err != nil {
		return reflect.Value{}, registrationType, err
	}

	return value, registrationType, nil
}

func (r *ReflectResolver) lookupRegistration(requestedType reflect.Type) (reflect.Type, ComponentRegistration, error) {
	if registration, ok := r.registrations[requestedType]; ok {
		return requestedType, registration, nil
	}

	if requestedType.Kind() != reflect.Interface {
		return nil, ComponentRegistration{}, ErrNotFound
	}

	var (
		matchType reflect.Type
		match     ComponentRegistration
		found     bool
	)

	for _, registeredType := range r.registrationOrder {
		if !registeredType.AssignableTo(requestedType) {
			continue
		}
		if found {
			return nil, ComponentRegistration{}, fmt.Errorf("core: lookup %s: multiple registrations assignable: %w", requestedType, ErrUnresolvable)
		}
		matchType = registeredType
		match = r.registrations[registeredType]
		found = true
	}

	if !found {
		return nil, ComponentRegistration{}, ErrNotFound
	}

	return matchType, match, nil
}

func (r *ReflectResolver) appendGraphEdge(owner, dependency string) {
	dependencies := r.graph.Edges[owner]
	for _, existing := range dependencies {
		if existing == dependency {
			return
		}
	}
	r.graph.Edges[owner] = append(dependencies, dependency)
}

func isRegistrableComponent(value reflect.Value) bool {
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return false
	}
	return value.Elem().Kind() == reflect.Struct
}

func isResolvableTarget(value reflect.Value) bool {
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return false
	}
	return value.Elem().CanSet()
}

func convertScalarValue(value any, targetType reflect.Type) (reflect.Value, error) {
	input := reflect.ValueOf(value)
	if input.IsValid() && input.Type().AssignableTo(targetType) {
		return input, nil
	}
	if input.IsValid() && input.Type().ConvertibleTo(targetType) && isDirectlyConvertibleKind(targetType.Kind()) {
		return input.Convert(targetType), nil
	}

	switch targetType.Kind() {
	case reflect.String:
		if text, ok := value.(string); ok {
			return reflect.ValueOf(text).Convert(targetType), nil
		}
	case reflect.Bool:
		switch typed := value.(type) {
		case bool:
			return reflect.ValueOf(typed).Convert(targetType), nil
		case string:
			parsed, err := strconv.ParseBool(typed)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("core: convert %T to %s: %w", value, targetType, ErrUnresolvable)
			}
			return reflect.ValueOf(parsed).Convert(targetType), nil
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch typed := value.(type) {
		case string:
			parsed, err := strconv.ParseInt(typed, 10, targetType.Bits())
			if err != nil {
				return reflect.Value{}, fmt.Errorf("core: convert %T to %s: %w", value, targetType, ErrUnresolvable)
			}
			converted := reflect.New(targetType).Elem()
			converted.SetInt(parsed)
			return converted, nil
		}
	}

	return reflect.Value{}, fmt.Errorf("core: convert %T to %s: %w", value, targetType, ErrUnresolvable)
}

func isDirectlyConvertibleKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}

func newResolutionState() *resolutionState {
	return &resolutionState{
		positions: make(map[reflect.Type]int),
	}
}

func (s *resolutionState) push(registrationType reflect.Type) {
	s.positions[registrationType] = len(s.stack)
	s.stack = append(s.stack, registrationType)
}

func (s *resolutionState) pop() {
	if len(s.stack) == 0 {
		return
	}

	lastIndex := len(s.stack) - 1
	lastType := s.stack[lastIndex]
	s.stack = s.stack[:lastIndex]
	delete(s.positions, lastType)
}

func (s *resolutionState) detectCycle(registrationType reflect.Type) ([]string, bool) {
	start, ok := s.positions[registrationType]
	if !ok {
		return nil, false
	}

	path := make([]string, 0, len(s.stack)-start+1)
	for _, step := range s.stack[start:] {
		path = append(path, step.String())
	}
	path = append(path, registrationType.String())

	return path, true
}
