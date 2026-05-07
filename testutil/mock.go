package testutil

import (
	"fmt"
	"reflect"

	"github.com/enokdev/helix/core"
)

type mockBean struct {
	target reflect.Type
	impl   any
}

// MockBean replaces components assignable to T with impl in a Helix test app.
func MockBean[T any](impl T) Option {
	target := reflect.TypeOf((*T)(nil)).Elem()
	return func(opts *appOptions) {
		opts.mockBeans = append(opts.mockBeans, mockBean{
			target: target,
			impl:   impl,
		})
	}
}

func prepareTestComponents(components []any, mocks []mockBean) ([]any, []mockBean, error) {
	if err := validateMockBeans(mocks); err != nil {
		return nil, nil, err
	}

	injectedInterfaces := collectInjectedInterfaces(components)
	filtered := make([]any, 0, len(components))
	for _, component := range components {
		if !isReplacedComponent(component, mocks) {
			filtered = append(filtered, component)
			continue
		}
		replacedTargets := replacedTargetsForComponent(component, mocks)
		usedAtStartup := hasNonTargetInterfaceUse(component, replacedTargets, injectedInterfaces)
		filtered = append(filtered, componentWithExcludedTargets(component, replacedTargets, usedAtStartup))
	}

	return filtered, append([]mockBean(nil), mocks...), nil
}

func validateMockBeans(mocks []mockBean) error {
	seen := make(map[reflect.Type]struct{}, len(mocks))
	for _, mock := range mocks {
		if mock.target == nil {
			return fmt.Errorf("testutil: mock bean: missing target type: %w", core.ErrUnresolvable)
		}
		if mock.target.Kind() != reflect.Interface {
			return fmt.Errorf("testutil: mock bean %s: target must be an interface: %w", mock.target, core.ErrUnresolvable)
		}
		if mock.target.NumMethod() == 0 {
			return fmt.Errorf("testutil: mock bean %s: empty interface target: %w", mock.target, core.ErrUnresolvable)
		}
		if _, ok := seen[mock.target]; ok {
			return fmt.Errorf("testutil: mock bean %s: duplicate target: %w", mock.target, core.ErrUnresolvable)
		}
		seen[mock.target] = struct{}{}

		mockValue := reflect.ValueOf(mock.impl)
		isNilable := mockValue.IsValid() && isNilableKind(mockValue.Kind())
		if !mockValue.IsValid() || (isNilable && mockValue.IsNil()) {
			return fmt.Errorf("testutil: mock bean %s: nil implementation: %w", mock.target, core.ErrUnresolvable)
		}
		mockType := reflect.TypeOf(mock.impl)
		if !mockType.AssignableTo(mock.target) {
			return fmt.Errorf("testutil: mock bean %s: implementation %s is not assignable: %w", mock.target, mockType, core.ErrUnresolvable)
		}
		if !isRegistrableMock(mockValue) {
			return fmt.Errorf("testutil: mock bean %s: implementation must be a non-nil pointer to struct: %w", mock.target, core.ErrUnresolvable)
		}
	}
	return nil
}

func isNilableKind(k reflect.Kind) bool {
	switch k {
	case reflect.Pointer, reflect.Map, reflect.Func, reflect.Chan, reflect.Slice:
		return true
	default:
		return false
	}
}

func isReplacedComponent(component any, mocks []mockBean) bool {
	return len(replacedTargetsForComponent(component, mocks)) > 0
}

func replacedTargetsForComponent(component any, mocks []mockBean) []reflect.Type {
	componentType := componentRegistrationType(component)
	if componentType == nil {
		return nil
	}
	replaced := make([]reflect.Type, 0, len(mocks))
	for _, mock := range mocks {
		if componentType.AssignableTo(mock.target) {
			replaced = append(replaced, mock.target)
		}
	}
	return replaced
}

func isRegistrableMock(value reflect.Value) bool {
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return false
	}
	return value.Elem().Kind() == reflect.Struct
}

func collectInjectedInterfaces(components []any) []reflect.Type {
	seen := make(map[reflect.Type]struct{})
	for _, component := range components {
		componentType := componentRegistrationType(component)
		if componentType == nil || componentType.Kind() != reflect.Pointer || componentType.Elem().Kind() != reflect.Struct {
			continue
		}
		structType := componentType.Elem()
		for i := 0; i < structType.NumField(); i++ {
			field := structType.Field(i)
			if field.Tag.Get("inject") != "true" || field.Type.Kind() != reflect.Interface {
				continue
			}
			seen[field.Type] = struct{}{}
		}
	}

	interfaces := make([]reflect.Type, 0, len(seen))
	for typ := range seen {
		interfaces = append(interfaces, typ)
	}
	return interfaces
}

func hasNonTargetInterfaceUse(component any, replacedTargets, injectedInterfaces []reflect.Type) bool {
	componentType := componentRegistrationType(component)
	if componentType == nil {
		return false
	}
	if !containsReflectType(replacedTargets, lifecycleInterfaceType()) && componentType.Implements(lifecycleInterfaceType()) {
		return true
	}
	for _, injected := range injectedInterfaces {
		if containsReflectType(replacedTargets, injected) {
			continue
		}
		if componentType.AssignableTo(injected) {
			return true
		}
	}
	return false
}

func lifecycleInterfaceType() reflect.Type {
	return reflect.TypeOf((*core.Lifecycle)(nil)).Elem()
}

func componentWithExcludedTargets(component any, targets []reflect.Type, usedAtStartup bool) any {
	registration, ok := component.(core.ComponentRegistration)
	if !ok {
		registration = core.NewComponentRegistration(component)
	}
	registration.ExcludeFrom = appendUniqueReflectTypes(registration.ExcludeFrom, targets...)
	if !usedAtStartup && registration.Scope == core.ScopeSingleton {
		registration.Lazy = true
	}
	return registration
}

func componentRegistrationType(component any) reflect.Type {
	if registration, ok := component.(core.ComponentRegistration); ok {
		return reflect.TypeOf(registration.Component)
	}
	return reflect.TypeOf(component)
}

func appendUniqueReflectTypes(types []reflect.Type, additions ...reflect.Type) []reflect.Type {
	for _, addition := range additions {
		if containsReflectType(types, addition) {
			continue
		}
		types = append(types, addition)
	}
	return types
}

func containsReflectType(types []reflect.Type, target reflect.Type) bool {
	for _, typ := range types {
		if typ == target {
			return true
		}
	}
	return false
}

func mockRegistration(mock mockBean) core.ComponentRegistration {
	registration := core.NewComponentRegistration(mock.impl)
	registration.ResolveAs = []reflect.Type{mock.target}
	return registration
}
