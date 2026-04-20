package observability

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/enokdev/helix/core"
)

type testHealthIndicator struct {
	name   string
	health ComponentHealth
}

func (i *testHealthIndicator) Name() string {
	return i.name
}

func (i *testHealthIndicator) Health(context.Context) ComponentHealth {
	return i.health
}

type nilHealthIndicator struct{}

func (*nilHealthIndicator) Name() string {
	return "nil"
}

func (*nilHealthIndicator) Health(context.Context) ComponentHealth {
	return ComponentHealth{Status: StatusUp}
}

func TestCompositeHealthCheckerCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		indicators []HealthIndicator
		want       HealthResponse
	}{
		{
			name: "no indicators is up without components",
			want: HealthResponse{Status: StatusUp},
		},
		{
			name: "up indicators without details omit components",
			indicators: []HealthIndicator{
				&testHealthIndicator{name: "db", health: ComponentHealth{Status: StatusUp}},
			},
			want: HealthResponse{Status: StatusUp},
		},
		{
			name: "up indicators with details include components",
			indicators: []HealthIndicator{
				&testHealthIndicator{name: "db", health: ComponentHealth{Details: map[string]any{"latency": "1ms"}}},
			},
			want: HealthResponse{
				Status: StatusUp,
				Components: map[string]ComponentHealth{
					"db": {Status: StatusUp, Details: map[string]any{"latency": "1ms"}},
				},
			},
		},
		{
			name: "down indicator marks global down",
			indicators: []HealthIndicator{
				&testHealthIndicator{name: "db", health: ComponentHealth{Status: StatusDown, Error: "connection refused"}},
			},
			want: HealthResponse{
				Status: StatusDown,
				Components: map[string]ComponentHealth{
					"db": {Status: StatusDown, Error: "connection refused"},
				},
			},
		},
		{
			name: "error without status marks component down",
			indicators: []HealthIndicator{
				&testHealthIndicator{name: "cache", health: ComponentHealth{Error: "timeout"}},
			},
			want: HealthResponse{
				Status: StatusDown,
				Components: map[string]ComponentHealth{
					"cache": {Status: StatusDown, Error: "timeout"},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker, err := NewCompositeHealthChecker(tt.indicators...)
			if err != nil {
				t.Fatalf("NewCompositeHealthChecker() error = %v", err)
			}

			got := checker.Check(context.Background())
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Check() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNewCompositeHealthCheckerRejectsInvalidIndicators(t *testing.T) {
	t.Parallel()

	var typedNil *nilHealthIndicator
	tests := []struct {
		name       string
		indicators []HealthIndicator
	}{
		{name: "nil interface", indicators: []HealthIndicator{nil}},
		{name: "typed nil", indicators: []HealthIndicator{typedNil}},
		{name: "empty name", indicators: []HealthIndicator{&testHealthIndicator{}}},
		{name: "blank name", indicators: []HealthIndicator{&testHealthIndicator{name: "   "}}},
		{
			name: "duplicate name",
			indicators: []HealthIndicator{
				&testHealthIndicator{name: "db"},
				&testHealthIndicator{name: "db"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewCompositeHealthChecker(tt.indicators...)
			if !errors.Is(err, ErrInvalidHealthIndicator) {
				t.Fatalf("NewCompositeHealthChecker() error = %v, want ErrInvalidHealthIndicator", err)
			}
		})
	}
}

func TestHealthCheckerFromContainerIncludesRegisteredIndicators(t *testing.T) {
	t.Parallel()

	container := core.NewContainer(core.WithResolver(core.NewReflectResolver()))
	if err := container.Register(&containerHealthIndicator{name: "db"}); err != nil {
		t.Fatalf("Register(db) error = %v", err)
	}
	if err := container.Register(&containerHealthIndicator{name: "cache", health: ComponentHealth{Details: map[string]any{"kind": "memory"}}}); err != nil {
		t.Fatalf("Register(cache) error = %v", err)
	}

	checker, err := HealthCheckerFromContainer(container)
	if err != nil {
		t.Fatalf("HealthCheckerFromContainer() error = %v", err)
	}

	got := checker.Check(context.Background())
	want := HealthResponse{
		Status: StatusUp,
		Components: map[string]ComponentHealth{
			"cache": {Status: StatusUp, Details: map[string]any{"kind": "memory"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Check() = %#v, want %#v", got, want)
	}
}

func TestHealthCheckerFromContainerIncludesDownIndicator(t *testing.T) {
	t.Parallel()

	container := core.NewContainer(core.WithResolver(core.NewReflectResolver()))
	if err := container.Register(&containerHealthIndicator{
		name:   "db",
		health: ComponentHealth{Status: StatusDown, Error: "connection refused"},
	}); err != nil {
		t.Fatalf("Register(db) error = %v", err)
	}

	checker, err := HealthCheckerFromContainer(container)
	if err != nil {
		t.Fatalf("HealthCheckerFromContainer() error = %v", err)
	}

	got := checker.Check(context.Background())
	if got.Status != StatusDown {
		t.Fatalf("Check() status = %q, want %q", got.Status, StatusDown)
	}
	if got.Components["db"].Error != "connection refused" {
		t.Fatalf("Check() db error = %q, want \"connection refused\"", got.Components["db"].Error)
	}
}

func TestHealthCheckerFromContainerWrapsResolveErrors(t *testing.T) {
	t.Parallel()

	container := core.NewContainer(core.WithResolver(core.NewReflectResolver()))
	if err := container.Register(&unresolvableContainerHealthIndicator{}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	_, err := HealthCheckerFromContainer(container)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("HealthCheckerFromContainer() error = %v, want core.ErrNotFound", err)
	}
}

type containerHealthIndicator struct {
	name   string
	health ComponentHealth
}

func (i *containerHealthIndicator) Name() string {
	return i.name
}

func (i *containerHealthIndicator) Health(context.Context) ComponentHealth {
	return i.health
}

type unresolvableContainerHealthIndicator struct {
	Dependency *missingHealthDependency `inject:"true"`
}

func (i *unresolvableContainerHealthIndicator) Name() string {
	return "broken"
}

func (i *unresolvableContainerHealthIndicator) Health(context.Context) ComponentHealth {
	return ComponentHealth{Status: StatusUp}
}

type missingHealthDependency struct{}
