package core

import (
	"fmt"
	"reflect"
)

type assignableResolver interface {
	resolveAllAssignable(targetType reflect.Type) ([]reflect.Value, error)
}

// ResolveAll resolves every registered component assignable to T.
//
// Resolver implementations may opt into this capability without expanding the
// base Resolver interface used by both reflection and generated modes.
func ResolveAll[T any](container *Container) ([]T, error) {
	if container == nil {
		return nil, fmt.Errorf("core: resolve all: %w", ErrUnresolvable)
	}

	container.resolverMu.RLock()
	resolver := container.resolver
	container.resolverMu.RUnlock()

	if resolver == nil {
		return nil, fmt.Errorf("core: resolve all: %w", ErrUnresolvable)
	}

	assignable, ok := resolver.(assignableResolver)
	if !ok {
		return nil, fmt.Errorf("core: resolve all: %w", ErrUnresolvable)
	}

	targetType := reflect.TypeOf((*T)(nil)).Elem()
	values, err := assignable.resolveAllAssignable(targetType)
	if err != nil {
		return nil, fmt.Errorf("core: resolve all %s: %w", targetType, err)
	}

	resolved := make([]T, 0, len(values))
	for _, value := range values {
		v, ok := value.Interface().(T)
		if !ok {
			return nil, fmt.Errorf("core: resolve all %s: unexpected value type %T: %w", targetType, value.Interface(), ErrUnresolvable)
		}
		resolved = append(resolved, v)
	}
	return resolved, nil
}
