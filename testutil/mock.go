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

	filtered := make([]any, 0, len(components))
	for _, component := range components {
		if isReplacedComponent(component, mocks) {
			continue
		}
		filtered = append(filtered, component)
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
	case reflect.Ptr, reflect.Map, reflect.Func, reflect.Chan, reflect.Slice:
		return true
	default:
		return false
	}
}

func isReplacedComponent(component any, mocks []mockBean) bool {
	componentType := reflect.TypeOf(component)
	if componentType == nil {
		return false
	}
	for _, mock := range mocks {
		if componentType.AssignableTo(mock.target) {
			return true
		}
	}
	return false
}

func isRegistrableMock(value reflect.Value) bool {
	if !value.IsValid() || value.Kind() != reflect.Ptr || value.IsNil() {
		return false
	}
	return value.Elem().Kind() == reflect.Struct
}
