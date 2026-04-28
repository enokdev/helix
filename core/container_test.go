package core

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestContainer_Register(t *testing.T) {
	tests := []struct {
		name      string
		resolver  Resolver
		component any
		wantErr   error
	}{
		{
			name:      "nil resolver returns ErrUnresolvable",
			resolver:  nil,
			component: &struct{}{},
			wantErr:   ErrUnresolvable,
		},
		{
			name:      "nil component returns ErrUnresolvable",
			resolver:  &stubResolver{},
			component: nil,
			wantErr:   ErrUnresolvable,
		},
		{
			name:      "with resolver delegates to resolver",
			resolver:  &stubResolver{},
			component: &struct{}{},
			wantErr:   nil,
		},
		{
			name:     "prototype lazy registration returns ErrUnresolvable",
			resolver: NewReflectResolver(),
			component: ComponentRegistration{
				Component: &testDependency{},
				Scope:     ScopePrototype,
				Lazy:      true,
			},
			wantErr: ErrUnresolvable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{resolver: tt.resolver}
			err := c.Register(tt.component)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainer_Resolve(t *testing.T) {
	tests := []struct {
		name     string
		resolver Resolver
		target   any
		wantErr  error
	}{
		{
			name:     "nil resolver returns ErrUnresolvable",
			resolver: nil,
			target:   &struct{}{},
			wantErr:  ErrUnresolvable,
		},
		{
			name:     "nil target returns ErrUnresolvable",
			resolver: &stubResolver{},
			target:   nil,
			wantErr:  ErrUnresolvable,
		},
		{
			name:     "with resolver delegates to resolver",
			resolver: &stubResolver{},
			target:   &struct{}{},
			wantErr:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Container{resolver: tt.resolver}
			err := c.Resolve(tt.target)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewContainer(t *testing.T) {
	t.Run("no options returns non-nil container with nil resolver", func(t *testing.T) {
		c := NewContainer()
		if c == nil {
			t.Fatal("NewContainer() returned nil")
		}
		if c.resolver != nil {
			t.Error("expected nil resolver without options")
		}
	})

	t.Run("WithResolver sets resolver", func(t *testing.T) {
		r := &stubResolver{}
		c := NewContainer(WithResolver(r))
		if c.resolver != r {
			t.Error("WithResolver option not applied")
		}
	})

	t.Run("multiple options applied in order", func(t *testing.T) {
		r1 := &stubResolver{}
		r2 := &stubResolver{}
		c := NewContainer(WithResolver(r1), WithResolver(r2))
		if c.resolver != r2 {
			t.Error("last WithResolver should win")
		}
	})
}

func TestContainer_Graph(t *testing.T) {
	t.Run("nil resolver returns writable empty graph", func(t *testing.T) {
		container := NewContainer()

		graph := container.Graph()
		if graph.Edges == nil {
			t.Fatal("Graph().Edges should be initialized")
		}

		graph.Edges["caller-owned"] = nil
	})

	t.Run("delegates to resolver with defensive copy", func(t *testing.T) {
		container := NewContainer(WithResolver(NewReflectResolver()))
		dependency := &testDependency{Name: "graph"}
		service := &testService{}
		if err := container.Register(dependency); err != nil {
			t.Fatalf("Register(dependency) error = %v", err)
		}
		if err := container.Register(service); err != nil {
			t.Fatalf("Register(service) error = %v", err)
		}

		var resolved *testService
		if err := container.Resolve(&resolved); err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}

		graph := container.Graph()
		serviceType := reflect.TypeOf(service).String()
		dependencyType := reflect.TypeOf(dependency).String()
		if !reflect.DeepEqual(graph.Edges[serviceType], []string{dependencyType}) {
			t.Fatalf("Graph().Edges[%q] = %v, want [%s]", serviceType, graph.Edges[serviceType], dependencyType)
		}

		graph.Nodes = append(graph.Nodes, "mutated")
		graph.Edges[serviceType] = append(graph.Edges[serviceType], "mutated")

		freshGraph := container.Graph()
		if reflect.DeepEqual(freshGraph.Nodes, graph.Nodes) {
			t.Fatal("Graph() should return a defensive copy of nodes")
		}
		if reflect.DeepEqual(freshGraph.Edges[serviceType], graph.Edges[serviceType]) {
			t.Fatal("Graph() should return a defensive copy of edges")
		}
	})
}

func TestContainerConcurrentResolveUsesRegisteredGraph(t *testing.T) {
	t.Parallel()

	container := NewContainer(WithResolver(NewReflectResolver()))
	if err := container.Register(&testDependency{Name: "shared"}); err != nil {
		t.Fatalf("Register(dependency) error = %v", err)
	}
	if err := container.Register(&testService{}); err != nil {
		t.Fatalf("Register(service) error = %v", err)
	}

	runConcurrently(t, 32, func() {
		var service *testService
		if err := container.Resolve(&service); err != nil {
			t.Errorf("Resolve() error = %v", err)
			return
		}
		if service == nil || service.Dependency == nil {
			t.Error("Resolve() returned service without injected dependency")
		}
	})
}

func TestContainerConcurrentRegisterResolveAndGraphDoesNotRace(t *testing.T) {
	t.Parallel()

	resolver := NewReflectResolver()
	container := NewContainer(WithResolver(resolver))
	if err := container.Register(&testDependency{Name: "initial"}); err != nil {
		t.Fatalf("Register(initial dependency) error = %v", err)
	}
	if err := container.Register(&testService{}); err != nil {
		t.Fatalf("Register(service) error = %v", err)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 50; j++ {
				_ = container.Register(&testDependency{Name: "replacement"})
			}
		}()
	}

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 50; j++ {
				var service *testService
				_ = container.Resolve(&service)
				_ = container.Graph()
			}
		}()
	}

	close(start)
	wg.Wait()
}
