package core

import (
	"errors"
	"reflect"
	"testing"
)

type resolveAllService interface {
	ID() string
}

type resolveAllFirst struct{}

func (s *resolveAllFirst) ID() string {
	return "first"
}

type resolveAllSecond struct {
	Dependency *resolveAllDependency `inject:"true"`
}

func (s *resolveAllSecond) ID() string {
	if s.Dependency == nil {
		return "missing"
	}
	return "second:" + s.Dependency.Value
}

type resolveAllDependency struct {
	Value string
}

type resolveAllMissingDependency struct {
	Dependency *resolveAllDependency `inject:"true"`
}

func (s *resolveAllMissingDependency) ID() string {
	return "missing-dependency"
}

func TestResolveAllReturnsAssignableComponentsInRegistrationOrder(t *testing.T) {
	t.Parallel()

	container := NewContainer(WithResolver(NewReflectResolver()))
	if err := container.Register(&resolveAllFirst{}); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	if err := container.Register(&resolveAllDependency{Value: "ready"}); err != nil {
		t.Fatalf("Register(dependency) error = %v", err)
	}
	if err := container.Register(&resolveAllSecond{}); err != nil {
		t.Fatalf("Register(second) error = %v", err)
	}

	services, err := ResolveAll[resolveAllService](container)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	if got, want := len(services), 2; got != want {
		t.Fatalf("len(services) = %d, want %d", got, want)
	}
	if services[0].ID() != "first" {
		t.Fatalf("services[0].ID() = %q, want first", services[0].ID())
	}
	if services[1].ID() != "second:ready" {
		t.Fatalf("services[1].ID() = %q, want second:ready", services[1].ID())
	}
}

func TestResolveAllReturnsEmptySliceWhenNoComponentMatches(t *testing.T) {
	t.Parallel()

	container := NewContainer(WithResolver(NewReflectResolver()))
	if err := container.Register(&resolveAllDependency{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	services, err := ResolveAll[resolveAllService](container)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}
	if services == nil {
		t.Fatal("ResolveAll() returned nil slice, want empty slice")
	}
	if len(services) != 0 {
		t.Fatalf("len(services) = %d, want 0", len(services))
	}
}

func TestResolveAllErrorsWhenMatchingComponentCannotResolve(t *testing.T) {
	t.Parallel()

	container := NewContainer(WithResolver(NewReflectResolver()))
	if err := container.Register(&resolveAllMissingDependency{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err := ResolveAll[resolveAllService](container)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ResolveAll() error = %v, want ErrNotFound", err)
	}
}

func TestResolveAllRejectsInvalidContainer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		container *Container
	}{
		{name: "nil container", container: nil},
		{name: "missing resolver", container: NewContainer()},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ResolveAll[resolveAllService](tt.container)
			if !errors.Is(err, ErrUnresolvable) {
				t.Fatalf("ResolveAll() error = %v, want ErrUnresolvable", err)
			}
		})
	}
}

func TestReflectResolverResolveAllAssignableRejectsInvalidTargetType(t *testing.T) {
	t.Parallel()

	_, err := NewReflectResolver().resolveAllAssignable(reflect.TypeOf(0))
	if !errors.Is(err, ErrUnresolvable) {
		t.Fatalf("resolveAllAssignable() error = %v, want ErrUnresolvable", err)
	}
}
