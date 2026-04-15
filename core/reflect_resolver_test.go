package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type testDependency struct {
	Name string
}

type testService struct {
	Dependency *testDependency `inject:"true"`
}

type greeter interface {
	Greet() string
}

type greeterImpl struct{}

func (g *greeterImpl) Greet() string {
	return "hello"
}

type greeterImplAlt struct{}

func (g *greeterImplAlt) Greet() string {
	return "hello-alt"
}

type interfaceConsumer struct {
	Greeter greeter `inject:"true"`
}

type invalidInjectConsumer struct {
	dependency *testDependency `inject:"true"` //nolint:unused // unexported field tests inject rejection
}

type valueConsumer struct {
	Port    int    `value:"server.port"`
	Name    string `value:"app.name"`
	Enabled bool   `value:"feature.enabled"`
}

type prototypeService struct {
	Dependency *testDependency `inject:"true"`
	Label      string
}

type lazyConsumer struct {
	Dependency *testDependency `inject:"true"`
}

type cycleServiceA struct {
	ServiceB *cycleServiceB `inject:"true"`
}

type cycleServiceB struct {
	ServiceA *cycleServiceA `inject:"true"`
}

type longCycleServiceA struct {
	ServiceB *longCycleServiceB `inject:"true"`
}

type longCycleServiceB struct {
	ServiceC *longCycleServiceC `inject:"true"`
}

type longCycleServiceC struct {
	ServiceA *longCycleServiceA `inject:"true"`
}

func TestNewReflectResolver(t *testing.T) {
	resolver := NewReflectResolver()
	if resolver == nil {
		t.Fatal("NewReflectResolver() returned nil")
	}
	if resolver.registrations == nil {
		t.Fatal("registrations map should be initialized")
	}
	if resolver.singletons == nil {
		t.Fatal("singletons map should be initialized")
	}
	if resolver.graph.Edges == nil {
		t.Fatal("graph edges map should be initialized")
	}
	if resolver.valueLookup == nil {
		t.Fatal("valueLookup should be initialized")
	}

	graph := resolver.Graph()
	graph.Nodes = append(graph.Nodes, "mutated")
	graph.Edges["mutated"] = []string{"dependency"}

	freshGraph := resolver.Graph()
	if len(freshGraph.Nodes) != 0 {
		t.Fatalf("Graph() should return a defensive copy of nodes, got %v", freshGraph.Nodes)
	}
	if len(freshGraph.Edges) != 0 {
		t.Fatalf("Graph() should return a defensive copy of edges, got %v", freshGraph.Edges)
	}
}

func TestReflectResolver_Register(t *testing.T) {
	type nonStruct int

	tests := []struct {
		name      string
		component any
		wantErr   error
	}{
		{
			name:      "nil component",
			component: nil,
			wantErr:   ErrUnresolvable,
		},
		{
			name:      "typed nil pointer",
			component: (*testDependency)(nil),
			wantErr:   ErrUnresolvable,
		},
		{
			name:      "non pointer component",
			component: testDependency{},
			wantErr:   ErrUnresolvable,
		},
		{
			name:      "pointer to non struct",
			component: new(nonStruct),
			wantErr:   ErrUnresolvable,
		},
		{
			name:      "valid pointer to struct",
			component: &testDependency{Name: "registered"},
			wantErr:   nil,
		},
		{
			name: "valid component registration metadata",
			component: ComponentRegistration{
				Component: &testDependency{Name: "registered"},
				Scope:     ScopePrototype,
				Lazy:      true,
			},
			wantErr: nil,
		},
		{
			name: "component registration with invalid scope",
			component: ComponentRegistration{
				Component: &testDependency{Name: "registered"},
				Scope:     Scope("invalid"),
			},
			wantErr: ErrUnresolvable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewReflectResolver()
			err := resolver.Register(tt.component)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr != nil {
				return
			}

			componentType := registeredComponentType(tt.component)
			registration, ok := resolver.registrations[componentType]
			if !ok {
				t.Fatalf("registration for %v was not stored", componentType)
			}
			expectedScope := ScopeSingleton
			expectedLazy := false
			if registrationInput, ok := tt.component.(ComponentRegistration); ok {
				expectedScope = registrationInput.Scope
				expectedLazy = registrationInput.Lazy
			}
			if registration.Scope != expectedScope {
				t.Fatalf("registration scope = %q, want %q", registration.Scope, expectedScope)
			}
			if registration.Lazy != expectedLazy {
				t.Fatalf("registration lazy = %t, want %t", registration.Lazy, expectedLazy)
			}

			graph := resolver.Graph()
			if len(graph.Nodes) != 1 || graph.Nodes[0] != componentType.String() {
				t.Fatalf("Graph().Nodes = %v, want [%s]", graph.Nodes, componentType.String())
			}
		})
	}
}

func TestReflectResolver_Resolve(t *testing.T) {
	t.Run("invalid targets return ErrUnresolvable", func(t *testing.T) {
		resolver := NewReflectResolver()

		tests := []struct {
			name   string
			target any
		}{
			{name: "nil target", target: nil},
			{name: "non pointer target", target: testDependency{}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := resolver.Resolve(tt.target)
				if !errors.Is(err, ErrUnresolvable) {
					t.Fatalf("Resolve() error = %v, want %v", err, ErrUnresolvable)
				}
			})
		}
	})

	t.Run("missing registration returns ErrNotFound", func(t *testing.T) {
		resolver := NewReflectResolver()

		var dependency *testDependency
		err := resolver.Resolve(&dependency)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Resolve() error = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("singleton resolution returns same instance", func(t *testing.T) {
		resolver := NewReflectResolver()
		component := &testDependency{Name: "singleton"}
		if err := resolver.Register(component); err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		var first *testDependency
		if err := resolver.Resolve(&first); err != nil {
			t.Fatalf("Resolve(first) error = %v", err)
		}

		var second *testDependency
		if err := resolver.Resolve(&second); err != nil {
			t.Fatalf("Resolve(second) error = %v", err)
		}

		if first != second {
			t.Fatalf("Resolve() should return the same singleton instance, got %p and %p", first, second)
		}
		if first != component {
			t.Fatalf("Resolve() should reuse the registered component instance, got %p and %p", first, component)
		}
	})

	t.Run("container delegates to reflect resolver", func(t *testing.T) {
		container := NewContainer(WithResolver(NewReflectResolver()))
		component := &testDependency{Name: "container"}
		if err := container.Register(component); err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		var resolved *testDependency
		if err := container.Resolve(&resolved); err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}

		if resolved != component {
			t.Fatalf("resolved component = %p, want %p", resolved, component)
		}
	})

	t.Run("prototype resolution returns fresh instances and does not cache them", func(t *testing.T) {
		resolver := NewReflectResolver()
		registration := ComponentRegistration{
			Component: &prototypeService{Label: "prototype"},
			Scope:     ScopePrototype,
		}
		if err := resolver.Register(&testDependency{Name: "shared"}); err != nil {
			t.Fatalf("Register(testDependency) error = %v", err)
		}
		if err := resolver.Register(registration); err != nil {
			t.Fatalf("Register(prototypeService) error = %v", err)
		}

		var first *prototypeService
		if err := resolver.Resolve(&first); err != nil {
			t.Fatalf("Resolve(first) error = %v", err)
		}

		var second *prototypeService
		if err := resolver.Resolve(&second); err != nil {
			t.Fatalf("Resolve(second) error = %v", err)
		}

		if first == second {
			t.Fatal("Resolve() should return a fresh prototype instance on each call")
		}
		if first.Label != "" || second.Label != "" {
			t.Fatal("prototype non-inject fields should be zero-valued, not copied from the registered source")
		}
		if first.Dependency == nil || second.Dependency == nil {
			t.Fatal("prototype dependencies should be injected on each resolution")
		}
		if first.Dependency != second.Dependency {
			t.Fatal("prototype should still reuse registered singleton dependencies")
		}
		if len(resolver.singletons) != 1 {
			t.Fatalf("singletons should only contain singleton dependencies, got %d entries", len(resolver.singletons))
		}
	})

	t.Run("lazy registration is not cached before first resolve", func(t *testing.T) {
		resolver := NewReflectResolver()
		registration := ComponentRegistration{
			Component: &lazyConsumer{},
			Lazy:      true,
		}
		if err := resolver.Register(&testDependency{Name: "lazy"}); err != nil {
			t.Fatalf("Register(testDependency) error = %v", err)
		}
		if err := resolver.Register(registration); err != nil {
			t.Fatalf("Register(lazyConsumer) error = %v", err)
		}

		consumerType := reflect.TypeOf(registration.Component)
		if _, ok := resolver.singletons[consumerType]; ok {
			t.Fatal("lazy component should not be cached before first resolve")
		}
		// Verify no injection was triggered on the registered source during Register.
		source := registration.Component.(*lazyConsumer)
		if source.Dependency != nil {
			t.Fatal("lazy component should not be injected before first resolve")
		}

		stored, ok := resolver.registrations[consumerType]
		if !ok {
			t.Fatalf("registration for %v not found", consumerType)
		}
		if !stored.Lazy {
			t.Fatal("lazy metadata should be preserved in registration")
		}

		var resolved *lazyConsumer
		if err := resolver.Resolve(&resolved); err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if resolved.Dependency == nil {
			t.Fatal("lazy component should be injected when first resolved")
		}
		if _, ok := resolver.singletons[consumerType]; !ok {
			t.Fatal("lazy singleton should be cached after first resolve")
		}
	})
}

func TestReflectResolver_InjectDependenciesAndGraph(t *testing.T) {
	resolver := NewReflectResolver()
	dependency := &testDependency{Name: "dependency"}
	service := &testService{}

	if err := resolver.Register(dependency); err != nil {
		t.Fatalf("Register(dependency) error = %v", err)
	}
	if err := resolver.Register(service); err != nil {
		t.Fatalf("Register(service) error = %v", err)
	}

	var resolved *testService
	if err := resolver.Resolve(&resolved); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if resolved.Dependency != dependency {
		t.Fatalf("resolved.Dependency = %p, want %p", resolved.Dependency, dependency)
	}

	graph := resolver.Graph()
	serviceType := reflect.TypeOf(service).String()
	dependencyType := reflect.TypeOf(dependency).String()
	edges := graph.Edges[serviceType]
	if len(edges) != 1 || edges[0] != dependencyType {
		t.Fatalf("Graph().Edges[%q] = %v, want [%s]", serviceType, edges, dependencyType)
	}
}

func TestReflectResolver_InjectAssignableInterface(t *testing.T) {
	resolver := NewReflectResolver()

	if err := resolver.Register(&greeterImpl{}); err != nil {
		t.Fatalf("Register(greeterImpl) error = %v", err)
	}
	if err := resolver.Register(&interfaceConsumer{}); err != nil {
		t.Fatalf("Register(interfaceConsumer) error = %v", err)
	}

	var consumer *interfaceConsumer
	if err := resolver.Resolve(&consumer); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if consumer.Greeter == nil {
		t.Fatal("Greeter should be injected")
	}
	if got := consumer.Greeter.Greet(); got != "hello" {
		t.Fatalf("Greeter.Greet() = %q, want %q", got, "hello")
	}
}

func TestReflectResolver_InjectErrors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*ReflectResolver)
		resolve func(*ReflectResolver) error
		wantErr error
	}{
		{
			name: "unexported inject field returns ErrUnresolvable",
			setup: func(r *ReflectResolver) {
				_ = r.Register(&testDependency{})
				_ = r.Register(&invalidInjectConsumer{})
			},
			resolve: func(r *ReflectResolver) error {
				var consumer *invalidInjectConsumer
				return r.Resolve(&consumer)
			},
			wantErr: ErrUnresolvable,
		},
		{
			name: "ambiguous interface injection returns ErrUnresolvable",
			setup: func(r *ReflectResolver) {
				_ = r.Register(&greeterImpl{})
				_ = r.Register(&greeterImplAlt{})
				_ = r.Register(&interfaceConsumer{})
			},
			resolve: func(r *ReflectResolver) error {
				var consumer *interfaceConsumer
				return r.Resolve(&consumer)
			},
			wantErr: ErrUnresolvable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewReflectResolver()
			tt.setup(resolver)
			err := tt.resolve(resolver)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Resolve() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestReflectResolver_ValueInjection(t *testing.T) {
	tests := []struct {
		name        string
		valueLookup func(key string) (any, bool)
		wantErr     error
		checkResult func(t *testing.T, consumer *valueConsumer)
	}{
		{
			name: "injects scalar values from lookup",
			valueLookup: func(key string) (any, bool) {
				values := map[string]any{
					"server.port":     "8080",
					"app.name":        "helix",
					"feature.enabled": "true",
				}
				value, ok := values[key]
				return value, ok
			},
			wantErr: nil,
			checkResult: func(t *testing.T, consumer *valueConsumer) {
				t.Helper()
				if consumer.Port != 8080 {
					t.Fatalf("Port = %d, want 8080", consumer.Port)
				}
				if consumer.Name != "helix" {
					t.Fatalf("Name = %q, want %q", consumer.Name, "helix")
				}
				if !consumer.Enabled {
					t.Fatal("Enabled should be true")
				}
			},
		},
		{
			name:        "missing value returns ErrNotFound",
			valueLookup: func(_ string) (any, bool) { return nil, false },
			wantErr:     ErrNotFound,
		},
		{
			name:        "invalid conversion returns ErrUnresolvable",
			valueLookup: func(_ string) (any, bool) { return "not-a-number", true },
			wantErr:     ErrUnresolvable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewReflectResolver()
			resolver.valueLookup = tt.valueLookup
			if err := resolver.Register(&valueConsumer{}); err != nil {
				t.Fatalf("Register(valueConsumer) error = %v", err)
			}

			var consumer *valueConsumer
			err := resolver.Resolve(&consumer)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Resolve() error = %v, want %v", err, tt.wantErr)
			}
			if tt.checkResult != nil && err == nil {
				tt.checkResult(t, consumer)
			}
		})
	}
}

func TestReflectResolver_CyclicDependencies(t *testing.T) {
	tests := []struct {
		name       string
		register   func(*ReflectResolver) error
		resolve    func(*ReflectResolver) error
		wantPath   []string
		checkState func(t *testing.T, resolver *ReflectResolver)
	}{
		{
			name: "direct cycle returns ErrCyclicDep with readable path",
			register: func(r *ReflectResolver) error {
				for _, component := range []any{&cycleServiceA{}, &cycleServiceB{}} {
					if err := r.Register(component); err != nil {
						return err
					}
				}
				return nil
			},
			resolve: func(r *ReflectResolver) error {
				var service *cycleServiceA
				return r.Resolve(&service)
			},
			wantPath: []string{"*core.cycleServiceA", "*core.cycleServiceB", "*core.cycleServiceA"},
			checkState: func(t *testing.T, resolver *ReflectResolver) {
				t.Helper()
				if len(resolver.singletons) != 0 {
					t.Fatalf("singletons should stay empty after cyclic resolution failure, got %d entries", len(resolver.singletons))
				}
			},
		},
		{
			name: "long cycle returns complete path",
			register: func(r *ReflectResolver) error {
				for _, component := range []any{&longCycleServiceA{}, &longCycleServiceB{}, &longCycleServiceC{}} {
					if err := r.Register(component); err != nil {
						return err
					}
				}
				return nil
			},
			resolve: func(r *ReflectResolver) error {
				var service *longCycleServiceA
				return r.Resolve(&service)
			},
			wantPath: []string{"*core.longCycleServiceA", "*core.longCycleServiceB", "*core.longCycleServiceC", "*core.longCycleServiceA"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewReflectResolver()
			if err := tt.register(resolver); err != nil {
				t.Fatalf("register() error = %v", err)
			}

			err := tt.resolve(resolver)
			if !errors.Is(err, ErrCyclicDep) {
				t.Fatalf("Resolve() error = %v, want %v", err, ErrCyclicDep)
			}

			var cyclicErr *CyclicDepError
			if !errors.As(err, &cyclicErr) {
				t.Fatalf("Resolve() error = %v, want *CyclicDepError", err)
			}

			if !reflect.DeepEqual(cyclicErr.Path, tt.wantPath) {
				t.Fatalf("CyclicDepError.Path = %v, want %v", cyclicErr.Path, tt.wantPath)
			}

			message := cyclicErr.Error()
			for _, step := range tt.wantPath {
				if !strings.Contains(message, step) {
					t.Fatalf("CyclicDepError.Error() = %q, want substring %q", message, step)
				}
			}

			if tt.checkState != nil {
				tt.checkState(t, resolver)
			}
		})
	}
}

func registeredComponentType(component any) reflect.Type {
	if registration, ok := component.(ComponentRegistration); ok {
		return reflect.TypeOf(registration.Component)
	}
	return reflect.TypeOf(component)
}
